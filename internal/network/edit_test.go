package network

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func serviceFixture(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	object := &unstructured.Unstructured{}
	if err := object.UnmarshalJSON([]byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"api","namespace":"default","uid":"uid","resourceVersion":"12","annotations":{"other":"keep"}},"spec":{"type":"LoadBalancer","clusterIP":"10.0.0.2","clusterIPs":["10.0.0.2","fd00::2"],"ipFamilies":["IPv4","IPv6"],"ipFamilyPolicy":"RequireDualStack","externalTrafficPolicy":"Local","healthCheckNodePort":31000,"allocateLoadBalancerNodePorts":false,"loadBalancerClass":"example.com/lb","loadBalancerSourceRanges":["192.0.2.0/24"],"internalTrafficPolicy":"Local","sessionAffinity":"ClientIP","sessionAffinityConfig":{"clientIP":{"timeoutSeconds":123}},"selector":{"app":"api"},"ports":[{"name":"http","protocol":"TCP","port":80,"targetPort":"http","nodePort":30080,"appProtocol":"http","futureField":9007199254740993},{"name":"metrics","protocol":"TCP","port":9090,"targetPort":9090,"nodePort":30090,"futureField":"metrics-value"}]},"status":{"loadBalancer":{"ingress":[{"ip":"192.0.2.2"}]}}}`)); err != nil {
		t.Fatal(err)
	}
	return object
}

func TestServicePreservesAllocatedAndUnknownFields(t *testing.T) {
	object := serviceFixture(t)
	expected := object.DeepCopy()
	snapshot, err := serviceSnapshot(object)
	if err != nil {
		t.Fatal(err)
	}
	form := snapshot.Form
	form.Ports[0].TargetPort = "8080"
	if err := editService(object, form, false); err != nil {
		t.Fatal(err)
	}
	ports, _, _ := unstructured.NestedSlice(expected.Object, "spec", "ports")
	ports[0].(map[string]interface{})["targetPort"] = int64(8080)
	_ = unstructured.SetNestedSlice(expected.Object, ports, "spec", "ports")
	if !reflect.DeepEqual(object.Object, expected.Object) {
		t.Fatal("editing targetPort changed unrelated Service fields")
	}
}

func TestPortsKeepTheirOriginalFieldsAcrossReorderingAndRenaming(t *testing.T) {
	object := serviceFixture(t)
	snapshot, _ := serviceSnapshot(object)
	form := snapshot.Form
	form.Ports[0], form.Ports[1] = form.Ports[1], form.Ports[0]
	form.Ports[0].Name, form.Ports[1].Name = "http", "metrics"
	if err := editService(object, form, false); err != nil {
		t.Fatal(err)
	}
	ports, _, _ := unstructured.NestedSlice(object.Object, "spec", "ports")
	if ports[0].(map[string]interface{})["futureField"] != "metrics-value" || ports[1].(map[string]interface{})["futureField"] != int64(9007199254740993) {
		t.Fatal("port identity shifted when renamed")
	}
}

func TestServiceTypeTransitions(t *testing.T) {
	object := serviceFixture(t)
	snapshot, _ := serviceSnapshot(object)
	form := snapshot.Form
	form.Type = "ClusterIP"
	if err := editService(object, form, false); err != nil {
		t.Fatal(err)
	}
	spec, _, _ := unstructured.NestedMap(object.Object, "spec")
	for _, field := range []string{"externalTrafficPolicy", "healthCheckNodePort", "loadBalancerClass", "allocateLoadBalancerNodePorts", "loadBalancerSourceRanges"} {
		if _, exists := spec[field]; exists {
			t.Fatalf("inapplicable field %s survived", field)
		}
	}
	if spec["clusterIP"] != "10.0.0.2" {
		t.Fatal("allocated IP changed")
	}
	for _, value := range spec["ports"].([]interface{}) {
		if _, exists := value.(map[string]interface{})["nodePort"]; exists {
			t.Fatal("nodePort survived ClusterIP transition")
		}
	}
	form.Type, form.ExternalName = "ExternalName", "api.example.com"
	if err := editService(object, form, false); err != nil {
		t.Fatal(err)
	}
	spec, _, _ = unstructured.NestedMap(object.Object, "spec")
	if !reflect.DeepEqual(spec, map[string]interface{}{"type": "ExternalName", "externalName": "api.example.com"}) {
		t.Fatalf("unexpected ExternalName spec: %#v", spec)
	}
}

func TestServiceRejectsInvalidEdits(t *testing.T) {
	cases := map[string]func(*ServiceForm){
		"switch allocated IP":     func(f *ServiceForm) { f.Type = "ClusterIP"; f.Headless = true },
		"duplicate port identity": func(f *ServiceForm) { f.Ports[1].OriginalKey = f.Ports[0].OriginalKey },
		"missing original port":   func(f *ServiceForm) { f.Ports[0].OriginalKey = "name:deleted" },
		"duplicate names":         func(f *ServiceForm) { f.Ports[1].Name = f.Ports[0].Name },
		"missing port names":      func(f *ServiceForm) { f.Ports[0].Name = "" },
		"bad target":              func(f *ServiceForm) { f.Ports[0].TargetPort = "65536" },
		"bad selector":            func(f *ServiceForm) { f.Selector = []Selector{{Key: "invalid key", Value: "app"}} },
		"duplicate selector":      func(f *ServiceForm) { f.Selector = append(f.Selector, f.Selector[0]) },
		"negative nodePort":       func(f *ServiceForm) { f.Ports[0].NodePort = -1 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			object := serviceFixture(t)
			snapshot, _ := serviceSnapshot(object)
			change(&snapshot.Form)
			if err := editService(object, snapshot.Form, false); err == nil {
				t.Fatal("invalid edit accepted")
			}
		})
	}
}

func TestExternalNamePreservesFullyQualifiedDNSName(t *testing.T) {
	for _, name := range []string{"api.example.com", "api.example.com."} {
		t.Run(name, func(t *testing.T) {
			object := serviceFixture(t)
			snapshot, err := serviceSnapshot(object)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Form.Type, snapshot.Form.ExternalName = "ExternalName", name
			if err := editService(object, snapshot.Form, false); err != nil {
				t.Fatal(err)
			}
			actual, _, _ := unstructured.NestedString(object.Object, "spec", "externalName")
			if actual != name {
				t.Fatalf("ExternalName changed: got %q, want %q", actual, name)
			}
		})
	}
}
