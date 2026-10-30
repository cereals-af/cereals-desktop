package workload

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func controllerFixture(t *testing.T, kind string) (*unstructured.Unstructured, resource.Ref) {
	object := fixture(t)
	object.SetKind(controllerKind(kind))
	target := resource.Ref{ClusterID: "test", Group: "apps", Version: "v1", Resource: kind, Name: object.GetName(), Namespace: object.GetNamespace()}
	if kind == "statefulsets" {
		_ = unstructured.SetNestedField(object.Object, "headless", "spec", "serviceName")
		_ = unstructured.SetNestedField(object.Object, int64(1), "spec", "updateStrategy", "rollingUpdate", "partition")
	}
	if kind == "daemonsets" {
		unstructured.RemoveNestedField(object.Object, "spec", "replicas")
	}
	return object, target
}
func TestControllerEditsPreserveWholeObject(t *testing.T) {
	for _, kind := range []string{"statefulsets", "daemonsets", "replicasets"} {
		t.Run(kind, func(t *testing.T) {
			object, target := controllerFixture(t, kind)
			snapshot, err := controllerSnapshot(object, target)
			if err != nil {
				t.Fatal(err)
			}
			before := object.DeepCopy()
			request := ControllerRequest{Target: target, Baseline: formedit.Baseline(object), Action: "update", Form: snapshot.Form}
			if err := updateController(object, request); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(object, before) {
				t.Fatal("unchanged form normalized object")
			}
			request.Form.Workload.Containers[0].Image = "nginx:new"
			if err := updateController(object, request); err != nil {
				t.Fatal(err)
			}
			containers, _, _ := unstructured.NestedSlice(before.Object, "spec", "template", "spec", "containers")
			containers[0].(map[string]interface{})["image"] = "nginx:new"
			_ = unstructured.SetNestedSlice(before.Object, containers, "spec", "template", "spec", "containers")
			if !reflect.DeepEqual(object, before) {
				t.Fatal("edit lost unexposed fields or int64")
			}
		})
	}
}
func TestControllerRejectsImmutableAndReplicaChanges(t *testing.T) {
	cases := map[string]func(*ControllerForm){"replicas": func(f *ControllerForm) { f.Workload.Replicas++ }, "service": func(f *ControllerForm) { f.ServiceName = "other" }, "order": func(f *ControllerForm) { f.PodManagementPolicy = "Parallel" }, "claims": func(f *ControllerForm) { f.Claims = append(f.Claims, ClaimTemplate{Name: "new"}) }, "partition": func(f *ControllerForm) { f.Partition = -1 }, "unknown strategy": func(f *ControllerForm) { f.Strategy = "Unknown" }, "invalid label": func(f *ControllerForm) { f.NodeSelector = []Label{{"!invalid", "x"}} }}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			object, target := controllerFixture(t, "statefulsets")
			snapshot, _ := controllerSnapshot(object, target)
			mutate(&snapshot.Form)
			if err := updateController(object, ControllerRequest{Target: target, Form: snapshot.Form}); err == nil {
				t.Fatal("invalid edit accepted")
			}
		})
	}
}
func TestControllerRevisionStrictFormatAndUID(t *testing.T) {
	object, _ := controllerFixture(t, "statefulsets")
	template, _, _ := unstructured.NestedMap(object.Object, "spec", "template")
	template["$patch"] = "replace"
	revision := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "apps/v1", "kind": "ControllerRevision", "metadata": map[string]interface{}{"name": "history", "namespace": object.GetNamespace(), "uid": "history-1", "resourceVersion": "1"}, "revision": int64(9007199254740993), "data": map[string]interface{}{"spec": map[string]interface{}{"template": template}}}}
	controller := true
	revision.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: object.GetKind(), Name: object.GetName(), UID: object.GetUID(), Controller: &controller}})
	if !ownsControllerRevision(object, revision) {
		t.Fatal("valid owner rejected")
	}
	summary := controllerRevisionSummary(object, revision)
	if summary.Revision != "9007199254740993" || !summary.Current || summary.Unavailable != "" {
		t.Fatalf("summary: %+v", summary)
	}
	parsed, err := controllerRevisionTemplate(revision)
	if err != nil {
		t.Fatal(err)
	}
	value, _, _ := unstructured.NestedInt64(parsed, "spec", "securityContext", "runAsUser")
	if value != 9007199254740993 {
		t.Fatal("history precision lost")
	}
	object.SetUID("replacement")
	if ownsControllerRevision(object, revision) {
		t.Fatal("same-name replacement accepted")
	}
	for _, path := range [][]string{{"data", "metadata"}, {"data", "spec", "replicas"}, {"data", "spec", "template", "spec", "$patch"}} {
		copy := revision.DeepCopy()
		_ = unstructured.SetNestedField(copy.Object, "unexpected", path...)
		if _, err := controllerRevisionTemplate(copy); err == nil {
			t.Fatal("unknown historical mutation accepted")
		}
	}
	copy := revision.DeepCopy()
	unstructured.RemoveNestedField(copy.Object, "data", "spec", "template", "$patch")
	if _, err := controllerRevisionTemplate(copy); err == nil {
		t.Fatal("missing replace directive accepted")
	}
}
func TestCreateControllerClaimsAndOnDelete(t *testing.T) {
	object, target := controllerFixture(t, "statefulsets")
	snapshot, _ := controllerSnapshot(object, target)
	form := snapshot.Form
	form.Claims = []ClaimTemplate{{Name: "data", Capacity: "1Gi", ClassMode: "none", AccessModes: []string{"ReadWriteOnce"}, VolumeMode: "Filesystem", MountPath: "/data"}}
	form.Strategy = "OnDelete"
	form.Workload.Containers = []Container{{Name: "app", Image: "nginx"}}
	created, err := newController(ControllerRequest{Target: target, Form: form})
	if err != nil {
		t.Fatal(err)
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(created.Object, "spec", "updateStrategy", "rollingUpdate"); found {
		t.Fatal("OnDelete retained rolling strategy")
	}
	raw, _ := created.MarshalJSON()
	if !strings.Contains(string(raw), `"storageClassName":""`) || !strings.Contains(string(raw), `"mountPath":"/data"`) {
		t.Fatal("PVC semantics missing")
	}
	form.Claims[0].Capacity = "0"
	if _, err = newController(ControllerRequest{Target: target, Form: form}); err == nil {
		t.Fatal("zero claim capacity accepted")
	}
}

type controllerPages struct {
	testClient
	page  *unstructured.UnstructuredList
	calls int
	fail  bool
}

func (c *controllerPages) ListObjects(_ context.Context, target resource.Ref, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if target.Resource != "controllerrevisions" || target.Namespace != "default" || options.Limit != 100 {
		return nil, errors.New("incorrect history scope")
	}
	c.calls++
	if c.fail && c.calls > 1 {
		return nil, errors.New("permission denied")
	}
	return c.page, nil
}
func TestControllerHistoryPartialFailureAndRepeatedCursor(t *testing.T) {
	object, target := controllerFixture(t, "daemonsets")
	template, _, _ := unstructured.NestedMap(object.Object, "spec", "template")
	template["$patch"] = "replace"
	revision := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "apps/v1", "kind": "ControllerRevision", "metadata": map[string]interface{}{"name": "history", "namespace": "default", "uid": "history", "resourceVersion": "1"}, "revision": int64(2), "data": map[string]interface{}{"spec": map[string]interface{}{"template": template}}}}
	controller := true
	revision.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: object.GetKind(), Name: object.GetName(), UID: object.GetUID(), Controller: &controller}})
	foreign := revision.DeepCopy()
	foreign.SetUID("foreign")
	foreign.SetOwnerReferences(nil)
	page := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{*revision, *foreign}}
	page.SetContinue("again")
	reader := &controllerPages{testClient: testClient{object: object}, page: page, fail: true}
	service := NewService(testResolver{reader}, &captureWriter{})
	history, err := service.ControllerHistory(t.Context(), target)
	if err != nil || len(history.Revisions) != 1 || len(history.Warnings) != 1 {
		t.Fatalf("partial result hidden: %+v %v", history, err)
	}
	reader.calls = 0
	reader.fail = false
	history, err = service.ControllerHistory(t.Context(), target)
	if err != nil || !history.Truncated || len(history.Revisions) != 1 || reader.calls != 2 {
		t.Fatalf("unbounded or duplicate history: %+v calls=%d %v", history, reader.calls, err)
	}
}
