package workload

import (
	"reflect"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestContainerEditsPreserveIdentityAndUnknownFields(t *testing.T) {
	for _, kind := range []string{"deployments", "statefulsets", "daemonsets", "replicasets"} {
		t.Run(kind, func(t *testing.T) {
			object := fixture(t)
			target := resource.Ref{Resource: kind}
			if kind != "deployments" {
				object, target = controllerFixture(t, kind)
			}
			containers, _, _ := unstructured.NestedSlice(object.Object, "spec", "template", "spec", "containers")
			app := containers[0].(map[string]interface{})
			app["ports"].([]interface{})[0].(map[string]interface{})["futureField"] = int64(9007199254740993)
			containers = append(containers,
				map[string]interface{}{"name": "remove-me", "image": "old", "command": []interface{}{"old"}},
				map[string]interface{}{"name": "keep-me", "image": "keep", "volumeMounts": []interface{}{map[string]interface{}{"name": "data", "mountPath": "/data"}}, "futureField": "keep"})
			if err := unstructured.SetNestedSlice(object.Object, containers, "spec", "template", "spec", "containers"); err != nil {
				t.Fatal(err)
			}
			expected := object.DeepCopy()
			mutate := func(form *Deployment) {
				app := form.Containers[0]
				app.Command = []string{"/bin/sh", "-c"}
				app.Args = []string{"printf '%s' \"a b\"", "", "line one\nline two"}
				app.Ports[0].Name, app.Ports[0].ContainerPort = "web", 9090
				app.Ports[0].HostPort, app.Ports[0].HostIP = 19090, "127.0.0.1"
				form.Containers = []Container{form.Containers[2], app, {Name: "added", Image: "busybox", Ports: []ContainerPort{{Protocol: "UDP", ContainerPort: 5353}}}}
			}
			if kind == "deployments" {
				snapshot, err := snapshot(object)
				if err != nil {
					t.Fatal(err)
				}
				mutate(&snapshot.Form)
				if err := updateDeployment(object, snapshot.Form); err != nil {
					t.Fatal(err)
				}
			} else {
				snapshot, err := controllerSnapshot(object, target)
				if err != nil {
					t.Fatal(err)
				}
				mutate(&snapshot.Form.Workload)
				if err := updateController(object, ControllerRequest{Target: target, Form: snapshot.Form}); err != nil {
					t.Fatal(err)
				}
			}
			app["command"] = []interface{}{"/bin/sh", "-c"}
			app["args"] = []interface{}{"printf '%s' \"a b\"", "", "line one\nline two"}
			port := app["ports"].([]interface{})[0].(map[string]interface{})
			port["name"], port["containerPort"], port["hostPort"], port["hostIP"] = "web", int64(9090), int64(19090), "127.0.0.1"
			added := map[string]interface{}{"name": "added", "image": "busybox", "ports": []interface{}{map[string]interface{}{"protocol": "UDP", "containerPort": int64(5353)}}}
			if err := unstructured.SetNestedSlice(expected.Object, []interface{}{containers[2], app, added}, "spec", "template", "spec", "containers"); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(expected.Object, object.Object) {
				t.Fatalf("container edits changed unrelated fields\nwant: %#v\ngot: %#v", expected.Object, object.Object)
			}
		})
	}
}

func TestContainerProcessAndPortsCanBeCleared(t *testing.T) {
	object := fixture(t)
	containers, _, _ := unstructured.NestedSlice(object.Object, "spec", "template", "spec", "containers")
	raw := containers[0].(map[string]interface{})
	raw["command"], raw["args"] = []interface{}{"/bin/sh", "-c"}, []interface{}{"echo hello world", ""}
	_ = unstructured.SetNestedSlice(object.Object, containers, "spec", "template", "spec", "containers")
	snapshot, err := snapshot(object)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot.Form.Containers[0].Args, []string{"echo hello world", ""}) {
		t.Fatal("snapshot lost arguments")
	}
	before := object.DeepCopy()
	if err := updateDeployment(object, snapshot.Form); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, object) {
		t.Fatal("unchanged commands or ports were normalized")
	}
	snapshot.Form.Containers[0].Command, snapshot.Form.Containers[0].Args, snapshot.Form.Containers[0].Ports = nil, nil, nil
	if err := updateDeployment(object, snapshot.Form); err != nil {
		t.Fatal(err)
	}
	containers, _, _ = unstructured.NestedSlice(object.Object, "spec", "template", "spec", "containers")
	for _, field := range []string{"command", "args", "ports"} {
		if _, exists := containers[0].(map[string]interface{})[field]; exists {
			t.Fatalf("%s was not cleared", field)
		}
	}
}

