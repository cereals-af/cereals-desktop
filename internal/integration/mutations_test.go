//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/observe"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type resolver struct{ client manifest.Client }

func (r resolver) ManifestClient(string) (manifest.Client, error) { return r.client, nil }

func TestAPIServerMutationLifecycle(t *testing.T) {
	config := controlPlane(t)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	connection, err := (kube.Factory{}).New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connection.Close)
	writer := connection.(manifest.Client)
	s := manifest.NewService(resolver{client: writer})
	t.Cleanup(func() { _ = s.ServiceShutdown() })
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	ref := resource.Ref{ClusterID: "test-only", Version: "v1", Resource: "configmaps", Namespace: "default", Name: "creals-test"}
	api := dynamicClient.Resource(schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}).Namespace("default")
	text := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: creals-test\n  namespace: default\ndata:\n  untouched: preserve-me\n  change: first\n"
	preview, err := s.Preview(ctx, manifest.Request{Target: ref, Mode: "create", YAML: text})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.Get(ctx, ref.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("dry run persisted an object: %v", err)
	}
	if _, err := s.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Snapshot(ctx, ref, false)
	if err != nil {
		t.Fatal(err)
	}

	// A real watch observes subsequent writes and exits when its caller cancels.
	page, err := connection.List(ctx, resource.Query{Version: "v1", Resource: "configmaps", Namespace: "default"})
	if err != nil {
		t.Fatal(err)
	}
	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	changed := make(chan struct{}, 1)
	watchDone := make(chan error, 1)
	go func() {
		watchDone <- connection.(observe.Client).Observe(watchCtx, resource.Query{Version: "v1", Resource: "configmaps", Namespace: "default"}, page.ResourceVersion, func(change observe.Change) {
			if change.Dirty {
				select {
				case changed <- struct{}{}:
				default:
				}
			}
		})
	}()

	request := manifest.Request{Target: ref, Mode: "patch", YAML: strings.Replace(snapshot.YAML, "change: first", "change: second", 1), Baseline: snapshot.Baseline}
	preview, err = s.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	// Change the live object between preview and confirmation.
	if _, err := api.Patch(ctx, ref.Name, types.MergePatchType, []byte(`{"data":{"external":"keep-also"}}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, preview.Token); err == nil {
		t.Fatal("stale preview overwrote a concurrent update")
	}
	snapshot, err = s.Snapshot(ctx, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	request.Baseline, request.YAML = snapshot.Baseline, strings.Replace(snapshot.YAML, "change: first", "change: second", 1)
	preview, err = s.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	object, err := api.Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	data, _, _ := unstructured.NestedStringMap(object.Object, "data")
	if data["untouched"] != "preserve-me" || data["external"] != "keep-also" || data["change"] != "second" {
		t.Fatalf("lost fields: %v", data)
	}
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("watch missed live writes")
	}
	stopWatch()
	select {
	case <-watchDone:
	case <-time.After(3 * time.Second):
		t.Fatal("watch leaked after cancellation")
	}

	t.Run("deployment-forms", func(t *testing.T) { checkDeploymentForms(t, ctx, s, writer, dynamicClient) })
	t.Run("scale-subresource-rbac", func(t *testing.T) { checkScaleRBAC(t, ctx, config, dynamicClient) })
	t.Run("other-scale-resources", func(t *testing.T) { checkOtherScaleResources(t, ctx, s, dynamicClient) })
	t.Run("deployment-release-history", func(t *testing.T) { checkReleaseHistory(t, ctx, s, writer, dynamicClient) })
	t.Run("service-configmap-forms", func(t *testing.T) { checkCoreForms(t, ctx, s, writer, dynamicClient) })
	t.Run("ssa-conflict-and-delete-preconditions", func(t *testing.T) { checkApplyAndDelete(t, ctx, s, api, ref) })
	t.Run("server-schema-validation", func(t *testing.T) {
		bad := strings.Replace(text, "change: first", "change: [invalid-type]", 1)
		if _, err := s.Preview(ctx, manifest.Request{Target: ref, Mode: "create", YAML: bad}); err == nil {
			t.Fatal("server accepted invalid data schema")
		}
	})
	t.Run("crd-int64", func(t *testing.T) { checkCRDInteger(t, ctx, s, dynamicClient) })
	t.Run("custom-resource-forms", func(t *testing.T) { checkCustomResources(t, ctx, s, writer, dynamicClient, config) })
	t.Run("schema-and-relations", func(t *testing.T) { checkInspection(t, ctx, config, connection, dynamicClient) })
	t.Run("server-defaults-in-diff", func(t *testing.T) {
		serviceRef := resource.Ref{ClusterID: "test-only", Version: "v1", Resource: "services", Namespace: "default", Name: "creals-service"}
		text := "apiVersion: v1\nkind: Service\nmetadata:\n  name: creals-service\n  namespace: default\nspec:\n  selector:\n    app: demo\n  ports:\n    - port: 80\n"
		preview, err := s.Preview(ctx, manifest.Request{Target: serviceRef, Mode: "create", YAML: text})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(preview.Desired, "protocol:") || !strings.Contains(preview.After, "protocol: TCP") {
			t.Fatal("server defaults missing from the separate diff")
		}
		s.Discard(preview.Token)
	})
	t.Run("rbac-denies-write", func(t *testing.T) {
		limited := rest.CopyConfig(config)
		limited.Impersonate = rest.ImpersonationConfig{UserName: "creals-no-permissions"}
		connection, err := (kube.Factory{}).New(ctx, limited)
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		service := manifest.NewService(resolver{client: connection.(manifest.Client)})
		defer service.ServiceShutdown()
		if _, err := service.Preview(ctx, manifest.Request{Target: ref, Mode: "create", YAML: text}); err == nil || !strings.Contains(err.Error(), "权限不足") {
			t.Fatalf("RBAC denial was not surfaced: %v", err)
		}
	})
}

func checkApplyAndDelete(t *testing.T, ctx context.Context, s *manifest.Service, api dynamic.ResourceInterface, ref resource.Ref) {
	t.Helper()
	force := true
	other := []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"creals-test","namespace":"default"},"data":{"owned":"other"}}`)
	if _, err := api.Patch(ctx, ref.Name, types.ApplyPatchType, other, metav1.PatchOptions{FieldManager: "other-controller", Force: &force}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Snapshot(ctx, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	intent := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: creals-test\n  namespace: default\ndata:\n  owned: mine\n"
	if _, err := s.Preview(ctx, manifest.Request{Target: ref, Mode: "apply", YAML: intent, Baseline: snapshot.Baseline}); err == nil {
		t.Fatal("SSA stole a field without force")
	}
	intent = strings.Replace(intent, "owned: mine", "new-owned: mine", 1)
	preview, err := s.Preview(ctx, manifest.Request{Target: ref, Mode: "apply", YAML: intent, Baseline: snapshot.Baseline})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.Snapshot(ctx, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	deletion := manifest.DeleteRequest{Target: ref, Baseline: snapshot.Baseline, Propagation: "Background"}
	preview, err = s.PreviewDelete(ctx, deletion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.Get(ctx, ref.Name, metav1.GetOptions{}); err != nil {
		t.Fatal("delete dry run removed object")
	}
	if _, err := api.Patch(ctx, ref.Name, types.MergePatchType, []byte(`{"data":{"changed-again":"yes"}}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, preview.Token); err == nil {
		t.Fatal("delete ignored version precondition")
	}
	snapshot, err = s.Snapshot(ctx, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	deletion.Baseline = snapshot.Baseline
	preview, err = s.PreviewDelete(ctx, deletion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Get(ctx, ref.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("resource still exists: %v", err)
	}
}

func checkCRDInteger(t *testing.T, ctx context.Context, s *manifest.Service, client dynamic.Interface) {
	t.Helper()
	api := client.Resource(schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"})
	definition := &unstructured.Unstructured{}
	if err := definition.UnmarshalJSON([]byte(`{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"widgets.creals.test"},"spec":{"group":"creals.test","names":{"plural":"widgets","singular":"widget","kind":"Widget"},"scope":"Namespaced","versions":[{"name":"v1","served":true,"storage":true,"schema":{"openAPIV3Schema":{"type":"object","properties":{"spec":{"type":"object","properties":{"sequence":{"type":"integer","format":"int64"},"label":{"type":"string"}}}}}}}]}}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Create(ctx, definition, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		object, err := api.Get(ctx, "widgets.creals.test", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		conditions, _, _ := unstructured.NestedSlice(object.Object, "status", "conditions")
		established := false
		for _, value := range conditions {
			condition, ok := value.(map[string]interface{})
			if ok && condition["type"] == "Established" && condition["status"] == "True" {
				established = true
			}
		}
		if established {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	ref := resource.Ref{ClusterID: "test-only", Group: "creals.test", Version: "v1", Resource: "widgets", Namespace: "default", Name: "one"}
	text := "apiVersion: creals.test/v1\nkind: Widget\nmetadata:\n  name: one\n  namespace: default\nspec:\n  sequence: 9007199254740993\n  label: before\n"
	preview, err := s.Preview(ctx, manifest.Request{Target: ref, Mode: "create", YAML: text})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Snapshot(ctx, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	preview, err = s.Preview(ctx, manifest.Request{Target: ref, Mode: "patch", YAML: strings.Replace(snapshot.YAML, "label: before", "label: after", 1), Baseline: snapshot.Baseline})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.After, "9007199254740993") {
		t.Fatal("CRD integer rounded during server preview")
	}
	if _, err := s.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
}
