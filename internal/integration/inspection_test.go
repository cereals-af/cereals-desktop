//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/relations"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	resourceschema "github.com/cereals-af/cereals-desktop/internal/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type inspectionResolver struct {
	schemas   resourceschema.Client
	relations relations.Client
}

func (r inspectionResolver) SchemaClient(string) (resourceschema.Client, error) {
	return r.schemas, nil
}
func (r inspectionResolver) RelationClient(string) (relations.Client, error) { return r.relations, nil }

func checkInspection(t *testing.T, ctx context.Context, config *rest.Config, connection resource.Client, client dynamic.Interface) {
	t.Helper()
	resolver := inspectionResolver{schemas: connection.(resourceschema.Client), relations: connection.(relations.Client)}
	schemas := resourceschema.NewService(resolver)
	for _, target := range []resource.Ref{
		{ClusterID: "test-only", Group: "apps", Version: "v1", Resource: "deployments"},
		{ClusterID: "test-only", Version: "v1", Resource: "pods"},
		{ClusterID: "test-only", Group: "creals.test", Version: "v1", Resource: "widgets"},
	} {
		document, err := schemas.Get(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(document.JSON, `"properties"`) || document.Kind == "" {
			t.Fatalf("missing schema: %+v", document)
		}
		if target.Resource == "widgets" && !strings.Contains(document.JSON, `"sequence"`) {
			t.Fatal("CRD schema omitted spec")
		}
		if target.Resource != "widgets" && document.Source != "OpenAPI v3" {
			t.Fatal("built-in schema did not use OpenAPI")
		}
	}

	deployment := inspectionCreate(t, ctx, client, "apps", "deployments", `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"inspect-app","namespace":"default"},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"inspect"}},"template":{"metadata":{"labels":{"app":"inspect"}},"spec":{"containers":[{"name":"app","image":"example.invalid/test:1"}]}}}}`, nil)
	replicaSet := inspectionCreate(t, ctx, client, "apps", "replicasets", `{"apiVersion":"apps/v1","kind":"ReplicaSet","metadata":{"name":"inspect-rs","namespace":"default","annotations":{"deployment.kubernetes.io/revision":"3"}},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"inspect"}},"template":{"metadata":{"labels":{"app":"inspect"}},"spec":{"containers":[{"name":"app","image":"example.invalid/test:1"}]}}}}`, deployment)
	inspectionCreate(t, ctx, client, "", "pods", `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"inspect-pod","namespace":"default","labels":{"app":"inspect"}},"spec":{"containers":[{"name":"app","image":"example.invalid/test:1"}]}}`, replicaSet)
	// Same labels alone must not make this unowned Pod a Deployment descendant.
	inspectionCreate(t, ctx, client, "", "pods", `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"inspect-unowned","namespace":"default","labels":{"app":"inspect"}},"spec":{"containers":[{"name":"app","image":"example.invalid/test:1"}]}}`, nil)
	event := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Event", "metadata": map[string]interface{}{"name": "inspect-event", "namespace": "default"},
		"involvedObject": map[string]interface{}{"apiVersion": "apps/v1", "kind": "Deployment", "name": deployment.GetName(), "namespace": "default", "uid": string(deployment.GetUID())},
		"type":           "Warning", "reason": "InspectTest", "message": "test event", "count": int64(2),
	}}
	if _, err := client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "events"}).Namespace("default").Create(ctx, event, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	ref := resource.Ref{ClusterID: "test-only", Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default", Name: deployment.GetName()}
	report, err := relations.NewService(resolver).Inspect(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Sections[0].Items) != 1 || report.Sections[0].Items[0].Revision != "3" || len(report.Sections[1].Items) != 1 || report.Sections[1].Items[0].Target.Name != "inspect-pod" || len(report.Sections[2].Events) != 1 {
		t.Fatalf("wrong Deployment relations: %+v", report)
	}
	checkServiceInspection(t, ctx, client, resolver)
	checkPartialInspectionPermissions(t, ctx, config, client, ref)
}

func inspectionCreate(t *testing.T, ctx context.Context, client dynamic.Interface, group, resourceName, text string, owner *unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()
	object := &unstructured.Unstructured{}
	if err := object.UnmarshalJSON([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if owner != nil {
		controller := true
		object.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: owner.GetAPIVersion(), Kind: owner.GetKind(), Name: owner.GetName(), UID: owner.GetUID(), Controller: &controller}})
	}
	result, err := client.Resource(schema.GroupVersionResource{Group: group, Version: "v1", Resource: resourceName}).Namespace(object.GetNamespace()).Create(ctx, object, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func checkServiceInspection(t *testing.T, ctx context.Context, client dynamic.Interface, resolver inspectionResolver) {
	t.Helper()
	inspectionCreate(t, ctx, client, "", "services", `{"apiVersion":"v1","kind":"Service","metadata":{"name":"inspect-service","namespace":"default"},"spec":{"selector":{"app":"inspect"},"ports":[{"port":80}]}}`, nil)
	inspectionCreate(t, ctx, client, "discovery.k8s.io", "endpointslices", `{"apiVersion":"discovery.k8s.io/v1","kind":"EndpointSlice","metadata":{"name":"inspect-endpoints","namespace":"default","labels":{"kubernetes.io/service-name":"inspect-service"}},"addressType":"IPv4","ports":[{"port":80,"protocol":"TCP"}],"endpoints":[{"addresses":["10.1.0.2"],"conditions":{"ready":true}}]}`, nil)
	report, err := relations.NewService(resolver).Inspect(ctx, resource.Ref{ClusterID: "test-only", Version: "v1", Resource: "services", Namespace: "default", Name: "inspect-service"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Sections[0].Items) != 1 || !strings.Contains(report.Sections[0].Items[0].Detail, "10.1.0.2") || len(report.Sections[1].Items) != 2 {
		t.Fatalf("wrong Service relations: %+v", report)
	}
}

func checkPartialInspectionPermissions(t *testing.T, ctx context.Context, config *rest.Config, client dynamic.Interface, ref resource.Ref) {
	t.Helper()
	inspectionCreate(t, ctx, client, "rbac.authorization.k8s.io", "roles", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"Role","metadata":{"name":"inspect-reader","namespace":"default"},"rules":[{"apiGroups":["apps"],"resources":["deployments"],"verbs":["get"]}]}`, nil)
	inspectionCreate(t, ctx, client, "rbac.authorization.k8s.io", "rolebindings", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"RoleBinding","metadata":{"name":"inspect-reader","namespace":"default"},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"Role","name":"inspect-reader"},"subjects":[{"kind":"User","name":"inspect-user","apiGroup":"rbac.authorization.k8s.io"}]}`, nil)
	limited := rest.CopyConfig(config)
	limited.Impersonate = rest.ImpersonationConfig{UserName: "inspect-user"}
	connection, err := (kube.Factory{}).New(ctx, limited)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	report, err := relations.NewService(inspectionResolver{relations: connection.(relations.Client)}).Inspect(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if report.Sections[0].Error == "" || report.Sections[2].Error == "" || !report.Sections[1].Truncated {
		t.Fatalf("RBAC errors concealed: %+v", report)
	}
}