func TestContainerEditsRejectInvalidFields(t *testing.T) {
	cases := map[string]func(*Deployment){
		"duplicate names":    func(f *Deployment) { f.Containers = append(f.Containers, f.Containers[0]) },
		"init name conflict": func(f *Deployment) { f.Containers = append(f.Containers, Container{Name: "init", Image: "busybox"}) },
		"invalid name": func(f *Deployment) {
			f.Containers = append(f.Containers, Container{Name: "Bad_Name", Image: "busybox"})
		},
		"missing identity":    func(f *Deployment) { f.Containers[0].OriginalName = "" },
		"stale identity":      func(f *Deployment) { f.Containers[0].OriginalName = "missing" },
		"empty executable":    func(f *Deployment) { f.Containers[0].Command = []string{""} },
		"NUL argument":        func(f *Deployment) { f.Containers[0].Args = []string{"bad\x00arg"} },
		"invalid port":        func(f *Deployment) { f.Containers[0].Ports[0].ContainerPort = 65536 },
		"negative host port":  func(f *Deployment) { f.Containers[0].Ports[0].HostPort = -1 },
		"invalid host IP":     func(f *Deployment) { f.Containers[0].Ports[0].HostIP = "example.com" },
		"invalid protocol":    func(f *Deployment) { f.Containers[0].Ports[0].Protocol = "HTTP" },
		"invalid port name":   func(f *Deployment) { f.Containers[0].Ports[0].Name = "123" },
		"stale port identity": func(f *Deployment) { f.Containers[0].Ports[0].OriginalKey = "index:99" },
		"duplicate port identity": func(f *Deployment) {
			port := f.Containers[0].Ports[0]
			port.Name, port.ContainerPort = "second", 9090
			f.Containers[0].Ports = append(f.Containers[0].Ports, port)
		},
		"duplicate port address": func(f *Deployment) {
			f.Containers[0].Ports = append(f.Containers[0].Ports, ContainerPort{ContainerPort: 8080, Protocol: "TCP"})
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			object := fixture(t)
			snapshot, err := snapshot(object)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&snapshot.Form)
			if err := updateDeployment(object, snapshot.Form); err == nil {
				t.Fatal("invalid container edit accepted")
			}
		})
	}
}

func TestContainerPortsKeepDistinctProtocolsAndUnchangedRows(t *testing.T) {
	object := fixture(t)
	snapshot, err := snapshot(object)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Form.Containers[0].Ports = append(snapshot.Form.Containers[0].Ports, ContainerPort{Protocol: "UDP", ContainerPort: 8080, HostIP: "::1", HostPort: 18080})
	if err := updateDeployment(object, snapshot.Form); err != nil {
		t.Fatal(err)
	}
	containers, _, _ := unstructured.NestedSlice(object.Object, "spec", "template", "spec", "containers")
	ports := containers[0].(map[string]interface{})["ports"].([]interface{})
	if _, exists := ports[0].(map[string]interface{})["protocol"]; exists {
		t.Fatal("untouched port was normalized")
	}
	if ports[1].(map[string]interface{})["protocol"] != "UDP" {
		t.Fatal("different protocol was not preserved")
	}
}

func TestContainerPortIdentitySurvivesBaselineReordering(t *testing.T) {
	object := fixture(t)
	containers, _, _ := unstructured.NestedSlice(object.Object, "spec", "template", "spec", "containers")
	raw := containers[0].(map[string]interface{})
	raw["ports"] = []interface{}{
		map[string]interface{}{"name": "http", "containerPort": int64(8080), "futureField": "http-only"},
		map[string]interface{}{"name": "metrics", "containerPort": int64(9090), "futureField": "metrics-only"},
	}
	_ = unstructured.SetNestedSlice(object.Object, containers, "spec", "template", "spec", "containers")
	draft, err := snapshot(object)
	if err != nil {
		t.Fatal(err)
	}
	draft.Form.Containers[0].Ports[0].ContainerPort = 8081
	// Simulate a concurrently reordered list followed by an explicit baseline reload.
	ports := raw["ports"].([]interface{})
	raw["ports"] = []interface{}{ports[1], ports[0]}
	_ = unstructured.SetNestedSlice(object.Object, containers, "spec", "template", "spec", "containers")
	if err := updateDeployment(object, draft.Form); err != nil {
		t.Fatal(err)
	}
	containers, _, _ = unstructured.NestedSlice(object.Object, "spec", "template", "spec", "containers")
	ports = containers[0].(map[string]interface{})["ports"].([]interface{})
	if ports[0].(map[string]interface{})["futureField"] != "http-only" || ports[1].(map[string]interface{})["futureField"] != "metrics-only" {
		t.Fatal("port fields followed row positions instead of stable identities")
	}
	if ports[0].(map[string]interface{})["containerPort"] != int64(8081) {
		t.Fatal("draft port change was lost")
	}
}
