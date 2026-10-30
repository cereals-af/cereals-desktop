//go:build integration

package integration

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/customresource"
	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const customSchema = `{"type":"object","required":["spec"],"properties":{"spec":{"type":"object","required":["label"],"properties":{"sequence":{"type":"integer","format":"int64","minimum":0},"label":{"type":"string","minLength":1},"nullable":{"type":"string","nullable":true},"defaulted":{"type":"integer","default":7},"settings":{"type":"object","properties":{"enabled":{"type":"boolean"}}},"items":{"type":"array","items":{"type":"object","x-kubernetes-preserve-unknown-fields":true,"properties":{"name":{"type":"string"}}}},"opaque":{"type":"object","x-kubernetes-preserve-unknown-fields":true}}}}}`

func checkCustomResources(t *testing.T, ctx context.Context, manifests *manifest.Service, writer manifest.Client, client dynamic.Interface, config *rest.Config) {
	t.Helper()
	s := customresource.NewService(resolver{writer}, manifests)
	for _, scope := range []string{"Namespaced", "Cluster"} {
		t.Run(scope, func(t *testing.T) {
			name := "gadgets"
			kind := "Gadget"
			namespace := "default"
			if scope == "Cluster" {
				name = "clustergadgets"
				kind = "ClusterGadget"
				namespace = ""
			}
			definition := createCustomDefinition(t, ctx, client, name, kind, scope)
			target := resource.Ref{ClusterID: "test-only", Group: "creals.test", Version: "v1", Resource: name, Namespace: namespace, Name: "sample"}
			create := target
			create.Name = ""
			snapshot, err := s.Snapshot(ctx, create)
			if err != nil {
				t.Fatal(err)
			}
			text := `{"apiVersion":"creals.test/v1","kind":"` + kind + `","metadata":{"name":"sample","namespace":"` + namespace + `"},"spec":{"label":"before","sequence":9007199254740993,"nullable":"old","settings":{"enabled":true},"items":[{"name":"a","unknown":9007199254740993}],"opaque":{"exact":9223372036854775807}}}`
			request := customresource.Request{Target: create, Definition: snapshot.Definition, Mode: "create", YAML: text}
			preview, err := s.Preview(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(preview.After, "defaulted: 7") {
				t.Fatal("server defaults absent")
			}
			if _, err := manifests.Commit(ctx, preview.Token); err != nil {
				t.Fatal(err)
			}
			api := client.Resource(schema.GroupVersionResource{Group: target.Group, Version: target.Version, Resource: name}).Namespace(namespace)
			before := getRelease(t, ctx, api, target.Name)
			snapshot, err = s.Snapshot(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			spec := snapshot.Draft.Value.Properties["spec"]
			if spec.Properties["sequence"].Text != "9007199254740993" {
				t.Fatal("snapshot precision lost")
			}
			spec.Properties["label"] = customresource.Value{Type: "string", Text: "after"}
			spec.Properties["nullable"] = customresource.Value{Type: "null"}
			delete(spec.Properties, "settings")
			snapshot.Draft.Value.Properties["spec"] = spec
			request = customresource.Request{Target: target, Definition: snapshot.Definition, Baseline: snapshot.Baseline, Mode: "update", Value: &snapshot.Draft.Value}
			preview, err = s.Preview(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, getRelease(t, ctx, api, target.Name)) {
				t.Fatal("form dry run persisted")
			}
			crdAPI := client.Resource(schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"})
			patchRelease(t, ctx, crdAPI, definition.GetName(), `{"metadata":{"annotations":{"schema-observer":"changed"}}}`)
			if _, err := manifests.Commit(ctx, preview.Token); err == nil {
				t.Fatal("changed definition accepted at commit")
			}
			if _, err := s.Preview(ctx, request); err == nil {
				t.Fatal("stale schema accepted at preview")
			}
			next, err := s.Snapshot(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			request.Definition = next.Definition
			preview, err = s.Preview(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			patchRelease(t, ctx, api, target.Name, `{"metadata":{"annotations":{"other":"keep"}}}`)
			if _, err := manifests.Commit(ctx, preview.Token); err == nil {
				t.Fatal("JSON patch ignored instance concurrency")
			}
			next, err = s.Snapshot(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			// Rebase from the full fresh tree, retaining the new annotation.
			nextSpec := next.Draft.Value.Properties["spec"]
			nextSpec.Properties["label"] = customresource.Value{Type: "string", Text: "after"}
			nextSpec.Properties["nullable"] = customresource.Value{Type: "null"}
			delete(nextSpec.Properties, "settings")
			next.Draft.Value.Properties["spec"] = nextSpec
			request.Baseline = next.Baseline
			request.Value = &next.Draft.Value
			preview, err = s.Preview(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := manifests.Commit(ctx, preview.Token); err != nil {
				t.Fatal(err)
			}
			after := getRelease(t, ctx, api, target.Name)
			specObject, _, _ := unstructured.NestedMap(after.Object, "spec")
			nullable, present := specObject["nullable"]
			if !present || nullable != nil {
				t.Fatal("explicit null was removed")
			}
			if _, exists := specObject["settings"]; exists {
				t.Fatal("removed object retained")
			}
			for _, field := range []string{"opaque", "items", "sequence"} {
				old, _, _ := unstructured.NestedFieldCopy(before.Object, "spec", field)
				got, _, _ := unstructured.NestedFieldCopy(after.Object, "spec", field)
				if !reflect.DeepEqual(old, got) {
					t.Fatalf("lost field %s", field)
				}
			}
			if after.GetAnnotations()["other"] != "keep" {
				t.Fatal("unknown annotation lost")
			}
			checkCustomTable(t, ctx, s, target)
			next, err = s.Snapshot(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			spec = next.Draft.Value.Properties["spec"]
			spec.Properties["sequence"] = customresource.Value{Type: "integer", Text: "-1"}
			next.Draft.Value.Properties["spec"] = spec
			if _, err := s.Preview(ctx, customresource.Request{Target: target, Definition: next.Definition, Baseline: next.Baseline, Mode: "update", Value: &next.Draft.Value}); err == nil {
				t.Fatal("server schema validation bypassed")
			}
			target.Version = "v3"
			if _, err := s.Snapshot(ctx, target); err == nil {
				t.Fatal("unserved version accepted")
			}
		})
	}
	t.Run("instance-only-permissions", func(t *testing.T) { checkCustomReadPermissions(t, ctx, client, config) })
}
func createCustomDefinition(t *testing.T, ctx context.Context, client dynamic.Interface, plural, kind, scope string) *unstructured.Unstructured {
	t.Helper()
	column := `[{"name":"Sequence","type":"integer","jsonPath":".spec.sequence"},{"name":"Label","type":"string","jsonPath":".spec.label"},{"name":"Advanced","type":"string","priority":1,"jsonPath":".spec.label"}]`
	text := `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"` + plural + `.creals.test"},"spec":{"group":"creals.test","names":{"plural":"` + plural + `","singular":"` + strings.ToLower(kind) + `","kind":"` + kind + `"},"scope":"` + scope + `","versions":[{"name":"v1","served":true,"storage":true,"schema":{"openAPIV3Schema":` + customSchema + `},"additionalPrinterColumns":` + column + `},{"name":"v2","served":true,"storage":false,"schema":{"openAPIV3Schema":` + customSchema + `},"additionalPrinterColumns":[{"name":"V2 Label","type":"string","jsonPath":".spec.label"}]},{"name":"v3","served":false,"storage":false,"schema":{"openAPIV3Schema":` + customSchema + `}}]}}`
	object := &unstructured.Unstructured{}
	if err := object.UnmarshalJSON([]byte(text)); err != nil {
		t.Fatal(err)
	}
	api := client.Resource(schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"})
	created, err := api.Create(ctx, object, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	waitEstablished(t, ctx, api, created.GetName())
	return getRelease(t, ctx, api, created.GetName())
}
func checkCustomTable(t *testing.T, ctx context.Context, s *customresource.Service, target resource.Ref) {
	t.Helper()
	query := resource.Query{ClusterID: target.ClusterID, Group: target.Group, Version: target.Version, Resource: target.Resource, Namespace: target.Namespace}
	page, err := s.List(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || page.Rows[0].Name != "sample" || len(page.Columns) != 4 || page.Rows[0].Cells[1] != "9007199254740993" {
		t.Fatalf("table mismatch: %+v", page)
	}
	query.Version = "v2"
	page, err = s.List(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Columns) != 2 || page.Columns[1].Name != "V2 Label" {
		t.Fatalf("version-specific columns missing: %+v", page.Columns)
	}
}

func waitEstablished(t *testing.T, ctx context.Context, api dynamic.ResourceInterface, name string) {
	t.Helper()
	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		definition, err := api.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		conditions, _, _ := unstructured.NestedSlice(definition.Object, "status", "conditions")
		for _, raw := range conditions {
			condition, ok := raw.(map[string]interface{})
			if ok && condition["type"] == "Established" && condition["status"] == "True" {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func checkCustomReadPermissions(t *testing.T, ctx context.Context, client dynamic.Interface, config *rest.Config) {
	t.Helper()
	for _, item := range []struct{ resource, text string }{
		{"roles", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"Role","metadata":{"name":"custom-reader","namespace":"default"},"rules":[{"apiGroups":["creals.test"],"resources":["gadgets"],"verbs":["get","list"]}]}`},
		{"rolebindings", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"RoleBinding","metadata":{"name":"custom-reader","namespace":"default"},"subjects":[{"kind":"User","name":"creals-custom-reader","apiGroup":"rbac.authorization.k8s.io"}],"roleRef":{"kind":"Role","name":"custom-reader","apiGroup":"rbac.authorization.k8s.io"}}`},
	} {
		object := &unstructured.Unstructured{}
		if err := object.UnmarshalJSON([]byte(item.text)); err != nil {
			t.Fatal(err)
		}
		api := client.Resource(schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: item.resource}).Namespace("default")
		if _, err := api.Create(ctx, object, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	limited := rest.CopyConfig(config)
	limited.Impersonate = rest.ImpersonationConfig{UserName: "creals-custom-reader", Groups: []string{"system:authenticated"}}
	connection, err := (kube.Factory{}).New(ctx, limited)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	writer := connection.(manifest.Client)
	manifests := manifest.NewService(resolver{writer})
	defer manifests.ServiceShutdown()
	service := customresource.NewService(resolver{writer}, manifests)
	target := resource.Ref{ClusterID: "test-only", Group: "creals.test", Version: "v1", Resource: "gadgets", Namespace: "default", Name: "sample"}
	if _, err := service.Definition(ctx, target); err == nil || !strings.Contains(err.Error(), "权限不足") {
		t.Fatalf("CRD permissions: %v", err)
	}
	if _, err := service.Snapshot(ctx, target); err == nil {
		t.Fatal("form read without definition permission")
	}
	if _, err := manifests.Snapshot(ctx, target, false); err != nil {
		t.Fatalf("generic YAML fallback failed: %v", err)
	}
	page, err := connection.List(ctx, resource.Query{ClusterID: target.ClusterID, Group: target.Group, Version: target.Version, Resource: target.Resource, Namespace: target.Namespace})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("generic list fallback failed: %v", err)
	}
}
