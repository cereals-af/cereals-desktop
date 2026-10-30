package kube

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/schema"
)

func TestSchemaUsesExactGroupVersionKindAndLocalReferences(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/demo.test/v2":
			writeJSON(t, w, `{"apiVersion":"v1","kind":"APIResourceList","groupVersion":"demo.test/v2","resources":[{"name":"widgets","kind":"Widget","namespaced":true,"verbs":["get","list"]}]}`)
		case "/openapi/v3/apis/demo.test/v2":
			writeJSON(t, w, `{"components":{"schemas":{"old":{"x-kubernetes-group-version-kind":[{"group":"demo.test","version":"v1","kind":"Widget"}],"description":"wrong-version"},"Widget":{"type":"object","x-kubernetes-group-version-kind":[{"group":"demo.test","version":"v2","kind":"Widget"}],"properties":{"spec":{"$ref":"#/components/schemas/Spec"}}},"Spec":{"type":"object","properties":{"value":{"type":"integer"}}}}}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	doc, err := client.(schema.Client).ResourceSchema(t.Context(), resource.Ref{Group: "demo.test", Version: "v2", Resource: "widgets"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.APIVersion != "demo.test/v2" || doc.Source != "OpenAPI v3" || !strings.Contains(doc.JSON, "#/definitions/Spec") || strings.Contains(doc.JSON, "wrong-version") {
		t.Fatalf("wrong schema %+v", doc)
	}
}

func TestSchemaFallsBackToExactServedCRDVersion(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/demo.test/v2":
			writeJSON(t, w, `{"apiVersion":"v1","kind":"APIResourceList","groupVersion":"demo.test/v2","resources":[{"name":"widgets","kind":"Widget","namespaced":true,"verbs":["get","list"]}]}`)
		case "/openapi/v3/apis/demo.test/v2":
			w.WriteHeader(404)
		case "/apis/apiextensions.k8s.io/v1/customresourcedefinitions/widgets.demo.test":
			writeJSON(t, w, `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","spec":{"versions":[{"name":"v1","served":true,"schema":{"openAPIV3Schema":{"description":"old-schema"}}},{"name":"v2","served":true,"schema":{"openAPIV3Schema":{"type":"object","properties":{"spec":{"type":"object","properties":{"sequence":{"type":"integer","format":"int64"}}}}}}}]}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	doc, err := client.(schema.Client).ResourceSchema(t.Context(), resource.Ref{Group: "demo.test", Version: "v2", Resource: "widgets"})
	if err != nil || doc.Source != "CRD OpenAPI v3" || !strings.Contains(doc.JSON, "sequence") || !strings.Contains(doc.JSON, "metadata") || strings.Contains(doc.JSON, "old-schema") {
		t.Fatalf("fallback: %+v, %v", doc, err)
	}
}
