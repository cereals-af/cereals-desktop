package manifest

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"go.yaml.in/yaml/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const maxManifestBytes = 2 << 20

// parse uses YAML nodes so duplicate keys, aliases and integer overflow cannot be
// silently normalized. Integer values remain int64 all the way to the API server.
func parse(text string) (*unstructured.Unstructured, error) {
	if len(text) == 0 || len(text) > maxManifestBytes {
		return nil, errors.New("YAML 不能为空，且不能超过 2 MiB")
	}
	decoder := yaml.NewDecoder(strings.NewReader(text))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil || len(document.Content) != 1 {
		return nil, errors.New("YAML 语法无效，请检查缩进、引号和括号")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("每次操作仅接受一个 YAML 文档")
	}
	value, err := nodeValue(document.Content[0], 0)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]interface{})
	if !ok {
		return nil, errors.New("YAML 根节点必须是 Kubernetes 资源对象")
	}
	return &unstructured.Unstructured{Object: object}, nil
}

func nodeValue(node *yaml.Node, depth int) (interface{}, error) {
	if depth > 64 {
		return nil, nodeError(node, "嵌套层级超过限制")
	}
	switch node.Kind {
	case yaml.MappingNode:
		result := make(map[string]interface{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Tag != "!!str" {
				return nil, nodeError(key, "字段名称必须是字符串")
			}
			if _, exists := result[key.Value]; exists {
				return nil, nodeError(key, "存在重复字段")
			}
			value, err := nodeValue(node.Content[i+1], depth+1)
			if err != nil {
				return nil, err
			}
			result[key.Value] = value
		}
		return result, nil
	case yaml.SequenceNode:
		result := make([]interface{}, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := nodeValue(child, depth+1)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	case yaml.ScalarNode:
		return scalarValue(node)
	default:
		return nil, nodeError(node, "请展开 YAML 别名，不支持锚点引用和合并键")
	}
}

func scalarValue(node *yaml.Node) (interface{}, error) {
	if node.Tag == "!!str" || node.Tag == "!!timestamp" {
		return node.Value, nil
	}
	if node.Tag == "!!null" {
		return nil, nil
	}
	if node.Tag != "!!bool" && node.Tag != "!!int" && node.Tag != "!!float" {
		return nil, nodeError(node, "不支持此 YAML 类型")
	}
	var value interface{}
	if err := node.Decode(&value); err != nil {
		return nil, nodeError(node, "标量格式无效")
	}
	switch number := value.(type) {
	case int:
		return int64(number), nil
	case int64, bool:
		return value, nil
	case uint64:
		if number <= math.MaxInt64 {
			return int64(number), nil
		}
	case float64:
		if node.Tag != "!!int" && !math.IsNaN(number) && !math.IsInf(number, 0) {
			return number, nil
		}
	}
	return nil, nodeError(node, "数字超出 Kubernetes JSON 支持范围；整数限 int64")
}

func nodeError(node *yaml.Node, reason string) error {
	return fmt.Errorf("YAML 第 %d 行，第 %d 列：%s", node.Line, node.Column, reason)
}
