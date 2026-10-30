package relations

import (
	"context"
	"errors"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type storageClient struct {
	*fakeClient
	objects map[string]*unstructured.Unstructured
}

func (c storageClient) GetObject(_ context.Context, ref resource.Ref) (*unstructured.Unstructured, error) {
	if object := c.objects[ref.Resource]; object != nil {
		return object, nil
	}
	return nil, errors.New("权限不足")
}
func TestStorageRelationsRejectReplacedClaimAndKeepPartialResults(t *testing.T) {
	claim := object("PersistentVolumeClaim", "data", "claim-new", nil)
	claim.Object["spec"] = map[string]interface{}{"volumeName": "volume", "storageClassName": "fast"}
	pv := object("PersistentVolume", "volume", "pv-uid", nil)
	pv.SetNamespace("")
	pv.Object["spec"] = map[string]interface{}{"claimRef": map[string]interface{}{"name": "data", "namespace": "team", "uid": "claim-old"}}
	pod := object("Pod", "consumer", "pod", nil)
	pod.Object["spec"] = map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"name": "data", "persistentVolumeClaim": map[string]interface{}{"claimName": "data"}}}}
	foreign := pod.DeepCopy()
	foreign.SetNamespace("other")
	client := storageClient{objects: map[string]*unstructured.Unstructured{"persistentvolumeclaims": &claim, "persistentvolumes": &pv}, fakeClient: &fakeClient{list: func(ref resource.Ref, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
		if ref.Resource == "events" {
			return page(), nil
		}
		if ref.Namespace != "team" {
			t.Fatalf("scope leaked: %+v", ref)
		}
		if options.Continue != "" {
			return nil, errors.New("later page denied")
		}
		result := page(pod, *foreign)
		result.SetContinue("next")
		return result, nil
	}}}
	report, err := NewService(&fakeResolver{client: client}).Inspect(t.Context(), target("", "persistentvolumeclaims", "data"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Sections[0].Error == "" || len(report.Sections[0].Items) != 0 || report.Sections[1].Error == "" || report.Sections[2].Error == "" || len(report.Sections[2].Items) != 1 {
		t.Fatalf("partial/binding errors concealed: %+v", report)
	}
	pv.Object["spec"].(map[string]interface{})["claimRef"].(map[string]interface{})["uid"] = "claim-new"
	if !boundTo(&pv, &claim) {
		t.Fatal("valid binding rejected")
	}
}
