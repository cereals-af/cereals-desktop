//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workload"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

func checkDeploymentForms(t *testing.T, ctx context.Context, manifests *manifest.Service, writer manifest.Client, client dynamic.Interface) {
	t.Helper()
	service := workload.NewService(resolver{client: writer}, manifests)
	target := resource.Ref{ClusterID: "test-only", Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default", Name: "form-app"}
	api := client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).Namespace("default")
	form := workload.Deployment{Name: target.Name, Namespace: target.Namespace, Replicas: 2, Containers: []workload.Container{{Name: "app", Image: "nginx:1.28", CPURequest: "100m", Environment: []workload.Environment{{Name: "ENV", Value: "before"}}}}}
	preview, err := service.Preview(ctx, workload.Request{Target: target, Action: "create", Form: form})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.Get(ctx, target.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("form dry run wrote object: %v", err)
	}
	if !strings.Contains(preview.After, "RollingUpdate") {
		t.Fatal("server defaults absent from form preview")
	}
	if _, err := manifests.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	// Add fields the form does not expose, including a Secret reference and an init container.
	patch := []byte(`{"spec":{"template":{"spec":{"initContainers":[{"name":"init","image":"busybox","command":["true"]}],"containers":[{"name":"app","image":"nginx:1.28","ports":[{"name":"http","containerPort":8080}],"env":[{"name":"ENV","value":"before"},{"name":"TOKEN","valueFrom":{"secretKeyRef":{"name":"token","key":"value"}}}],"resources":{"requests":{"cpu":"100m","ephemeral-storage":"1Gi"}}}]}}}}`)
	if _, err := api.Patch(ctx, target.Name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Deployment(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status.Observed || snapshot.Status.Phase == "已就绪" {
		t.Fatal("API acceptance reported as rollout readiness")
	}
	snapshot.Form.Containers[0].Image = "nginx:1.29"
	snapshot.Form.Containers[0].MemoryLimit = "128Mi"
	snapshot.Form.Containers[0].Environment[0].Value = "after"
	snapshot.Form.Containers[0].Readiness = workload.Probe{Kind: "http", Path: "/health", Port: "http", Scheme: "HTTP", Period: 10, Timeout: 1, SuccessThreshold: 1, FailureThreshold: 3}
	preview, err = service.Preview(ctx, workload.Request{Target: target, Baseline: snapshot.Baseline, Action: "update", Form: snapshot.Form})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manifests.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	object, err := api.Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	json, err := object.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"initContainers", "secretKeyRef", "ephemeral-storage", "nginx:1.29", "128Mi", "/health"} {
		if !strings.Contains(string(json), expected) {
			t.Fatalf("lost field %s", expected)
		}
	}
	// Scale updates only spec.replicas, guarded by the same resourceVersion as other writes.
	snapshot, err = service.Deployment(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Form.Replicas = 0
	request := workload.Request{Target: target, Baseline: snapshot.Baseline, Action: "scale", Form: snapshot.Form}
	preview, err = service.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.Patch(ctx, target.Name, types.MergePatchType, []byte(`{"metadata":{"annotations":{"other":"keep"}}}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := manifests.Commit(ctx, preview.Token); err == nil {
		t.Fatal("form overwrote concurrent update")
	}
	snapshot, err = service.Deployment(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	request.Baseline = snapshot.Baseline
	preview, err = service.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manifests.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	snapshot, err = service.Deployment(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Form.Replicas != 0 {
		t.Fatal("scale was not persisted")
	}
	preview, err = service.Preview(ctx, workload.Request{Target: target, Baseline: snapshot.Baseline, Action: "restart"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manifests.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	object, err = api.Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	stamp, _, _ := unstructured.NestedString(object.Object, "spec", "template", "metadata", "annotations", "kubectl.kubernetes.io/restartedAt")
	if stamp == "" || object.GetAnnotations()["other"] != "keep" {
		t.Fatal("restart failed or lost annotations")
	}
	// Schema semantics remain the API server's authority.
	snapshot, err = service.Deployment(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Form.Containers[0].CPURequest = "2"
	snapshot.Form.Containers[0].CPULimit = "1"
	if _, err := service.Preview(ctx, workload.Request{Target: target, Baseline: snapshot.Baseline, Action: "update", Form: snapshot.Form}); err == nil {
		t.Fatal("server accepted request > limit")
	}
}
