package manifest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type scaleClient struct{ *testClient }

func (c scaleClient) GetScale(context.Context, resource.Ref) (*unstructured.Unstructured, error) {
	return c.object.DeepCopy(), nil
}

// Parent access intentionally panics: scale-only accounts must never need it.
func (c scaleClient) GetObject(context.Context, resource.Ref) (*unstructured.Unstructured, error) {
	panic("parent get")
}
func (c scaleClient) Describe(context.Context, resource.Ref) (resource.Type, error) {
	panic("parent discovery")
}

type clientResolver struct{ client Client }

func (r clientResolver) ManifestClient(string) (Client, error) { return r.client, nil }

func TestScaleUsesOnlySubresourceWithImmutablePreconditions(t *testing.T) {
	target := resource.Ref{ClusterID: "one", Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default", Name: "app"}
	object, err := parse("apiVersion: autoscaling/v1\nkind: Scale\nmetadata:\n  name: app\n  namespace: default\n  uid: original\n  resourceVersion: '13'\nspec:\n  replicas: 3\nstatus:\n  replicas: 2\n  selector: app=app\n")
	if err != nil {
		t.Fatal(err)
	}
	c := scaleClient{&testClient{id: "session", object: object}}
	s := NewService(clientResolver{c})
	defer s.ServiceShutdown()
	snapshot, err := s.Scale(t.Context(), target)
	if err != nil || snapshot.Replicas != 3 || snapshot.Actual != 2 {
		t.Fatalf("snapshot: %+v, %v", snapshot, err)
	}
	for _, method := range []string{"update", "patch"} {
		preview, err := s.PreviewScale(t.Context(), ScaleRequest{Target: target, Baseline: snapshot.Baseline, Replicas: 0, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		operation := c.writes[len(c.writes)-1]
		var payload map[string]interface{}
		if err := json.Unmarshal(operation.Data, &payload); err != nil {
			t.Fatal(err)
		}
		metadata := payload["metadata"].(map[string]interface{})
		if operation.Mode != method || operation.Subresource != "scale" || metadata["uid"] != "original" || metadata["resourceVersion"] != "13" || payload["status"] != nil {
			t.Fatalf("unsafe scale payload: %s", operation.Data)
		}
		if _, err := s.Commit(t.Context(), preview.Token); err != nil {
			t.Fatal(err)
		}
	}
	for _, request := range []ScaleRequest{
		{Target: target, Baseline: snapshot.Baseline, Replicas: -1, Method: "update"},
		{Target: target, Baseline: snapshot.Baseline, Replicas: 2, Method: "apply"},
		{Target: target, Baseline: Baseline{UID: "recreated", ResourceVersion: "13"}, Method: "update"},
	} {
		if _, err := s.PreviewScale(t.Context(), request); err == nil {
			t.Fatal("invalid scale request accepted")
		}
	}
	if len(c.writes) != 4 {
		t.Fatal("invalid request reached server")
	}
}

func TestDependencyGuardRejectsChangedSourceAndConsumesTicket(t *testing.T) {
	s, c := newTestService(t)
	request := Request{Target: testRef, Mode: "patch", YAML: testYAML, Baseline: baseline(c.object)}
	dependency := Dependency{Target: testRef, Baseline: baseline(c.object)}
	preview, err := s.PreviewGuarded(t.Context(), request, []Dependency{dependency})
	if err != nil {
		t.Fatal(err)
	}
	c.object.SetResourceVersion("14")
	if _, err := s.Commit(t.Context(), preview.Token); err == nil || len(c.writes) != 1 {
		t.Fatal("changed source reached write")
	}
	c.object.SetResourceVersion("12")
	if _, err := s.Commit(t.Context(), preview.Token); err == nil {
		t.Fatal("failed dependency ticket replayed")
	}
	dependency.Target.ClusterID = "other"
	if _, err := s.PreviewGuarded(t.Context(), request, []Dependency{dependency}); err == nil {
		t.Fatal("cross-cluster dependency accepted")
	}
}
