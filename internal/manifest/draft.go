package manifest

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// ParseDraft shares strict, lossless YAML parsing with schema-driven forms.
func ParseDraft(text string) (*unstructured.Unstructured, error) { return parse(text) }

// RenderDraft keeps integers exact and removes server-managed fields.
func RenderDraft(object *unstructured.Unstructured) (string, error) {
	return render(editableObject(object))
}
