package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/client-go/rest"
)

func testClientFor(t *testing.T, handler http.HandlerFunc) resource.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := (Factory{}).New(t.Context(), &rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Error(err)
	}
}

func TestVersionAndDynamicListUseRequestedScopePaginationAndLabels(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			writeJSON(t, w, `{"gitVersion":"v1.36.2"}`)
			return
		}
		if r.URL.Path != "/apis/demo.example.com/v1/namespaces/team/widgets" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("continue") != "page-2" || r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("labelSelector") != "app=demo" {
			t.Errorf("incorrect list options: %s", r.URL.RawQuery)
		}
		writeJSON(t, w, `{"apiVersion":"demo.example.com/v1","kind":"WidgetList","metadata":{"continue":"page-3","resourceVersion":"42"},"items":[{"apiVersion":"demo.example.com/v1","kind":"Widget","metadata":{"name":"one","namespace":"team","uid":"uid-1"},"spec":{"secretValue":"do-not-expose-in-list"}}]}`)
	})
	version, err := client.Version(t.Context())
	if err != nil || version != "v1.36.2" {
		t.Fatalf("version = %q, err = %v", version, err)
	}
	page, err := client.List(t.Context(), resource.Query{Group: "demo.example.com", Version: "v1", Resource: "widgets", Namespace: "team", Continue: "page-2", LabelSelector: "app=demo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Continue != "page-3" || page.ResourceVersion != "42" {
		t.Fatalf("unexpected page: %+v", page)
	}
	data, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "do-not-expose") {
		t.Fatal("list response leaked full resource")
	}
}

func TestSecretYAMLRedactsValuesAndLastAppliedAnnotation(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"credentials","annotations":{"kubectl.kubernetes.io/last-applied-configuration":"sensitive-annotation"},"managedFields":[{"manager":"test"}]},"data":{"password":"c2VjcmV0"},"stringData":{"token":"sensitive-token"}}`)
	})
	yaml, err := client.YAML(t.Context(), resource.Ref{Version: "v1", Resource: "secrets", Name: "credentials"})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"sensitive-annotation", "c2VjcmV0", "sensitive-token", "managedFields"} {
		if strings.Contains(yaml, secret) {
			t.Fatalf("YAML exposed %q", secret)
		}
	}
	if !strings.Contains(yaml, "password: <已隐藏>") {
		t.Fatalf("missing redacted key: %s", yaml)
	}
}

func TestYAMLKeepsLargeIntegersAsText(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"apiVersion":"demo.example.com/v1","kind":"Widget","metadata":{"name":"one"},"spec":{"sequence":9007199254740993}}`)
	})
	value, err := client.YAML(t.Context(), resource.Ref{Group: "demo.example.com", Version: "v1", Resource: "widgets", Name: "one"})
	if err != nil || !strings.Contains(value, "9007199254740993") {
		t.Fatalf("integer lost precision: %s, %v", value, err)
	}
}

func TestDiscoveryKeepsHealthyGroupsWhenAnExtensionFails(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis":
			writeJSON(t, w, `{"kind":"APIGroupList","apiVersion":"v1","groups":[{"name":"demo.example.com","preferredVersion":{"groupVersion":"demo.example.com/v1","version":"v1"}},{"name":"broken.example.com","preferredVersion":{"groupVersion":"broken.example.com/v1","version":"v1"}}]}`)
		case "/api/v1":
			writeJSON(t, w, `{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"v1","resources":[{"name":"pods","kind":"Pod","namespaced":true,"verbs":["get","list"]}]}`)
		case "/apis/demo.example.com/v1":
			writeJSON(t, w, `{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"demo.example.com/v1","resources":[{"name":"widgets","kind":"Widget","namespaced":true,"verbs":["get","list"]},{"name":"widgets/status","kind":"Widget","verbs":["get","update"]}]}`)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	catalog, err := client.Discover(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Resources) != 2 || len(catalog.Warnings) != 1 {
		t.Fatalf("unexpected discovery: %+v", catalog)
	}
}

func TestCanceledRequestsAndSensitiveErrors(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("canceled request reached server") })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.Version(ctx); err == nil {
		t.Fatal("canceled request succeeded")
	}
	err := requestError(fmt.Errorf("exec failed: token=credential-do-not-expose"))
	if strings.Contains(err.Error(), "credential-do-not-expose") {
		t.Fatal("raw error leaked credential")
	}
}
