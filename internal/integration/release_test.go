//go:build integration

package integration

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workload"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var releaseTarget = resource.Ref{ClusterID: "test-only", Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default", Name: "form-app"}

func deploymentAPI(client dynamic.Interface) dynamic.ResourceInterface {
	return client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).Namespace("default")
}
func getRelease(t *testing.T, ctx context.Context, api dynamic.ResourceInterface, name string) *unstructured.Unstructured {
	t.Helper()
	object, err := api.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return object
}
func patchRelease(t *testing.T, ctx context.Context, api dynamic.ResourceInterface, name, text string) {
	t.Helper()
	if _, err := api.Patch(ctx, name, types.MergePatchType, []byte(text), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
}

func checkScaleRBAC(t *testing.T, ctx context.Context, config *rest.Config, client dynamic.Interface) {
	t.Helper()
	api := deploymentAPI(client)
	for _, verb := range []string{"update", "patch"} {
		t.Run(verb+"-scale-only", func(t *testing.T) {
			user := "creals-scale-" + verb
			grantScale(t, ctx, client, user, verb)
			limited := rest.CopyConfig(config)
			limited.Impersonate = rest.ImpersonationConfig{UserName: user, Groups: []string{"system:authenticated"}}
			connection, err := (kube.Factory{}).New(ctx, limited)
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			writer := connection.(manifest.Client)
			s := manifest.NewService(resolver{writer})
			defer s.ServiceShutdown()
			if _, err := writer.GetObject(ctx, releaseTarget); err == nil {
				t.Fatal("limited user can read parent")
			}
			if _, err := connection.List(ctx, resource.Query{Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default"}); err == nil {
				t.Fatal("limited user can list parent")
			}
			access, err := workload.NewService(resolver{writer}, s).Permissions(ctx, releaseTarget)
			if err != nil {
				t.Fatal(err)
			}
			for _, permission := range access {
				expected := "denied"
				if permission.Subresource == "scale" && (permission.Verb == "get" || permission.Verb == verb) {
					expected = "allowed"
				}
				if permission.State != expected {
					t.Fatalf("access review: %+v, expected %s", permission, expected)
				}
			}
			before := getRelease(t, ctx, api, releaseTarget.Name)
			snapshot, err := s.Scale(ctx, releaseTarget)
			if err != nil {
				t.Fatal(err)
			}
			request := manifest.ScaleRequest{Target: releaseTarget, Baseline: snapshot.Baseline, Replicas: 4, Method: verb}
			preview, err := s.PreviewScale(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, getRelease(t, ctx, api, releaseTarget.Name)) {
				t.Fatal("scale dry run persisted")
			}
			patchRelease(t, ctx, api, releaseTarget.Name, `{"metadata":{"annotations":{"scale-concurrent":"`+verb+`"}}}`)
			if _, err := s.Commit(ctx, preview.Token); err == nil {
				t.Fatal("scale ignored parent concurrent change")
			}
			snapshot, err = s.Scale(ctx, releaseTarget)
			if err != nil {
				t.Fatal(err)
			}
			request.Baseline = snapshot.Baseline
			preview, err = s.PreviewScale(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Commit(ctx, preview.Token); err != nil {
				t.Fatal(err)
			}
			after := getRelease(t, ctx, api, releaseTarget.Name)
			replicas, _, _ := unstructured.NestedInt64(after.Object, "spec", "replicas")
			oldTemplate, _, _ := unstructured.NestedMap(before.Object, "spec", "template")
			newTemplate, _, _ := unstructured.NestedMap(after.Object, "spec", "template")
			if replicas != 4 || !reflect.DeepEqual(oldTemplate, newTemplate) {
				t.Fatal("scale lost template or replicas")
			}
			snapshot, err = s.Scale(ctx, releaseTarget)
			if err != nil {
				t.Fatal(err)
			}
			request.Baseline = snapshot.Baseline
			if verb == "update" {
				request.Method = "patch"
			} else {
				request.Method = "update"
			}
			if _, err := s.PreviewScale(ctx, request); err == nil || !strings.Contains(err.Error(), "权限不足") {
				t.Fatalf("wrong scale verb accepted: %v", err)
			}
		})
	}
}
func grantScale(t *testing.T, ctx context.Context, client dynamic.Interface, user, verb string) {
	t.Helper()
	for _, definition := range []struct{ resource, text string }{
		{"roles", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"Role","metadata":{"name":"` + user + `","namespace":"default"},"rules":[{"apiGroups":["apps"],"resources":["deployments/scale"],"resourceNames":["form-app"],"verbs":["get","` + verb + `"]}]}`},
		{"rolebindings", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"RoleBinding","metadata":{"name":"` + user + `","namespace":"default"},"subjects":[{"kind":"User","name":"` + user + `","apiGroup":"rbac.authorization.k8s.io"}],"roleRef":{"kind":"Role","name":"` + user + `","apiGroup":"rbac.authorization.k8s.io"}}`},
	} {
		object := &unstructured.Unstructured{}
		if err := object.UnmarshalJSON([]byte(definition.text)); err != nil {
			t.Fatal(err)
		}
		api := client.Resource(schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: definition.resource}).Namespace("default")
		if _, err := api.Create(ctx, object, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
}

func checkReleaseHistory(t *testing.T, ctx context.Context, manifests *manifest.Service, writer manifest.Client, client dynamic.Interface) {
	t.Helper()
	service := workload.NewService(resolver{writer}, manifests)
	api := deploymentAPI(client)
	deployment := getRelease(t, ctx, api, releaseTarget.Name)
	source := createRevision(t, ctx, client, deployment)
	request := workload.RollbackRequest{Target: releaseTarget, Baseline: formedit.Baseline(deployment), ReplicaSet: source.GetName(), Source: formedit.Baseline(source), Revision: "9007199254740993"}
	history, err := service.History(ctx, releaseTarget)
	if err != nil || len(history.Revisions) != 1 || history.Revisions[0].Revision != request.Revision || history.Revisions[0].Current {
		t.Fatalf("history: %+v %v", history, err)
	}
	rsAPI := client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}).Namespace("default")
	preview, err := service.PreviewRollback(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(deployment, getRelease(t, ctx, api, releaseTarget.Name)) {
		t.Fatal("rollback dry run persisted")
	}
	patchRelease(t, ctx, rsAPI, source.GetName(), `{"metadata":{"annotations":{"concurrent":"change"}}}`)
	if _, err := manifests.Commit(ctx, preview.Token); err == nil {
		t.Fatal("changed history source accepted")
	}
	source = getRelease(t, ctx, rsAPI, source.GetName())
	request.Source = formedit.Baseline(source)
	preview, err = service.PreviewRollback(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	patchRelease(t, ctx, api, releaseTarget.Name, `{"metadata":{"annotations":{"rollback-concurrent":"change"}}}`)
	if _, err := manifests.Commit(ctx, preview.Token); err == nil {
		t.Fatal("changed Deployment accepted")
	}
	deployment = getRelease(t, ctx, api, releaseTarget.Name)
	request.Baseline = formedit.Baseline(deployment)
	preview, err = service.PreviewRollback(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.After, "9007199254740993") || strings.Contains(preview.After, "pod-template-hash") {
		t.Fatal("rollback lost int64 or retained controller label")
	}
	if _, err := manifests.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	after := getRelease(t, ctx, api, releaseTarget.Name)
	expectedTemplate, _, _ := unstructured.NestedMap(source.Object, "spec", "template")
	unstructured.RemoveNestedField(expectedTemplate, "metadata", "labels", "pod-template-hash")
	actualTemplate, _, _ := unstructured.NestedMap(after.Object, "spec", "template")
	if !reflect.DeepEqual(expectedTemplate, actualTemplate) {
		t.Fatalf("rollback template mismatch: %#v / %#v", expectedTemplate, actualTemplate)
	}
	beforeSpec, _, _ := unstructured.NestedMap(deployment.Object, "spec")
	afterSpec, _, _ := unstructured.NestedMap(after.Object, "spec")
	delete(beforeSpec, "template")
	delete(afterSpec, "template")
	if !reflect.DeepEqual(beforeSpec, afterSpec) || !reflect.DeepEqual(deployment.GetAnnotations(), after.GetAnnotations()) {
		t.Fatal("rollback changed parent configuration")
	}
	checkReleasePause(t, ctx, service, manifests, api, request)
	// A deleted history source cannot be committed, even though the ticket holds its template.
	patchRelease(t, ctx, api, releaseTarget.Name, `{"spec":{"template":{"metadata":{"annotations":{"after-rollback":"new"}}}}}`)
	request.Baseline = formedit.Baseline(getRelease(t, ctx, api, releaseTarget.Name))
	preview, err = service.PreviewRollback(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsAPI.Delete(ctx, source.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := manifests.Commit(ctx, preview.Token); err == nil {
		t.Fatal("deleted history source accepted")
	}
}
func createRevision(t *testing.T, ctx context.Context, client dynamic.Interface, deployment *unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()
	rs := deployment.DeepCopy()
	rs.SetKind("ReplicaSet")
	rs.SetName("form-app-previous")
	rs.SetUID("")
	rs.SetResourceVersion("")
	rs.SetGeneration(0)
	rs.SetManagedFields(nil)
	rs.SetCreationTimestamp(metav1.Time{})
	rs.SetAnnotations(map[string]string{"deployment.kubernetes.io/revision": "9007199254740993"})
	rs.SetLabels(map[string]string{"app.kubernetes.io/name": "form-app"})
	controller := true
	rs.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.GetName(), UID: deployment.GetUID(), Controller: &controller}})
	unstructured.RemoveNestedField(rs.Object, "status")
	unstructured.RemoveNestedField(rs.Object, "spec", "strategy")
	unstructured.RemoveNestedField(rs.Object, "spec", "progressDeadlineSeconds")
	unstructured.RemoveNestedField(rs.Object, "spec", "revisionHistoryLimit")
	_ = unstructured.SetNestedField(rs.Object, int64(0), "spec", "replicas")
	_ = unstructured.SetNestedField(rs.Object, "old-hash", "spec", "template", "metadata", "labels", "pod-template-hash")
	_ = unstructured.SetNestedField(rs.Object, int64(9007199254740993), "spec", "template", "spec", "terminationGracePeriodSeconds")
	_ = unstructured.SetNestedField(rs.Object, "historical", "spec", "template", "metadata", "annotations", "historical")
	api := client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}).Namespace("default")
	result, err := api.Create(ctx, rs, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func checkReleasePause(t *testing.T, ctx context.Context, service *workload.Service, manifests *manifest.Service, api dynamic.ResourceInterface, rollback workload.RollbackRequest) {
	t.Helper()
	for _, action := range []string{"pause", "resume"} {
		before := getRelease(t, ctx, api, releaseTarget.Name)
		preview, err := service.Preview(ctx, workload.Request{Target: releaseTarget, Baseline: formedit.Baseline(before), Action: action})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, getRelease(t, ctx, api, releaseTarget.Name)) {
			t.Fatal("pause dry run persisted")
		}
		if _, err := manifests.Commit(ctx, preview.Token); err != nil {
			t.Fatal(err)
		}
		after := getRelease(t, ctx, api, releaseTarget.Name)
		paused, _, _ := unstructured.NestedBool(after.Object, "spec", "paused")
		if paused != (action == "pause") {
			t.Fatal("pause/resume not persisted")
		}
		if paused {
			rollback.Baseline = formedit.Baseline(after)
			if _, err := service.PreviewRollback(ctx, rollback); err == nil || !strings.Contains(err.Error(), "已暂停") {
				t.Fatalf("paused rollback: %v", err)
			}
		}
	}
}

func checkOtherScaleResources(t *testing.T, ctx context.Context, manifests *manifest.Service, client dynamic.Interface) {
	t.Helper()
	for _, item := range []struct{ group, resource, kind string }{{"apps", "statefulsets", "StatefulSet"}, {"apps", "replicasets", "ReplicaSet"}, {"", "replicationcontrollers", "ReplicationController"}} {
		t.Run(item.resource, func(t *testing.T) {
			object := &unstructured.Unstructured{}
			version := "v1"
			if item.group != "" {
				version = item.group + "/v1"
			}
			text := `{"apiVersion":"` + version + `","kind":"` + item.kind + `","metadata":{"name":"scale-check","namespace":"default"},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"scale-check"}},"template":{"metadata":{"labels":{"app":"scale-check"}},"spec":{"containers":[{"name":"app","image":"nginx:1.29"}]}}}}`
			if err := object.UnmarshalJSON([]byte(text)); err != nil {
				t.Fatal(err)
			}
			if item.resource == "statefulsets" {
				_ = unstructured.SetNestedField(object.Object, "scale-check", "spec", "serviceName")
			}
			if item.group == "" {
				_ = unstructured.SetNestedStringMap(object.Object, map[string]string{"app": "scale-check"}, "spec", "selector")
			}
			target := resource.Ref{ClusterID: "test-only", Group: item.group, Version: "v1", Resource: item.resource, Namespace: "default", Name: "scale-check"}
			api := client.Resource(schema.GroupVersionResource{Group: item.group, Version: "v1", Resource: item.resource}).Namespace("default")
			if _, err := api.Create(ctx, object, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{"update", "patch"} {
				snapshot, err := manifests.Scale(ctx, target)
				if err != nil {
					t.Fatal(err)
				}
				preview, err := manifests.PreviewScale(ctx, manifest.ScaleRequest{Target: target, Baseline: snapshot.Baseline, Replicas: 0, Method: method})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := manifests.Commit(ctx, preview.Token); err != nil {
					t.Fatal(err)
				}
				actual, _, _ := unstructured.NestedInt64(getRelease(t, ctx, api, target.Name).Object, "spec", "replicas")
				if actual != 0 {
					t.Fatal("zero replicas lost")
				}
			}
			snapshot, err := manifests.Scale(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := manifests.PreviewScale(ctx, manifest.ScaleRequest{Target: target, Baseline: snapshot.Baseline, Replicas: 2, Method: "update"})
			if err != nil {
				t.Fatal(err)
			}
			policy := metav1.DeletePropagationOrphan
			if err := api.Delete(ctx, target.Name, metav1.DeleteOptions{PropagationPolicy: &policy}); err != nil {
				t.Fatal(err)
			}
			// No garbage collector runs in envtest; clear only its test object's orphan finalizer.
			existing, err := api.Get(ctx, target.Name, metav1.GetOptions{})
			if err == nil && existing.GetDeletionTimestamp() != nil {
				patchRelease(t, ctx, api, target.Name, `{"metadata":{"finalizers":null}}`)
			}
			if _, err := api.Create(ctx, object, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			if _, err := manifests.Commit(ctx, preview.Token); err == nil {
				t.Fatal("scale ticket crossed object recreation")
			}
		})
	}
}
