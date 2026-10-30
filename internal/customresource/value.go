package customresource

import (
	"encoding/json"
	"errors"

	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const maxBytes = 2 << 20

func draftObject(value *Value, text string) (*unstructured.Unstructured, error) {
	if (value == nil) == (text == "") {
		return nil, errors.New("请选择表单或 YAML 草稿")
	}
	if value == nil {
		object, err := manifest.ParseDraft(text)
		if err != nil {
			return nil, err
		}
		if _, err := formvalue.Encode(object.Object); err != nil {
			return nil, err
		}
		return object, nil
	}
	decoded, err := formvalue.Decode(*value)
	if err != nil {
		return nil, err
	}
	object, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, errors.New("资源根节点必须是对象")
	}
	data, err := json.Marshal(object)
	if err != nil || len(data) > maxBytes {
		return nil, errors.New("资源超过 2 MiB 或不能序列化")
	}
	return &unstructured.Unstructured{Object: object}, nil
}
func makeDraft(object *unstructured.Unstructured) (Draft, error) {
	text, err := manifest.RenderDraft(object)
	if err != nil {
		return Draft{}, err
	}
	clean, err := manifest.ParseDraft(text)
	if err != nil {
		return Draft{}, err
	}
	value, err := formvalue.Encode(clean.Object)
	return Draft{Value: value, YAML: text}, err
}

// Convert changes editor representation without persisting or writing to a cluster.
func (s *Service) Convert(value *Value, text string) (Draft, error) {
	object, err := draftObject(value, text)
	if err != nil {
		return Draft{}, err
	}
	return makeDraft(object)
}
