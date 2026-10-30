package formedit

import (
	"context"
	"errors"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type testClient struct {
	manifest.Client
	object *unstructured.Unstructured
}

func (c testClient) GetObject(ctx context.Context, _ resource.Ref) (*unstructured.Unstructured, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.object, nil
}

type testResolver struct{ client manifest.Client }

func (r testResolver) ManifestClient(string) (manifest.Client, error) { return r.client, nil }

type captureWriter struct{ requests []manifest.Request }

func (w *captureWriter) Preview(_ context.Context, request manifest.Request) (manifest.Preview, error) {
	w.requests = append(w.requests, request)
	return manifest.Preview{}, nil
}

func TestEditorGuardsBeforeEditingAndSharesManifestPipeline(t *testing.T) {
	object := &unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "test", "namespace": "default", "uid": "uid", "resourceVersion": "10"}}}
	writer := &captureWriter{}
	editor := New(testResolver{testClient{object: object}}, writer)
	ref := resource.Ref{ClusterID: "one", Version: "v1", Resource: "configmaps", Namespace: "default", Name: "test"}
	called := false
	edit := func(copy *unstructured.Unstructured) error {
		called = true
		copy.Object["data"] = map[string]interface{}{"key": "value"}
		return nil
	}
	if _, err := editor.Preview(t.Context(), ref, manifest.Baseline{}, "patch", "ConfigMap", edit); err == nil || called {
		t.Fatal("stale baseline reached mutation")
	}
	if _, err := editor.Preview(t.Context(), ref, Baseline(object), "patch", "ConfigMap", edit); err != nil {
		t.Fatal(err)
	}
	if object.Object["data"] != nil {
		t.Fatal("cached original object was mutated")
	}
	if len(writer.requests) != 1 || writer.requests[0].Baseline != Baseline(object) || writer.requests[0].Target != ref || writer.requests[0].Mode != "patch" {
		t.Fatal("manifest preconditions not forwarded")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := editor.Read(ctx, ref); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if _, err := Target(ref, "services", "test", "default"); err == nil {
		t.Fatal("cross-resource redirect accepted")
	}
	if _, err := Target(ref, "configmaps", "other", "default"); err == nil {
		t.Fatal("rename accepted")
	}
	if _, err := Target(ref, "configmaps", "test", "other"); err == nil {
		t.Fatal("cross-namespace redirect accepted")
	}
}
