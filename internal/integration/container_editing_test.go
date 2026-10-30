//go:build integration

package integration

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workload"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func TestAPIServerContainerEditing(t *testing.T) {
	config := controlPlane(t)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	connection, err := (kube.Factory{}).New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	writer := manifest.NewService(resolver{connection.(manifest.Client)})
	defer writer.ServiceShutdown()
	service := workload.NewService(resolver{connection.(manifest.Client)}, writer)
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"deployments", "statefulsets", "daemonsets", "replicasets"} {
		t.Run(kind, func(t *testing.T) {
			target := resource.Ref{ClusterID: "test-only", Group: "apps", Version: "v1", Resource: kind, Namespace: "default", Name: "container-" + kind}
			api := client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: kind}).Namespace(target.Namespace)
			controller := workload.ControllerForm{Strategy: "RollingUpdate", MaxUnavailable: "1", MaxSurge: "0", ServiceName: "headless", PodManagementPolicy: "OrderedReady", RetainDeleted: "Retain", RetainScaled: "Retain"}
			preview := func(action string, form workload.Deployment, baseline manifest.Baseline) (manifest.Preview, error) {
				if kind == "deployments" {
					return service.Preview(ctx, workload.Request{Target: target, Action: action, Form: form, Baseline: baseline})
				}
				controller.Workload = form
				return service.PreviewController(ctx, workload.ControllerRequest{Target: target, Action: action, Form: controller, Baseline: baseline})
			}
			read := func() (workload.Deployment, manifest.Baseline) {
				if kind == "deployments" {
					snapshot, err := service.Deployment(ctx, target)
					if err != nil {
						t.Fatal(err)
					}
					return snapshot.Form, snapshot.Baseline
				}
				snapshot, err := service.Controller(ctx, target)
				if err != nil {
					t.Fatal(err)
				}
				controller = snapshot.Form
				return snapshot.Form.Workload, snapshot.Baseline
			}
			form := workload.Deployment{Name: target.Name, Namespace: target.Namespace, Replicas: 2, Containers: []workload.Container{
				{Name: "app", Image: "busybox", Command: []string{"/bin/sh", "-c"}, Args: []string{"echo before"}, Ports: []workload.ContainerPort{{Name: "http", Protocol: "TCP", ContainerPort: 8080}}},
				{Name: "remove-me", Image: "busybox"}, {Name: "keep-me", Image: "busybox"},
			}}
			plan, err := preview("create", form, manifest.Baseline{})
			if err != nil {
				t.Fatal(err)
			}
			commitPreview(t, ctx, writer, plan)
			patchRelease(t, ctx, api, target.Name, `{"spec":{"template":{"spec":{"initContainers":[{"name":"init","image":"busybox","command":["true"]}],"volumes":[{"name":"data","emptyDir":{}}],"securityContext":{"runAsUser":1000},"terminationGracePeriodSeconds":9007199254740993}}}}`)
			before := getRelease(t, ctx, api, target.Name)
			form, baseline := read()
			if !reflect.DeepEqual(form.Containers[0].Command, []string{"/bin/sh", "-c"}) || form.Containers[0].Ports[0].ContainerPort != 8080 {
				t.Fatal("create lost container fields")
			}
			form.Containers[0].Args = []string{"printf '%s' \"hello world\"\nprintf done", ""}
			form.Containers[0].Ports[0].ContainerPort = 9090
			form.Containers[0].Ports[0].HostPort, form.Containers[0].Ports[0].HostIP = 19090, "127.0.0.1"
			form.Containers = []workload.Container{form.Containers[0], form.Containers[2], {Name: "added", Image: "busybox", Command: []string{"sleep"}, Args: []string{"3600"}}}
			plan, err = preview("update", form, baseline)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, getRelease(t, ctx, api, target.Name)) {
				t.Fatal("preview persisted changes")
			}
			commitPreview(t, ctx, writer, plan)
			actual, _ := read()
			if len(actual.Containers) != 3 || actual.Containers[1].Name != "keep-me" || actual.Containers[2].Name != "added" {
				t.Fatal("wrong container removed or added")
			}
			if !reflect.DeepEqual(actual.Containers[0].Args, form.Containers[0].Args) || actual.Containers[0].Ports[0].HostPort != 19090 || actual.Containers[0].Ports[0].ContainerPort != 9090 {
				t.Fatal("arguments or ports did not round-trip")
			}
			after := getRelease(t, ctx, api, target.Name)
			for _, field := range []string{"initContainers", "volumes", "securityContext", "terminationGracePeriodSeconds"} {
				want, _, _ := unstructured.NestedFieldNoCopy(before.Object, "spec", "template", "spec", field)
				got, _, _ := unstructured.NestedFieldNoCopy(after.Object, "spec", "template", "spec", field)
				if !reflect.DeepEqual(want, got) {
					t.Fatalf("lost %s", field)
				}
			}
			form, baseline = read()
			form.Containers = form.Containers[:2]
			form.Containers[0].Command, form.Containers[0].Args, form.Containers[0].Ports = nil, nil, nil
			plan, err = preview("update", form, baseline)
			if err != nil {
				t.Fatal(err)
			}
			patchRelease(t, ctx, api, target.Name, `{"metadata":{"annotations":{"external":"changed"}}}`)
			if _, err := writer.Commit(ctx, plan.Token); err == nil {
				t.Fatal("stale removal overwrote concurrent edit")
			}
			_, baseline = read()
			plan, err = preview("update", form, baseline)
			if err != nil {
				t.Fatal(err)
			}
			commitPreview(t, ctx, writer, plan)
			after, err = api.Get(ctx, target.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			containers, _, _ := unstructured.NestedSlice(after.Object, "spec", "template", "spec", "containers")
			if len(containers) != 2 || after.GetAnnotations()["external"] != "changed" {
				t.Fatal("removal or metadata preservation failed")
			}
			for _, field := range []string{"command", "args", "ports"} {
				if _, exists := containers[0].(map[string]interface{})[field]; exists {
					t.Fatalf("clear retained %s", field)
				}
			}
		})
	}
}
