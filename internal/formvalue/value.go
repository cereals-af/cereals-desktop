// Package formvalue transports Kubernetes values without losing integer precision.
package formvalue

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
)

// Value keeps numbers as text and retains fields outside a visual form.
type Value struct {
	Type       string           `json:"type"`
	Text       string           `json:"text"`
	Properties map[string]Value `json:"properties"`
	Items      []Value          `json:"items"`
}

// Encode creates a bounded, lossless editing tree.
func Encode(value interface{}) (Value, error) { return encodeValue(value, 0, &budget{}) }

// Decode validates the tree and restores Kubernetes JSON scalar types.
func Decode(value Value) (interface{}, error) { return decodeValue(value, 0, &budget{}) }

const maxNodes = 20000
const maxBytes = 2 << 20

type budget struct{ nodes, bytes int }

func (b *budget) visit(depth, size int) error {
	b.nodes++
	b.bytes += size
	if depth > 64 || b.nodes > maxNodes || b.bytes > maxBytes {
		return errors.New("表单超过 64 层、20000 节点或 2 MiB 限制，请缩小资源内容")
	}
	return nil
}

func encodeValue(value interface{}, depth int, b *budget) (Value, error) {
	result := Value{Properties: map[string]Value{}, Items: []Value{}}
	if err := b.visit(depth, 0); err != nil {
		return result, err
	}
	switch v := value.(type) {
	case nil:
		result.Type = "null"
	case string:
		result.Type = "string"
		result.Text = v
	case bool:
		result.Type = "boolean"
		result.Text = strconv.FormatBool(v)
	case int64:
		result.Type = "integer"
		result.Text = strconv.FormatInt(v, 10)
	case float64:
		result.Type = "number"
		result.Text = strconv.FormatFloat(v, 'g', -1, 64)
	case map[string]interface{}:
		result.Type = "object"
		for key, item := range v {
			child, err := encodeValue(item, depth+1, b)
			if err != nil {
				return result, err
			}
			b.bytes += len(key)
			result.Properties[key] = child
		}
	case []interface{}:
		result.Type = "array"
		for _, item := range v {
			child, err := encodeValue(item, depth+1, b)
			if err != nil {
				return result, err
			}
			result.Items = append(result.Items, child)
		}
	default:
		return result, errors.New("资源包含不支持的 JSON 类型")
	}
	b.bytes += len(result.Text)
	if b.bytes > maxBytes {
		return result, errors.New("资源超过 2 MiB")
	}
	return result, nil
}

func decodeValue(value Value, depth int, b *budget) (interface{}, error) {
	if err := b.visit(depth, len(value.Text)); err != nil {
		return nil, err
	}
	if value.Type != "object" && len(value.Properties) > 0 || value.Type != "array" && len(value.Items) > 0 {
		return nil, errors.New("表单值类型与内容不一致")
	}
	switch value.Type {
	case "null":
		return nil, nil
	case "string":
		return value.Text, nil
	case "boolean":
		if value.Text == "true" {
			return true, nil
		}
		if value.Text == "false" {
			return false, nil
		}
	case "integer":
		n, err := strconv.ParseInt(value.Text, 10, 64)
		if err == nil {
			return n, nil
		}
	case "number":
		if !strings.ContainsAny(value.Text, ".eE") {
			n, err := strconv.ParseInt(value.Text, 10, 64)
			if err == nil {
				return n, nil
			}
			break
		}
		if json.Valid([]byte(value.Text)) {
			n, err := strconv.ParseFloat(value.Text, 64)
			if err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) {
				return n, nil
			}
		}
	case "object":
		result := make(map[string]interface{}, len(value.Properties))
		for key, item := range value.Properties {
			b.bytes += len(key)
			child, err := decodeValue(item, depth+1, b)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case "array":
		result := make([]interface{}, 0, len(value.Items))
		for _, item := range value.Items {
			child, err := decodeValue(item, depth+1, b)
			if err != nil {
				return nil, err
			}
			result = append(result, child)
		}
		return result, nil
	}
	return nil, errors.New("表单值无效；整数必须在 int64 范围内，数字必须有限")
}
