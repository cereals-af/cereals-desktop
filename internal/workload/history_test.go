package workload

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func historical(t *testing.T, deployment *unstructured.Unstructured, name, revision string) *unstructured.Unstructured {
	t.Helper()
	rs := deployment.DeepCopy()
	rs.SetKind("ReplicaSet")
	rs.SetName(name)
	rs.SetUID(types.UID(name))
	rs.SetAnnotations(map[string]string{revisionAnnotation: revision})
	controller := true
	rs.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.GetName(), UID: deployment.GetUID(), Controller: &controller}})
	_ = unstructured.SetNestedField(rs.Object, "controller-hash", "spec", "template", "metadata", "labels", "pod-template-hash")
	return rs
}
func TestRollbackRestoresFullTemplateAndPreservesParent(t *testing.T) {
	deployment := fixture(t)
	rs := historical(t, deployment, "old", "9007199254740993")
	_ = unstructured.SetNestedField(deployment.Object, "current", "spec", "template", "metadata", "annotations", "external")
	_ = unstructured.SetNestedField(deployment.Object, int64(7), "spec", "replicas")
	original := deployment.DeepCopy()
	source := rs.DeepCopy()
	got, err := rollbackObject(deployment, rs)
	if err != nil {
		t.Fatal(err)
	}
	expected := original.DeepCopy()
	template, _, _ := unstructured.NestedMap(source.Object, "spec", "template")
	unstructured.RemoveNestedField(template, "metadata", "labels", "pod-template-hash")
	_ = unstructured.SetNestedMap(expected.Object, template, "spec", "template")
	if !reflect.DeepEqual(got, expected) || !reflect.DeepEqual(deployment, original) || !reflect.DeepEqual(rs, source) {
		t.Fatal("rollback lost fields, precision, or mutated inputs")
	}
	if _, err := rollbackObject(got, rs); err == nil {
		t.Fatal("unchanged template accepted")
	}
	_ = unstructured.SetNestedField(deployment.Object, true, "spec", "paused")
	if _, err := rollbackObject(deployment, rs); err == nil {
		t.Fatal("paused rollback accepted")
	}
	unstructured.RemoveNestedField(deployment.Object, "spec", "paused")
	deployment.SetUID("recreated")
	if _, err := rollbackObject(deployment, rs); err == nil {
		t.Fatal("old owner UID accepted")
	}
}

type pagedHistory struct {
	testClient
	pages []*unstructured.UnstructuredList
	calls int
	fail  bool
}

func (c *pagedHistory) ListObjects(_ context.Context, target resource.Ref, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if target.Resource != "replicasets" || options.LabelSelector != "app=app" || options.Limit != 100 {
		return nil, errors.New("incorrect scoped query")
	}
	if c.calls > 0 && c.fail {
		return nil, errors.New("permission revoked")
	}
	page := c.pages[min(c.calls, len(c.pages)-1)]
	c.calls++
	return page, nil
}
func TestHistoryOwnershipPaginationSortingAndPartialFailure(t *testing.T) {
	deployment := fixture(t)
	old := historical(t, deployment, "old", "9")
	current := historical(t, deployment, "current", "9007199254740993")
	wrong := historical(t, deployment, "wrong", "10")
	wrong.SetOwnerReferences(nil)
	first := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{*old, *wrong}}
	first.SetContinue("page2")
	second := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{*old, *current}}
	client := &pagedHistory{testClient: testClient{object: deployment}, pages: []*unstructured.UnstructuredList{first, second}}
	s := NewService(testResolver{client}, &captureWriter{})
	target := resource.Ref{ClusterID: "one", Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default", Name: "app"}
	history, err := s.History(t.Context(), target)
	if err != nil || len(history.Revisions) != 2 || history.Revisions[0].Revision != "9007199254740993" || !history.Revisions[0].Current {
		t.Fatalf("history: %+v, %v", history, err)
	}
	client.calls, client.fail = 0, true
	history, err = s.History(t.Context(), target)
	if err != nil || len(history.Warnings) != 1 || len(history.Revisions) != 1 {
		t.Fatal("partial failure hidden")
	}
	client.calls, client.fail, client.pages = 0, false, []*unstructured.UnstructuredList{first}
	history, err = s.History(t.Context(), target)
	if err != nil || !history.Truncated || client.calls != 2 {
		t.Fatal("repeated continuation not bounded")
	}
}
func TestPauseResumePreserveAllOtherFields(t *testing.T) {
	object := fixture(t)
	writer := &captureWriter{}
	s := NewService(testResolver{testClient{object: object}}, writer)
	target := resource.Ref{ClusterID: "one", Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default", Name: "app"}
	for _, action := range []string{"pause", "resume"} {
		before := object.DeepCopy()
		preview, err := s.Preview(t.Context(), Request{Target: target, Baseline: formedit.Baseline(object), Action: action})
		if err != nil {
			t.Fatal(err)
		}
		got := &unstructured.Unstructured{}
		if err := got.UnmarshalJSON([]byte(preview.Desired)); err != nil {
			t.Fatal(err)
		}
		_ = unstructured.SetNestedField(before.Object, action == "pause", "spec", "paused")
		if !reflect.DeepEqual(got, before) {
			t.Fatal("pause changed unrelated fields")
		}
		object.Object = got.Object
		if _, err := s.Preview(t.Context(), Request{Target: target, Baseline: manifest.Baseline{UID: string(object.GetUID()), ResourceVersion: object.GetResourceVersion()}, Action: action}); err == nil {
			t.Fatal("no-op pause accepted")
		}
	}
}
