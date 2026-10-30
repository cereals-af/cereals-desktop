//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/node"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type nodeResolver struct{ client node.Client }

func (r nodeResolver) NodeClient(string) (node.Client, error) { return r.client, nil }
func TestAPIServerNodeMaintenance(t *testing.T) {
	config := controlPlane(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	connection, err := (kube.Factory{}).New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	writer := manifest.NewService(resolver{connection.(manifest.Client)})
	defer writer.ServiceShutdown()
	service := node.NewService(ctx, nodeResolver{connection.(node.Client)}, writer)
	defer service.ServiceShutdown()
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	nodes := client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "nodes"})
	pods := client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).Namespace("default")
	pdbs := client.Resource(schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}).Namespace("default")
	target := resource.Ref{ClusterID: "test-only", Version: "v1", Resource: "nodes", Name: "maintenance-node"}
	object := createNodeFixture(t, ctx, nodes, `{"apiVersion":"v1","kind":"Node","metadata":{"name":"maintenance-node","labels":{"node-role.kubernetes.io/worker":""},"annotations":{"keep":"9007199254740993"}},"spec":{"taints":[{"key":"maintenance","effect":"PreferNoSchedule"}]}}`)
	object.Object["status"] = map[string]interface{}{"capacity": map[string]interface{}{"cpu": "8", "memory": "32Gi"}, "allocatable": map[string]interface{}{"cpu": "7800m", "memory": "30Gi"}, "conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": "True", "reason": "TestReady"}}}
	if _, err := nodes.UpdateStatus(ctx, object, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Inspect(ctx, target)
	if err != nil || snapshot.Capacity["memory"] != "32Gi" {
		t.Fatalf("inspect %+v %v", snapshot, err)
	}
	preview, err := service.PreviewCordon(ctx, node.CordonRequest{Target: target, Baseline: snapshot.Baseline, Unschedulable: true})
	if err != nil {
		t.Fatal(err)
	}
	live, err := nodes.Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if flag, _, _ := unstructured.NestedBool(live.Object, "spec", "unschedulable"); flag {
		t.Fatal("dry run cordoned")
	}
	if _, err := nodes.Patch(ctx, target.Name, types.MergePatchType, []byte(`{"metadata":{"annotations":{"concurrent":"change"}}}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Commit(ctx, preview.Token); err == nil {
		t.Fatal("cordon ignored concurrent change")
	}
	snapshot, err = service.Inspect(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	pod := createNodeFixture(t, ctx, pods, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"maintenance-pod","labels":{"app":"drain"}},"spec":{"nodeName":"maintenance-node","terminationGracePeriodSeconds":0,"containers":[{"name":"main","image":"example.test/app:1"}]}}`)
	pod.Object["status"] = map[string]interface{}{"phase": "Running", "conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": "True"}}}
	pod, err = pods.UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pdb := createNodeFixture(t, ctx, pdbs, `{"apiVersion":"policy/v1","kind":"PodDisruptionBudget","metadata":{"name":"maintenance-pdb"},"spec":{"minAvailable":1,"selector":{"matchLabels":{"app":"drain"}}}}`)
	// There is no controller manager in envtest. Set controller status explicitly;
	// enforcement and PDB reservation still run in the real API Server.
	setPDBStatus(t, ctx, pdbs, pdb, 0)
	plan, err := service.PreviewDrain(ctx, node.DrainRequest{Target: target, Baseline: snapshot.Baseline, Options: node.DrainOptions{TimeoutSeconds: 30, AllowUnmanaged: true}})
	if err != nil || plan.Token == "" || len(plan.Pods) != 1 {
		t.Fatalf("drain preview: %+v %v", plan, err)
	}
	if _, err := pods.Get(ctx, pod.GetName(), metav1.GetOptions{}); err != nil {
		t.Fatal("eviction dry run removed pod")
	}
	id, err := service.StartDrain(ctx, plan.Token, target.Name)
	if err != nil {
		t.Fatal(err)
	}
	waitNodeProgress(t, service, id, func(progress node.Progress) bool {
		return progress.State == "running" && len(progress.Pods) > 0 && strings.Contains(progress.Pods[0].Message, "等待 PDB 放行") && !progress.Done
	})
	live, err = nodes.Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if flag, _, _ := unstructured.NestedBool(live.Object, "spec", "unschedulable"); !flag {
		t.Fatal("node not cordoned")
	}
	pdb, err = pdbs.Get(ctx, pdb.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	setPDBStatus(t, ctx, pdbs, pdb, 1)
	result := waitNodeProgress(t, service, id, func(progress node.Progress) bool { return progress.Done })
	if result.State != "completed" {
		t.Fatalf("drain: %+v", result)
	}
	if _, err := pods.Get(ctx, pod.GetName(), metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("evicted pod remains: %v", err)
	}
	if _, err := service.StartDrain(ctx, plan.Token, target.Name); err == nil {
		t.Fatal("plan replay")
	}
	snapshot, err = service.Inspect(ctx, target)
	if err != nil || !snapshot.Unschedulable {
		t.Fatalf("node auto uncordoned: %+v %v", snapshot, err)
	}
	preview, err = service.PreviewCordon(ctx, node.CordonRequest{Target: target, Baseline: snapshot.Baseline, Unschedulable: false})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	live, err = nodes.Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if live.GetAnnotations()["keep"] != "9007199254740993" {
		t.Fatal("node annotations lost")
	}
	if flag, _, _ := unstructured.NestedBool(live.Object, "spec", "unschedulable"); flag {
		t.Fatal("uncordon failed")
	}
	t.Run("eviction-preconditions", func(t *testing.T) {
		replacement := createNodeFixture(t, ctx, pods, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"maintenance-pod"},"spec":{"nodeName":"maintenance-node","terminationGracePeriodSeconds":0,"containers":[{"name":"main","image":"example.test/app:1"}]}}`)
		_, err := connection.(node.Client).EvictPod(ctx, node.Eviction{Target: resource.Ref{Version: "v1", Resource: "pods", Namespace: "default", Name: replacement.GetName()}, Baseline: formedit.Baseline(pod)})
		if err == nil {
			t.Fatal("eviction ignored old UID/RV")
		}
	})
	t.Run("read-only-rbac", func(t *testing.T) { checkNodeReadOnly(t, ctx, config, client, target) })
	t.Run("eviction-denied-rbac", func(t *testing.T) { checkEvictionDenied(t, ctx, config, client, target) })
}
func createNodeFixture(t *testing.T, ctx context.Context, api dynamic.ResourceInterface, text string) *unstructured.Unstructured {
	t.Helper()
	object, err := manifest.ParseDraft(text)
	if err != nil {
		t.Fatal(err)
	}
	object, err = api.Create(ctx, object, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return object
}
func setPDBStatus(t *testing.T, ctx context.Context, api dynamic.ResourceInterface, pdb *unstructured.Unstructured, allowed int64) {
	t.Helper()
	pdb.Object["status"] = map[string]interface{}{"observedGeneration": pdb.GetGeneration(), "disruptionsAllowed": allowed, "currentHealthy": int64(1), "desiredHealthy": int64(1), "expectedPods": int64(1)}
	if _, err := api.UpdateStatus(ctx, pdb, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
}
func waitNodeProgress(t *testing.T, s *node.Service, id string, match func(node.Progress) bool) node.Progress {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	var last node.Progress
	for time.Now().Before(deadline) {
		progress, err := s.PollDrain(id)
		if err != nil {
			t.Fatal(err)
		}
		last = progress
		if match(progress) {
			return progress
		}
		if progress.Done {
			t.Fatalf("task ended early: %+v", progress)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("progress timed out: %+v", last)
	return last
}
func checkNodeReadOnly(t *testing.T, ctx context.Context, config *rest.Config, client dynamic.Interface, target resource.Ref) {
	for _, entry := range []struct{ resource, text string }{
		{"clusterroles", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"ClusterRole","metadata":{"name":"node-reader"},"rules":[{"apiGroups":[""],"resources":["nodes"],"verbs":["get","list"]}]}`},
		{"clusterrolebindings", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"ClusterRoleBinding","metadata":{"name":"node-reader"},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"node-reader"},"subjects":[{"kind":"User","name":"node-reader","apiGroup":"rbac.authorization.k8s.io"}]}`},
	} {
		createNodeFixture(t, ctx, client.Resource(schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: entry.resource}), entry.text)
	}
	restricted := rest.CopyConfig(config)
	restricted.Impersonate = rest.ImpersonationConfig{UserName: "node-reader", Groups: []string{"system:authenticated"}}
	connection, err := (kube.Factory{}).New(ctx, restricted)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	writer := manifest.NewService(resolver{connection.(manifest.Client)})
	defer writer.ServiceShutdown()
	s := node.NewService(ctx, nodeResolver{connection.(node.Client)}, writer)
	defer s.ServiceShutdown()
	snapshot, err := s.Inspect(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pods(ctx, target, "", ""); err == nil {
		t.Fatal("pod list permission bypass")
	}
	if _, err := s.PreviewCordon(ctx, node.CordonRequest{Target: target, Baseline: snapshot.Baseline, Unschedulable: true}); err == nil {
		t.Fatal("node patch permission bypass")
	}
	if _, err := s.PreviewDrain(ctx, node.DrainRequest{Target: target, Baseline: snapshot.Baseline, Options: node.DrainOptions{TimeoutSeconds: 30}}); err == nil {
		t.Fatal("partial inventory permitted drain")
	}
}

func checkEvictionDenied(t *testing.T, ctx context.Context, config *rest.Config, client dynamic.Interface, target resource.Ref) {
	roleAPI := client.Resource(schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"})
	if _, err := roleAPI.Patch(ctx, "node-reader", types.MergePatchType, []byte(`{"rules":[{"apiGroups":[""],"resources":["nodes"],"verbs":["get","list","patch"]},{"apiGroups":[""],"resources":["pods"],"verbs":["get","list"]}]}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	restricted := rest.CopyConfig(config)
	restricted.Impersonate = rest.ImpersonationConfig{UserName: "node-reader", Groups: []string{"system:authenticated"}}
	connection, err := (kube.Factory{}).New(ctx, restricted)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	writer := manifest.NewService(resolver{connection.(manifest.Client)})
	defer writer.ServiceShutdown()
	s := node.NewService(ctx, nodeResolver{connection.(node.Client)}, writer)
	defer s.ServiceShutdown()
	snapshot, err := s.Inspect(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.PreviewDrain(ctx, node.DrainRequest{Target: target, Baseline: snapshot.Baseline, Options: node.DrainOptions{TimeoutSeconds: 30, AllowUnmanaged: true}})
	if err != nil || plan.Token != "" || len(plan.Pods) != 1 || plan.Pods[0].State != "blocked" || !strings.Contains(plan.Pods[0].Message, "权限不足") {
		t.Fatalf("eviction permission bypass: %+v %v", plan, err)
	}
}
