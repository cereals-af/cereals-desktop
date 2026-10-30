//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/configuration"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/network"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

func checkCoreForms(t *testing.T, ctx context.Context, manifests *manifest.Service, writer manifest.Client, client dynamic.Interface) {
	t.Helper()
	networkService := network.NewService(resolver{client: writer}, manifests)
	configs := configuration.NewService(resolver{client: writer}, manifests)
	t.Run("service-types-and-transitions", func(t *testing.T) { checkServiceForms(t, ctx, manifests, networkService, client) })
	t.Run("configmap-data-and-immutability", func(t *testing.T) { checkConfigForms(t, ctx, manifests, configs, client) })
}

func checkServiceForms(t *testing.T, ctx context.Context, manifests *manifest.Service, service *network.Service, client dynamic.Interface) {
	api := client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "services"}).Namespace("default")
	for _, kind := range []string{"ClusterIP", "NodePort", "LoadBalancer", "ExternalName", "Headless"} {
		t.Run(kind, func(t *testing.T) {
			ref := resource.Ref{ClusterID: "test-only", Version: "v1", Resource: "services", Namespace: "default", Name: "form-" + strings.ToLower(kind)}
			form := network.ServiceForm{Name: ref.Name, Namespace: ref.Namespace, Type: kind, SessionAffinity: "None", ExternalTrafficPolicy: "Local",
				Selector: []network.Selector{{Key: "app", Value: "demo"}}, Ports: []network.Port{{Name: "http", Protocol: "TCP", Port: 80, TargetPort: "http"}}}
			if kind == "Headless" {
				form.Type, form.Headless = "ClusterIP", true
			}
			if kind == "ExternalName" {
				form.ExternalName = "api.example.com."
			}
			preview, err := service.Preview(ctx, network.Request{Target: ref, Mode: "create", Form: form})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := api.Get(ctx, ref.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
				t.Fatalf("service preview persisted: %v", err)
			}
			commitPreview(t, ctx, manifests, preview)
			current, err := service.Snapshot(ctx, ref)
			if err != nil {
				t.Fatal(err)
			}
			if kind != "ExternalName" && len(current.ClusterIPs) == 0 {
				t.Fatal("service allocation not readable")
			}
			if kind == "ExternalName" && current.Form.ExternalName != form.ExternalName {
				t.Fatal("fully qualified ExternalName changed")
			}
			if kind == "NodePort" {
				checkServicePortUpdates(t, ctx, manifests, service, api, ref, current)
			}
			if kind == "LoadBalancer" {
				checkLoadBalancerTransition(t, ctx, manifests, service, api, ref, current)
			}
			if kind == "Headless" && !current.Form.Headless {
				t.Fatal("headless field lost")
			}
		})
	}
}

