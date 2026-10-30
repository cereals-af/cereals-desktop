package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEditorSchemaResolvesCyclesAndKeepsPropertyNames(t *testing.T) {
	root := map[string]interface{}{"$ref": "#/components/schemas/Root"}
	definitions := map[string]interface{}{
		"Root": map[string]interface{}{"type": "object", "properties": map[string]interface{}{
			"id":       map[string]interface{}{"type": "string"},
			"default":  map[string]interface{}{"type": "integer", "default": json.Number("9007199254740993")},
			"nullable": map[string]interface{}{"type": "string", "nullable": true},
			"child":    map[string]interface{}{"$ref": "#/components/schemas/Root"},
			"port":     map[string]interface{}{"x-kubernetes-int-or-string": true},
		}},
		"Unused": map[string]interface{}{"description": "do-not-include"},
	}
	result, err := EditorSchema(root, definitions)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"id":{"type":"string"}`, `"default":{"type":"integer"}`, `"type":"null"`, `#/definitions/Root`, `"anyOf"`} {
		if !strings.Contains(result, expected) {
			t.Fatalf("missing %s in %s", expected, result)
		}
	}
	for _, unwanted := range []string{"do-not-include", "9007199254740993", "x-kubernetes-"} {
		if strings.Contains(result, unwanted) {
			t.Fatalf("unexpected %s", unwanted)
		}
	}
}

func TestEditorSchemaRejectsRemoteAndMissingReferences(t *testing.T) {
	for _, ref := range []string{"https://example.com/schema", "#/components/schemas/Missing"} {
		if _, err := EditorSchema(map[string]interface{}{"$ref": ref}, nil); err == nil {
			t.Fatalf("accepted %s", ref)
		}
	}
}

func TestEditorSchemaPreservesUnknownMapsAndDoesNotMutateSource(t *testing.T) {
	root := map[string]interface{}{"type": "object", "x-kubernetes-preserve-unknown-fields": true, "$id": "https://example.com/", "additionalProperties": false}
	text, err := EditorSchema(root, nil)
	if err != nil || !strings.Contains(text, `"additionalProperties":true`) || strings.Contains(text, "example.com") {
		t.Fatalf("schema: %s, %v", text, err)
	}
	if root["additionalProperties"] != false || root["$id"] == nil {
		t.Fatal("mutated shared source")
	}
}
