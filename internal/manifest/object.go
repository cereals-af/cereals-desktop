package manifest

import (
	"encoding/json"
	"errors"
	"reflect"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func editableObject(object *unstructured.Unstructured) *unstructured.Unstructured {
	copy := object.DeepCopy()
	delete(copy.Object, "status")
	for _, field := range []string{"uid", "resourceVersion", "generation", "creationTimestamp", "deletionTimestamp", "deletionGracePeriodSeconds", "managedFields", "selfLink"} {
		unstructured.RemoveNestedField(copy.Object, "metadata", field)
	}
	return copy
}

func render(object *unstructured.Unstructured) (string, error) {
	data, err := json.Marshal(object.Object)
	if err != nil {
		return "", errors.New("资源不能转换为 JSON")
	}
	text, err := yaml.JSONToYAML(data)
	if err != nil {
		return "", errors.New("资源不能转换为 YAML")
	}
	return string(text), nil
}

func baseline(object *unstructured.Unstructured) Baseline {
	return Baseline{UID: string(object.GetUID()), ResourceVersion: object.GetResourceVersion()}
}

func checkBaseline(object *unstructured.Unstructured, expected Baseline) error {
	if expected.UID == "" || expected.ResourceVersion == "" || baseline(object) != expected {
		return errors.New("资源已变化或已被重新创建，请保留草稿，重新读取后比较修改")
	}
	return nil
}

func isSecret(ref resource.Ref) bool { return ref.Group == "" && ref.Resource == "secrets" }

func redact(object *unstructured.Unstructured) {
	for _, field := range []string{"data", "stringData"} {
		if values, ok := object.Object[field].(map[string]interface{}); ok {
			for key := range values {
				values[key] = "<已隐藏>"
			}
		}
	}
	object.SetAnnotations(nil)
}

// mergeDiff implements JSON Merge Patch for a complete YAML edit. Unknown fields
// survive because the draft starts from the complete baseline, not a typed struct.
func mergeDiff(before, after map[string]interface{}) map[string]interface{} {
	patch := make(map[string]interface{})
	for key := range before {
		if _, exists := after[key]; !exists {
			patch[key] = nil
		}
	}
	for key, value := range after {
		previous, exists := before[key]
		if exists && reflect.DeepEqual(previous, value) {
			continue
		}
		oldMap, oldOK := previous.(map[string]interface{})
		newMap, newOK := value.(map[string]interface{})
		if oldOK && newOK {
			patch[key] = mergeDiff(oldMap, newMap)
		} else {
			patch[key] = value
		}
	}
	return patch
}
