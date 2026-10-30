package resourceform

import (
	"encoding/base64"
	"errors"
	"reflect"
	"sort"
	"unicode"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

func secretEntries(object *unstructured.Unstructured, reveal bool) ([]SecretEntry, error) {
	data, _, err := unstructured.NestedStringMap(object.Object, "data")
	if err != nil {
		return nil, errors.New("Secret 数据格式无效")
	}
	result := make([]SecretEntry, 0, len(data))
	for key, encoded := range data {
		entry := SecretEntry{Key: key, Encoding: "text"}
		if reveal {
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return nil, errors.New("Secret 包含无效 Base64 数据")
			}
			if readableSecret(decoded) {
				entry.Value = string(decoded)
			} else {
				entry.Encoding = "base64"
				entry.Value = encoded
			}
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}

func editSecret(object *unstructured.Unstructured, values map[string]interface{}, request Request) error {
	if !request.Revealed {
		return errors.New("请先显式解码 Secret，再编辑并预检")
	}
	data := map[string]interface{}{}
	total := 0
	for _, entry := range request.Entries {
		if len(validation.IsConfigMapKey(entry.Key)) != 0 {
			return errors.New("Secret 键名无效")
		}
		if _, exists := data[entry.Key]; exists {
			return errors.New("Secret 键名重复")
		}
		raw := []byte(entry.Value)
		switch entry.Encoding {
		case "text":
		case "base64":
			var err error
			raw, err = base64.StdEncoding.DecodeString(entry.Value)
			if err != nil {
				return errors.New("Secret 二进制条目必须使用有效 Base64")
			}
		default:
			return errors.New("请选择明文或 Base64 编码")
		}
		total += len(raw)
		if total > 1<<20 {
			return errors.New("Secret 数据不能超过 1 MiB")
		}
		data[entry.Key] = base64.StdEncoding.EncodeToString(raw)
	}
	immutable, _, _ := unstructured.NestedBool(object.Object, "immutable")
	emptyUnchanged := len(data) == 0 && object.Object["data"] == nil
	if immutable && ((!emptyUnchanged && !reflect.DeepEqual(object.Object["data"], data)) || values["immutable"] != true) {
		return errors.New("不可变 Secret 不能修改数据或取消不可变设置")
	}
	for _, key := range []string{"type", "immutable"} {
		if value, found := values[key]; found {
			object.Object[key] = value
		} else {
			delete(object.Object, key)
		}
	}
	if !emptyUnchanged {
		object.Object["data"] = data
	}
	delete(object.Object, "stringData")
	return nil
}

// Binary values that happen to be valid UTF-8 still need a byte-safe editor.
func readableSecret(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	for _, r := range string(data) {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}
