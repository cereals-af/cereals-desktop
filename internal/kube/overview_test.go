package kube

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/overview"
	"github.com/cereals-af/cereals-desktop/internal/resource"
)

func TestOverviewCountsAllNamespacesWithMetadataPagination(t *testing.T) {
	calls := 0
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v1/secrets" || r.URL.Query().Get("limit") != "500" || !strings.Contains(r.Header.Get("Accept"), "PartialObjectMetadataList") {
			t.Errorf("unexpected metadata request: %s %s", r.URL, r.Header.Get("Accept"))
		}
		if r.URL.Query().Get("continue") == "" {
			writeJSON(t, w, `{"apiVersion":"meta.k8s.io/v1","kind":"PartialObjectMetadataList","metadata":{"continue":"next","resourceVersion":"1"},"items":[{"metadata":{"name":"a","namespace":"one"}}]}`)
		} else {
			if r.URL.Query().Get("continue") != "next" {
				t.Error("incorrect continuation")
			}
			writeJSON(t, w, `{"apiVersion":"meta.k8s.io/v1","kind":"PartialObjectMetadataList","metadata":{"resourceVersion":"1"},"items":[{"metadata":{"name":"b","namespace":"two"}},{"metadata":{"name":"c","namespace":"two"}}]}`)
		}
	})
	count, err := client.(overview.Client).CountResources(t.Context(), resource.Type{Version: "v1", Resource: "secrets"})
	if err != nil || count != 3 || calls != 2 {
		t.Fatalf("count=%d calls=%d err=%v", count, calls, err)
	}
}

func TestOverviewUsesRemainingCountAndRejectsPartialFailures(t *testing.T) {
	for _, mode := range []string{"remaining", "forbidden", "expired", "stuck"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if mode == "remaining" {
					writeJSON(t, w, `{"apiVersion":"meta.k8s.io/v1","kind":"PartialObjectMetadataList","metadata":{"continue":"next","remainingItemCount":1200},"items":[{"metadata":{"name":"a"}}]}`)
					return
				}
				if calls == 1 || mode == "stuck" {
					writeJSON(t, w, `{"apiVersion":"meta.k8s.io/v1","kind":"PartialObjectMetadataList","metadata":{"continue":"next"},"items":[{"metadata":{"name":"a"}}]}`)
					return
				}
				code, reason := 403, "Forbidden"
				if mode == "expired" {
					code, reason = 410, "Expired"
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(code)
				writeJSON(t, w, `{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"`+reason+`","message":"sensitive-server-body"}`)
			})
			count, err := client.(overview.Client).CountResources(t.Context(), resource.Type{Group: "apps", Version: "v1", Resource: "deployments"})
			if mode == "remaining" {
				if count != 1201 || err != nil || calls != 1 {
					t.Fatalf("%d %d %v", count, calls, err)
				}
				return
			}
			if count != 0 || err == nil || strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("partial result escaped: %d %v", count, err)
			}
		})
	}
}

func TestOverviewAggregatesNodeQuantitiesAcrossPages(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/nodes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("continue") == "" {
			writeJSON(t, w, `{"apiVersion":"v1","kind":"NodeList","metadata":{"continue":"next"},"items":[{"metadata":{"name":"one"},"status":{"capacity":{"cpu":"1500m","memory":"1048576Ki","ephemeral-storage":"2Gi","pods":"110","example.com/gpu":"1"},"allocatable":{"cpu":"1","memory":"512Mi","pods":"100"},"conditions":[{"type":"Ready","status":"True"}]}}]}`)
		} else {
			writeJSON(t, w, `{"apiVersion":"v1","kind":"NodeList","items":[{"metadata":{"name":"two"},"spec":{"unschedulable":true},"status":{"capacity":{"cpu":"2","memory":"1Gi","ephemeral-storage":"2Gi","pods":"110"},"allocatable":{"cpu":"1500m","memory":"512Mi","pods":"100"},"conditions":[{"type":"Ready","status":"Unknown"}]}}]}`)
		}
	})
	result, err := client.(overview.Client).OverviewNodes(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 2 || result.Ready != 1 || result.Unschedulable != 1 || result.Capacity["cpu"] != "3.500" || result.Capacity["memory"] != "2Gi" || result.Allocatable["cpu"] != "2.500" || result.Capacity["example.com/gpu"] != "1" {
		t.Fatalf("unexpected totals: %+v", result)
	}
}

func TestOverviewHonorsCancellation(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("canceled request reached server") })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.(overview.Client).OverviewNodes(ctx); err == nil {
		t.Fatal("nodes ignored cancellation")
	}
	if _, err := client.(overview.Client).CountResources(ctx, resource.Type{Version: "v1", Resource: "pods"}); err == nil {
		t.Fatal("count ignored cancellation")
	}
}
