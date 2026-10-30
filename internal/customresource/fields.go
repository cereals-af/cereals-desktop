package customresource

import (
	"slices"
	"sort"

	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func fields(root map[string]interface{}) []Field {
	count := 0
	return buildField("", root, false, 0, &count).Fields
}
func buildField(name string, node map[string]interface{}, required bool, depth int, count *int) Field {
	*count++
	f := Field{Name: name, Required: required, Enum: []string{}, Fields: []Field{}}
	f.Type, _, _ = unstructured.NestedString(node, "type")
	f.Description, _, _ = unstructured.NestedString(node, "description")
	f.Nullable, _, _ = unstructured.NestedBool(node, "nullable")
	if depth > 12 || *count > 1000 {
		f.Reason = "结构过深或字段过多，请使用 YAML"
		return f
	}
	for _, keyword := range []string{"$ref", "oneOf", "anyOf", "allOf", "not", "x-kubernetes-int-or-string", "x-kubernetes-embedded-resource"} {
		if value, ok := node[keyword]; ok && value != false {
			f.Reason = "复杂 Schema（" + keyword + "），请使用 YAML"
			return f
		}
	}
	if node["readOnly"] == true {
		f.Reason = "只读字段"
		return f
	}
	if raw, ok := node["enum"].([]interface{}); ok {
		if f.Type == "array" || f.Type == "object" {
			f.Reason = "复合枚举请使用 YAML"
			return f
		}
		for _, item := range raw {
			value, err := formvalue.Encode(item)
			if err != nil || value.Type == "null" {
				f.Reason = "复合枚举请使用 YAML"
				return f
			}
			f.Enum = append(f.Enum, value.Text)
		}
	}
	switch f.Type {
	case "object":
		properties, _, _ := unstructured.NestedMap(node, "properties")
		keys := make([]string, 0, len(properties))
		for key := range properties {
			if depth == 0 && (key == "metadata" || key == "apiVersion" || key == "kind" || key == "status") {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		required, _, _ := unstructured.NestedStringSlice(node, "required")
		for _, key := range keys {
			if *count >= 1000 {
				f.Reason = "字段过多，请使用 YAML"
				f.Fields = nil
				return f
			}
			child, ok := properties[key].(map[string]interface{})
			if ok {
				f.Fields = append(f.Fields, buildField(key, child, slices.Contains(required, key), depth+1, count))
			}
		}
		if len(f.Fields) == 0 {
			f.Reason = "自由对象或映射，请使用 YAML"
		}
	case "array":
		child, ok := node["items"].(map[string]interface{})
		if !ok {
			f.Reason = "数组元素 Schema 不明确，请使用 YAML"
			return f
		}
		item := buildField("item", child, true, depth+1, count)
		f.Items = &item
	case "string", "integer", "number", "boolean":
	default:
		f.Reason = "类型不明确，请使用 YAML"
	}
	// Do not apply defaults here: admission remains authoritative, including numeric enums.
	return f
}
