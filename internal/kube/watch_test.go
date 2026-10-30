package kube

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/observe"
	"github.com/cereals-af/cereals-desktop/internal/resource"
)

func TestWatchRelistsAfterExpiredHistoryAndRetainsScope(t *testing.T) {
	var watches, lists atomic.Int32
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/team/pods" || r.URL.Query().Get("labelSelector") != "app=one" {
			t.Errorf("watch lost scope: %s", r.URL)
		}
		if r.URL.Query().Get("watch") != "true" {
			lists.Add(1)
			writeJSON(t, w, `{"apiVersion":"v1","kind":"PodList","metadata":{"resourceVersion":"20"},"items":[]}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if watches.Add(1) == 1 {
			if r.URL.Query().Get("resourceVersion") != "10" {
				t.Error("missing original RV")
			}
			_, _ = fmt.Fprintln(w, `{"type":"ERROR","object":{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"Expired","code":410}}`)
			return
		}
		if r.URL.Query().Get("resourceVersion") != "20" {
			t.Error("did not resume from refreshed RV")
		}
		_, _ = fmt.Fprintln(w, `{"type":"MODIFIED","object":{"apiVersion":"v1","kind":"Pod","metadata":{"name":"one","resourceVersion":"21"}}}`)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := client.(observe.Client).Observe(ctx, resource.Query{Version: "v1", Resource: "pods", Namespace: "team", LabelSelector: "app=one"}, "10", func(change observe.Change) {
		if change.State == "live" && change.Dirty {
			cancel()
		}
	})
	if err == nil || lists.Load() != 1 || watches.Load() != 2 {
		t.Fatalf("wrong recovery: lists=%d watches=%d err=%v", lists.Load(), watches.Load(), err)
	}
}
