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
	"github.com/cereals-af/cereals-desktop/internal/workloadstate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func healthObject(kind, name, uid string) unstructured.Unstructured {
	api := "apps/v1"
	if kind == "Pod" {
		api = "v1"
	}
	if kind == "Job" || kind == "CronJob" {
		api = "batch/v1"
	}
	result := unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": api, "kind": kind}}
	result.SetName(name)
	result.SetNamespace("team")
	result.SetUID(types.UID(uid))
	return result
}
func TestChildrenUseOwnerUIDAndPreferFaultOverPartialReadiness(t *testing.T) {
	deployment := healthObject("Deployment", "app", "deployment")
	rs := healthObject("ReplicaSet", "app-rs", "replica")
	ready := healthObject("Pod", "running", "ready")
	_ = unstructured.SetNestedField(ready.Object, "Running", "status", "phase")
	_ = unstructured.SetNestedSlice(ready.Object, []interface{}{map[string]interface{}{"ready": true}}, "status", "containerStatuses")
	broken := healthObject("Pod", "crashing", "error")
	_ = unstructured.SetNestedSlice(broken.Object, []interface{}{map[string]interface{}{"state": map[string]interface{}{"waiting": map[string]interface{}{"reason": "CrashLoopBackOff"}}}}, "status", "containerStatuses")
	children := workloadChildren{replicas: map[types.UID][]unstructured.Unstructured{"deployment": {rs}}, pods: map[types.UID][]unstructured.Unstructured{"replica": {ready}, "foreign": {broken}}}
	if state, _ := children.state(deployment); state != workloadstate.Ready {
		t.Fatal("foreign Pod affected status")
	}
	children.pods["replica"] = append(children.pods["replica"], broken)
	if state, _ := children.state(deployment); state != workloadstate.Error {
		t.Fatal("owned failing Pod did not override readiness")
	}
	delete(children.pods, "replica")
	if state, _ := children.state(deployment); state != workloadstate.Processing {
		t.Fatal("missing Pods with desired replicas were not progressing")
	}
}
func TestCronJobUsesLatestJobAndActivePodReadiness(t *testing.T) {
	cron := healthObject("CronJob", "schedule", "cron")
	old := healthObject("Job", "old", "old")
	old.SetCreationTimestamp(metav1.NewTime(time.Unix(1, 0)))
	fresh := healthObject("Job", "new", "new")
	fresh.SetCreationTimestamp(metav1.NewTime(time.Unix(2, 0)))
	_ = unstructured.SetNestedSlice(fresh.Object, []interface{}{map[string]interface{}{"type": "Failed", "status": "True", "reason": "BackoffLimitExceeded"}}, "status", "conditions")
	children := workloadChildren{jobs: map[types.UID][]unstructured.Unstructured{"cron": {old, fresh}}}
	if state, _ := children.state(cron); state != workloadstate.Error {
		t.Fatal("latest failed Job not reflected")
	}
	_ = unstructured.SetNestedSlice(fresh.Object, []interface{}{map[string]interface{}{"type": "Complete", "status": "True"}}, "status", "conditions")
	children.jobs["cron"] = []unstructured.Unstructured{old, fresh}
	if state, _ := children.state(cron); state != workloadstate.Completed {
		t.Fatal("latest completed Job not reflected")
	}
}
func TestWorkloadListReadsChildrenOnceAndKeepsPermissionFailureVisible(t *testing.T) {
	var podLists, rsLists atomic.Int32
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/apps/v1/namespaces/team/deployments":
			writeJSON(t, w, `{"apiVersion":"apps/v1","kind":"DeploymentList","items":[{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"one","uid":"one"}},{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"two","uid":"two"}}]}`)
		case "/api/v1/namespaces/team/pods":
			podLists.Add(1)
			writeJSON(t, w, `{"apiVersion":"v1","kind":"PodList","items":[]}`)
		case "/apis/apps/v1/namespaces/team/replicasets":
			rsLists.Add(1)
			w.WriteHeader(403)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	})
	page, err := client.List(t.Context(), resource.Query{Group: "apps", Version: "v1", Resource: "deployments", Namespace: "team"})
	if err != nil || len(page.Items) != 2 || podLists.Load() != 1 || rsLists.Load() != 1 {
		t.Fatalf("list=%+v err=%v", page, err)
	}
	for _, item := range page.Items {
		if item.Status != workloadstate.Processing || item.Detail == "" {
			t.Fatal("missing fallback report")
		}
	}
}
func TestWorkloadWatchObservesChildrenAndCancelsAllStreams(t *testing.T) {
	var pods atomic.Int32
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/team/pods" && r.URL.Path != "/apis/apps/v1/namespaces/team/deployments" {
			t.Errorf("unexpected watch scope %s", r.URL)
		}
		if r.URL.Path == "/api/v1/namespaces/team/pods" && r.URL.Query().Get("labelSelector") != "" {
			t.Error("parent-only selector reused for children")
		}
		if r.URL.Query().Get("watch") != "true" {
			writeJSON(t, w, `{"apiVersion":"v1","kind":"PodList","metadata":{"resourceVersion":"1"},"items":[]}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		if r.URL.Path == "/api/v1/namespaces/team/pods" {
			pods.Add(1)
			_, _ = fmt.Fprintln(w, `{"type":"MODIFIED","object":{"apiVersion":"v1","kind":"Pod","metadata":{"name":"one","resourceVersion":"2"}}}`)
			w.(http.Flusher).Flush()
		}
		<-r.Context().Done()
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	err := client.(observe.Client).Observe(ctx, resource.Query{Group: "apps", Version: "v1", Resource: "deployments", Namespace: "team", LabelSelector: "app=one"}, "1", func(change observe.Change) {
		if change.Dirty && pods.Load() > 0 {
			cancel()
		}
	})
	if err == nil || pods.Load() != 1 {
		t.Fatalf("missing child observation: %d %v", pods.Load(), err)
	}
}

func TestRetriedJobDoesNotKeepHistoricalFailedPodAsCurrentError(t *testing.T) {
	job := healthObject("Job", "retry", "job")
	failed := healthObject("Pod", "old-attempt", "failed")
	_ = unstructured.SetNestedField(failed.Object, "Failed", "status", "phase")
	ready := healthObject("Pod", "new-attempt", "ready")
	_ = unstructured.SetNestedField(ready.Object, "Running", "status", "phase")
	_ = unstructured.SetNestedSlice(ready.Object, []interface{}{map[string]interface{}{"ready": true}}, "status", "containerStatuses")
	children := workloadChildren{pods: map[types.UID][]unstructured.Unstructured{"job": {failed, ready}}}
	if state, _ := children.state(job); state != workloadstate.Ready {
		t.Fatal("historical failed attempt overshadowed current ready Pod")
	}
}
