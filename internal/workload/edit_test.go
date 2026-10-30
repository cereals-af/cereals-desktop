package workload

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func fixture(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	object := &unstructured.Unstructured{}
	err := object.UnmarshalJSON([]byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"app","namespace":"default","uid":"uid-1","resourceVersion":"10","generation":5,"annotations":{"external":"keep"}},"spec":{"replicas":3,"selector":{"matchLabels":{"app":"app"}},"template":{"metadata":{"labels":{"app":"app"},"annotations":{"external":"keep"}},"spec":{"securityContext":{"runAsUser":9007199254740993},"initContainers":[{"name":"init","image":"busybox"}],"volumes":[{"name":"data","emptyDir":{}}],"containers":[{"name":"app","image":"nginx:old","resources":{"requests":{"cpu":"100m","ephemeral-storage":"1Gi"},"limits":{"example.com/gpu":"1"}},"env":[{"name":"PLAIN","value":"before","futureField":"keep"},{"name":"TOKEN","valueFrom":{"secretKeyRef":{"name":"token","key":"value"}}}],"envFrom":[{"configMapRef":{"name":"config"}}],"readinessProbe":{"httpGet":{"path":"/ready","port":"http","scheme":"HTTP","httpHeaders":[{"name":"X-Probe","value":"yes"}]},"timeoutSeconds":1,"periodSeconds":10,"successThreshold":1,"failureThreshold":3},"ports":[{"containerPort":8080,"name":"http"}],"futureField":{"number":9007199254740993}}]}}},"status":{"observedGeneration":4,"replicas":3,"updatedReplicas":3,"readyReplicas":3,"availableReplicas":3}}`))
	if err != nil {
		t.Fatal(err)
	}
	return object
}

func TestFormPreservesUnexposedFieldsAndInt64(t *testing.T) {
	object := fixture(t)
	before := object.DeepCopy()
	snapshot, err := snapshot(object)
	if err != nil {
		t.Fatal(err)
	}
	form := snapshot.Form
	form.Containers[0].Image = "nginx:new"
	form.Containers[0].CPURequest = "200m"
	form.Containers[0].Environment[0].Value = "after"
	form.Containers[0].Readiness.Path = "/health"
	if err := updateDeployment(object, form); err != nil {
		t.Fatal(err)
	}
	// Independently build the expected complete object. Everything else must remain byte-for-byte equivalent.
	containers, _, _ := unstructured.NestedSlice(before.Object, "spec", "template", "spec", "containers")
	container := containers[0].(map[string]interface{})
	container["image"] = "nginx:new"
	_ = unstructured.SetNestedField(container, "200m", "resources", "requests", "cpu")
	env := container["env"].([]interface{})
	env[0].(map[string]interface{})["value"] = "after"
	_ = unstructured.SetNestedField(container, "/health", "readinessProbe", "httpGet", "path")
	_ = unstructured.SetNestedField(container, int64(0), "readinessProbe", "initialDelaySeconds")
	_ = unstructured.SetNestedSlice(before.Object, containers, "spec", "template", "spec", "containers")
	if !reflect.DeepEqual(before.Object, object.Object) {
		t.Fatalf("unexposed fields changed: %#v", object.Object)
	}
}

func TestUnchangedFormDoesNotNormalizeStoredFields(t *testing.T) {
	object := fixture(t)
	original := object.DeepCopy()
	form, err := snapshot(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := updateDeployment(object, form.Form); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, object) {
		t.Fatal("unchanged form rewrote stored fields")
	}
}

func TestFormRejectsAmbiguousEdits(t *testing.T) {
	cases := map[string]func(*Deployment){
		"rename":              func(d *Deployment) { d.Name = "other" },
		"remove container":    func(d *Deployment) { d.Containers = nil },
		"rename container":    func(d *Deployment) { d.Containers[0].Name = "other" },
		"negative replicas":   func(d *Deployment) { d.Replicas = -1 },
		"bad quantity":        func(d *Deployment) { d.Containers[0].CPULimit = "invalid" },
		"negative quantity":   func(d *Deployment) { d.Containers[0].MemoryLimit = "-1Mi" },
		"delete reference":    func(d *Deployment) { d.Containers[0].Environment = d.Containers[0].Environment[:1] },
		"overwrite reference": func(d *Deployment) { d.Containers[0].Environment[1].Value = "secret" },
		"duplicate env": func(d *Deployment) {
			d.Containers[0].Environment = append(d.Containers[0].Environment, d.Containers[0].Environment[0])
		},
		"invalid probe": func(d *Deployment) { d.Containers[0].Readiness.Timeout = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			object := fixture(t)
			form, err := snapshot(object)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&form.Form)
			if err := updateDeployment(object, form.Form); err == nil {
				t.Fatal("invalid form accepted")
			}
		})
	}
}

type testClient struct {
	manifest.Client
	object *unstructured.Unstructured
}

func (c testClient) GetObject(ctx context.Context, _ resource.Ref) (*unstructured.Unstructured, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.object.DeepCopy(), nil
}

type testResolver struct{ client manifest.Client }

func (r testResolver) ManifestClient(string) (manifest.Client, error) { return r.client, nil }

type captureWriter struct{ requests []manifest.Request }

func (w *captureWriter) Preview(_ context.Context, request manifest.Request) (manifest.Preview, error) {
	w.requests = append(w.requests, request)
	return manifest.Preview{Desired: request.YAML}, nil
}

func TestActionsSharePreviewAndPreserveBaseline(t *testing.T) {
	object := fixture(t)
	writer := &captureWriter{}
	service := NewService(testResolver{testClient{object: object}}, writer)
	target := resource.Ref{ClusterID: "one", Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default", Name: "app"}
	snapshot, err := service.Deployment(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status.Observed || snapshot.Status.Phase == "已就绪" {
		t.Fatal("stale controller status reported ready")
	}
	for _, action := range []string{"update", "restart"} {
		request := Request{Target: target, Baseline: snapshot.Baseline, Action: action, Form: snapshot.Form}
		request.Form.Replicas = 0
		if _, err := service.Preview(t.Context(), request); err != nil {
			t.Fatal(err)
		}
		got := writer.requests[len(writer.requests)-1]
		if got.Mode != "patch" || got.Target != target || got.Baseline != snapshot.Baseline || !strings.Contains(got.YAML, "9007199254740993") {
			t.Fatalf("lost write contract: %#v", got)
		}
		decoded := &unstructured.Unstructured{}
		if err := decoded.UnmarshalJSON([]byte(got.YAML)); err != nil {
			t.Fatal(err)
		}
		expected := object.DeepCopy()
		if action == "restart" {
			stamp, _, _ := unstructured.NestedString(decoded.Object, "spec", "template", "metadata", "annotations", "kubectl.kubernetes.io/restartedAt")
			if stamp == "" {
				t.Fatal("restart did not change template")
			}
			unstructured.RemoveNestedField(decoded.Object, "spec", "template", "metadata", "annotations", "kubectl.kubernetes.io/restartedAt")
		} else {
			_ = setReplicas(expected, 0)
		}
		if !reflect.DeepEqual(decoded, expected) {
			t.Fatal("action changed unrelated fields")
		}
	}
	request := Request{Target: target, Baseline: snapshot.Baseline, Action: "restart"}
	request.Baseline.ResourceVersion = "stale"
	if _, err := service.Preview(t.Context(), request); err == nil {
		t.Fatal("stale baseline accepted")
	}
	request.Baseline = snapshot.Baseline
	_ = unstructured.SetNestedField(object.Object, true, "spec", "paused")
	if _, err := service.Preview(t.Context(), request); err == nil {
		t.Fatal("paused restart accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.Deployment(ctx, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not propagated: %v", err)
	}
}

func TestProbeSwitchDropsOnlyOldHandler(t *testing.T) {
	container := map[string]interface{}{"livenessProbe": map[string]interface{}{"httpGet": map[string]interface{}{"port": int64(80)}, "terminationGracePeriodSeconds": int64(15)}}
	probe := Probe{Kind: "exec", Command: []string{"/bin/check", "arg with spaces"}, Period: 5, Timeout: 1, SuccessThreshold: 1, FailureThreshold: 3}
	if err := editProbe(container, "livenessProbe", Probe{Kind: "http"}, probe); err != nil {
		t.Fatal(err)
	}
	result := container["livenessProbe"].(map[string]interface{})
	if result["httpGet"] != nil || result["terminationGracePeriodSeconds"] != int64(15) {
		t.Fatal("probe handler replacement lost unrelated fields")
	}
}