func checkServicePortUpdates(t *testing.T, ctx context.Context, manifests *manifest.Service, service *network.Service, api dynamic.ResourceInterface, ref resource.Ref, snapshot network.Snapshot) {
	t.Helper()
	allocatedPort := snapshot.Form.Ports[0].NodePort
	if allocatedPort == 0 {
		t.Fatal("NodePort not allocated")
	}
	allocatedIP := snapshot.ClusterIPs[0]
	snapshot.Form.Ports[0].Name = "web"
	snapshot.Form.Ports[0].TargetPort = "8080"
	snapshot.Form.Ports[0].AppProtocol = "kubernetes.io/h2c"
	request := network.Request{Target: ref, Mode: "patch", Baseline: snapshot.Baseline, Form: snapshot.Form}
	preview, err := service.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.Patch(ctx, ref.Name, types.MergePatchType, []byte(`{"metadata":{"annotations":{"external":"keep"}}}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := manifests.Commit(ctx, preview.Token); err == nil {
		t.Fatal("stale Service preview committed")
	}
	fresh, err := service.Snapshot(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	request.Baseline = fresh.Baseline
	preview, err = service.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	commitPreview(t, ctx, manifests, preview)
	fresh, err = service.Snapshot(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Form.Ports[0].NodePort != allocatedPort || fresh.ClusterIPs[0] != allocatedIP || fresh.Form.Ports[0].TargetPort != "8080" {
		t.Fatal("port edit lost allocations")
	}
	// Transition through ExternalName and back exercises allocated IP cleanup and reallocation.
	fresh.Form.Type, fresh.Form.ExternalName = "ExternalName", "external.example.com"
	preview, err = service.Preview(ctx, network.Request{Target: ref, Mode: "patch", Baseline: fresh.Baseline, Form: fresh.Form})
	if err != nil {
		t.Fatal(err)
	}
	commitPreview(t, ctx, manifests, preview)
	fresh, err = service.Snapshot(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.ClusterIPs) != 0 {
		t.Fatal("ExternalName kept allocated IP")
	}
	fresh.Form.Type = "ClusterIP"
	fresh.Form.Ports = []network.Port{{Name: "http", Protocol: "TCP", Port: 80, TargetPort: "8080"}}
	preview, err = service.Preview(ctx, network.Request{Target: ref, Mode: "patch", Baseline: fresh.Baseline, Form: fresh.Form})
	if err != nil {
		t.Fatal(err)
	}
	commitPreview(t, ctx, manifests, preview)
	object, err := api.Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if object.GetAnnotations()["external"] != "keep" {
		t.Fatal("type changes lost metadata")
	}
}

func checkLoadBalancerTransition(t *testing.T, ctx context.Context, manifests *manifest.Service, service *network.Service, api dynamic.ResourceInterface, ref resource.Ref, snapshot network.Snapshot) {
	t.Helper()
	object, err := api.Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	port, _, _ := unstructured.NestedInt64(object.Object, "spec", "healthCheckNodePort")
	if port == 0 {
		t.Fatal("LoadBalancer Local health-check port not allocated")
	}
	snapshot.Form.Type = "ClusterIP"
	preview, err := service.Preview(ctx, network.Request{Target: ref, Mode: "patch", Baseline: snapshot.Baseline, Form: snapshot.Form})
	if err != nil {
		t.Fatal(err)
	}
	commitPreview(t, ctx, manifests, preview)
	object, err = api.Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"healthCheckNodePort", "externalTrafficPolicy", "allocateLoadBalancerNodePorts"} {
		if _, found, _ := unstructured.NestedFieldNoCopy(object.Object, "spec", name); found {
			t.Fatalf("invalid field %s remains", name)
		}
	}
}

func checkConfigForms(t *testing.T, ctx context.Context, manifests *manifest.Service, configs *configuration.Service, client dynamic.Interface) {
	ref := resource.Ref{ClusterID: "test-only", Version: "v1", Resource: "configmaps", Namespace: "default", Name: "form-config"}
	api := client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}).Namespace("default")
	form := configuration.ConfigMap{Name: ref.Name, Namespace: ref.Namespace, Data: []configuration.Entry{{Key: "app.yaml", Value: "name: 中文\ninteger: 9007199254740993\n"}}, BinaryData: []configuration.Entry{{Key: "payload", Value: "AAEC/w=="}}}
	preview, err := configs.Preview(ctx, configuration.Request{Target: ref, Mode: "create", Form: form})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.Get(ctx, ref.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatal("ConfigMap preview wrote data")
	}
	commitPreview(t, ctx, manifests, preview)
	snapshot, err := configs.ConfigMap(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Form.Data[0].Value != form.Data[0].Value || snapshot.Form.BinaryData[0].Value != "AAEC/w==" {
		t.Fatal("ConfigMap content changed encoding")
	}
	snapshot.Form.Data[0].Value += "updated: true\n"
	request := configuration.Request{Target: ref, Mode: "patch", Baseline: snapshot.Baseline, Form: snapshot.Form}
	preview, err = configs.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.Patch(ctx, ref.Name, types.MergePatchType, []byte(`{"metadata":{"annotations":{"external":"keep"}}}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := manifests.Commit(ctx, preview.Token); err == nil {
		t.Fatal("stale ConfigMap preview committed")
	}
	fresh, err := configs.ConfigMap(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	request.Baseline = fresh.Baseline
	request.Form.Immutable = true
	preview, err = configs.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	commitPreview(t, ctx, manifests, preview)
	fresh, err = configs.ConfigMap(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if !fresh.Form.Immutable {
		t.Fatal("immutable was not persisted")
	}
	object, err := api.Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if object.GetAnnotations()["external"] != "keep" {
		t.Fatal("metadata lost")
	}
	fresh.Form.Data[0].Value = "forbidden"
	if _, err := configs.Preview(ctx, configuration.Request{Target: ref, Mode: "patch", Baseline: fresh.Baseline, Form: fresh.Form}); err == nil {
		t.Fatal("immutable edit reached write pipeline")
	}
	// Independently verify the API server also enforces immutable data.
	if _, err := api.Patch(ctx, ref.Name, types.MergePatchType, []byte(`{"data":{"app.yaml":"forbidden"}}`), metav1.PatchOptions{DryRun: []string{metav1.DryRunAll}}); err == nil {
		t.Fatal("API server allowed immutable mutation")
	}
}

func commitPreview(t *testing.T, ctx context.Context, manifests *manifest.Service, preview manifest.Preview) {
	t.Helper()
	if _, err := manifests.Commit(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
}
