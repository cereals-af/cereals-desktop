package schema

import (
	"encoding/json"
	"errors"
	"strings"
)

// EditorSchema converts the reachable OpenAPI schema graph to local JSON Schema
// references. No external schema URLs or defaults are passed to the editor.
// Kubernetes CEL, admission and server defaults remain the API server's concern.
func EditorSchema(root map[string]interface{}, definitions map[string]interface{}) (string, error) {
	c := converter{source: definitions, definitions: map[string]interface{}{}, pending: map[string]bool{}}
	value, err := c.convert(root, 0)
	if err != nil {
		return "", err
	}
	result, ok := value.(map[string]interface{})
	if !ok {
		return "", errors.New("Schema 根节点无效")
	}
	result["definitions"] = c.definitions
	data, err := json.Marshal(result)
	if err != nil || len(data) > 8<<20 {
		return "", errors.New("Schema 无效或超过 8 MiB 限制")
	}
	return string(data), nil
}

type converter struct {
	source      map[string]interface{}
	definitions map[string]interface{}
	pending     map[string]bool
	nodes       int
}

func (c *converter) convert(value interface{}, depth int) (interface{}, error) {
	c.nodes++
	if depth > 64 || c.nodes > 100000 {
		return nil, errors.New("Schema 结构超过编辑器处理限制")
	}
	switch value := value.(type) {
	case map[string]interface{}:
		return c.object(value, depth)
	case []interface{}:
		result := make([]interface{}, 0, len(value))
		for _, item := range value {
			converted, err := c.convert(item, depth+1)
			if err != nil {
				return nil, err
			}
			result = append(result, converted)
		}
		return result, nil
	default:
		return value, nil
	}
}

func (c *converter) object(source map[string]interface{}, depth int) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(source))
	for key, value := range source {
		// Avoid remote resolution and inserting default numbers through JS floats.
		if key == "$id" || key == "id" || key == "$schema" || key == "default" || key == "example" || key == "examples" || strings.HasPrefix(key, "x-kubernetes-") || key == "nullable" {
			continue
		}
		if key == "$ref" {
			ref, ok := value.(string)
			if !ok {
				return nil, errors.New("Schema 引用格式无效")
			}
			converted, err := c.reference(ref, depth)
			if err != nil {
				return nil, err
			}
			result[key] = converted
			continue
		}
		switch key {
		case "properties", "patternProperties":
			properties, ok := value.(map[string]interface{})
			if !ok {
				return nil, errors.New("Schema properties 格式无效")
			}
			converted := make(map[string]interface{}, len(properties))
			for name, property := range properties {
				child, err := c.convert(property, depth+1)
				if err != nil {
					return nil, err
				}
				converted[name] = child
			}
			result[key] = converted
		case "items", "additionalProperties", "allOf", "anyOf", "oneOf", "not":
			converted, err := c.convert(value, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = converted
		default:
			result[key] = value
		}
	}
	if source["x-kubernetes-int-or-string"] == true {
		delete(result, "type")
		result["anyOf"] = []interface{}{map[string]interface{}{"type": "integer"}, map[string]interface{}{"type": "string"}}
	}
	if source["x-kubernetes-preserve-unknown-fields"] == true {
		result["additionalProperties"] = true
	}
	if source["nullable"] == true {
		return map[string]interface{}{"anyOf": []interface{}{result, map[string]interface{}{"type": "null"}}}, nil
	}
	return result, nil
}

func (c *converter) reference(ref string, depth int) (string, error) {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return "", errors.New("Schema 包含不支持的引用；不会下载外部 Schema")
	}
	encoded := strings.TrimPrefix(ref, prefix)
	name := strings.NewReplacer("~1", "/", "~0", "~").Replace(encoded)
	definition, ok := c.source[name]
	if !ok {
		return "", errors.New("Schema 引用缺失，请刷新后重试")
	}
	if !c.pending[name] {
		c.pending[name] = true
		value, err := c.convert(definition, depth+1)
		if err != nil {
			return "", err
		}
		c.definitions[name] = value
	}
	return "#/definitions/" + encoded, nil
}
