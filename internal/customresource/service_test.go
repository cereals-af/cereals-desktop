package customresource

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func object(t *testing.T, text string) *unstructured.Unstructured {
	t.Helper()
	result, err := manifest.ParseDraft(text)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestDraftRoundTripRetainsPrecisionUnknownsAndNull(t *testing.T) {
	input := object(t, `{"apiVersion":"demo.test/v1","kind":"Widget","metadata":{"name":"one","namespace":"team"},"spec":{"integer":9007199254740993,"nested":{"unexposed":{"huge":9223372036854775807}},"items":[{"x":-9223372036854775808,"unknown":true},null],"nullable":null}}`)
	draft, err := makeDraft(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := draftObject(&draft.Value, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, decoded) || !strings.Contains(draft.YAML, "9007199254740993") {
		t.Fatal("lossy round trip")
	}
	for _, value := range []Value{{Type: "integer", Text: "9223372036854775808"}, {Type: "number", Text: "9223372036854775808"}, {Type: "number", Text: "NaN"}, {Type: "boolean", Text: "1"}, {Type: "string", Properties: map[string]Value{"x": {Type: "null"}}}} {
		if _, err := formvalue.Decode(value); err == nil {
			t.Fatalf("invalid value accepted: %+v", value)
		}
	}
	for _, text := range []string{"spec: {x: 1, x: 2}", "spec: &x [1]\nother: *x", "a: 1\n---\na: 2"} {
		if _, err := draftObject(nil, text); err == nil {
			t.Fatal("ambiguous YAML accepted")
		}
	}
	value := Value{Type: "null"}
	for i := 0; i < 66; i++ {
		value = Value{Type: "array", Items: []Value{value}}
	}
	if _, err := formvalue.Decode(value); err == nil {
		t.Fatal("unbounded tree")
	}
}
func TestYAMLDraftEnforcesNodeLimitBeforePreview(t *testing.T) {
	text := `{"apiVersion":"demo.test/v1","kind":"Widget","metadata":{"name":"one"},"spec":{"items":[` + strings.Repeat("null,", 20000) + `null]}}`
	if _, err := draftObject(nil, text); err == nil || !strings.Contains(err.Error(), "20000") {
		t.Fatalf("expected bounded YAML draft, got %v", err)
	}
}

func TestSchemaProjectionDoesNotNormalizeEnumsOrExposeComplexFields(t *testing.T) {
	schema := object(t, `{"type":"object","properties":{"metadata":{"type":"object"},"status":{"type":"object"},"spec":{"type":"object","required":["integer"],"properties":{"integer":{"type":"integer","enum":[9007199254740993]},"choice":{"anyOf":[{"type":"string"},{"type":"integer"}]},"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}},"unknown":{"type":"object","x-kubernetes-preserve-unknown-fields":true},"nullable":{"type":"string","nullable":true}}}}}`)
	result := fields(schema.Object)
	if len(result) != 1 || result[0].Name != "spec" {
		t.Fatal("envelope exposed")
	}
	byName := map[string]Field{}
	for _, field := range result[0].Fields {
		byName[field.Name] = field
	}
	if byName["integer"].Enum[0] != "9007199254740993" || !byName["integer"].Required || !byName["nullable"].Nullable || byName["choice"].Reason == "" || byName["unknown"].Reason == "" || byName["items"].Items.Type != "object" {
		t.Fatalf("projection: %+v", byName)
	}
}

type client struct {
	manifest.Client
	definition *unstructured.Unstructured
	instance   *unstructured.Unstructured
}

func (c client) GetObject(_ context.Context, target resource.Ref) (*unstructured.Unstructured, error) {
	if target.Resource == "customresourcedefinitions" {
		return c.definition.DeepCopy(), nil
	}
	return c.instance.DeepCopy(), nil
}

type resolver struct{ client manifest.Client }

func (r resolver) ManifestClient(string) (manifest.Client, error) { return r.client, nil }

type capture struct {
	request manifest.Request
	guards  []manifest.Dependency
}

func (c *capture) PreviewGuarded(_ context.Context, request manifest.Request, guards []manifest.Dependency) (manifest.Preview, error) {
	c.request = request
	c.guards = guards
	return manifest.Preview{Desired: request.YAML}, nil
}
func TestFormGuardsDefinitionAndVersion(t *testing.T) {
	definition := object(t, `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"widgets.demo.test","uid":"crd","resourceVersion":"10"},"spec":{"group":"demo.test","names":{"plural":"widgets","kind":"Widget"},"scope":"Namespaced","versions":[{"name":"v1","served":true,"storage":true,"schema":{"openAPIV3Schema":{"type":"object","properties":{"spec":{"type":"object","properties":{"size":{"type":"integer"}}}}}}},{"name":"v2","served":false,"storage":false}]}}`)
	instance := object(t, `{"apiVersion":"demo.test/v1","kind":"Widget","metadata":{"name":"one","namespace":"team","uid":"obj","resourceVersion":"20"},"spec":{"size":9007199254740993},"status":{"secret":"not-in-form"}}`)
	writer := &capture{}
	s := NewService(resolver{client{definition: definition, instance: instance}}, writer)
	target := resource.Ref{ClusterID: "one", Group: "demo.test", Version: "v1", Resource: "widgets", Namespace: "team", Name: "one"}
	snapshot, err := s.Snapshot(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot.Draft.Value.Properties["status"]; ok {
		t.Fatal("status entered form")
	}
	request := Request{Target: target, Baseline: snapshot.Baseline, Definition: snapshot.Definition, Mode: "update", Value: &snapshot.Draft.Value}
	if _, err := s.Preview(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if writer.request.Mode != "jsonpatch" || len(writer.guards) != 1 || writer.guards[0].Target.Namespace != "" || writer.guards[0].Baseline != snapshot.Definition {
		t.Fatal("missing definition guard")
	}
	request.Target.Version = "v2"
	if _, err := s.Preview(t.Context(), request); err == nil {
		t.Fatal("unserved version accepted")
	}
	request.Target = target
	definition.SetResourceVersion("11")
	if _, err := s.Preview(t.Context(), request); err == nil {
		t.Fatal("changed schema accepted")
	}
	target.Namespace = "../other"
	if _, err := s.Snapshot(t.Context(), target); err == nil {
		t.Fatal("invalid namespace accepted")
	}
}
