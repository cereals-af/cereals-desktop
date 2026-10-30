package configuration

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"slices"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

type Service struct{ editor *formedit.Editor }

func NewService(resolver manifest.Resolver, writer formedit.Previewer) *Service {
	return &Service{editor: formedit.New(resolver, writer)}
}

func (s *Service) ConfigMap(ctx context.Context, target resource.Ref) (Snapshot, error) {
	if _, err := formedit.Target(target, "configmaps", target.Name, target.Namespace); err != nil {
		return Snapshot{}, err
	}
	object, err := s.editor.Read(ctx, target)
	if err != nil {
		return Snapshot{}, err
	}
	form, err := configMapForm(object)
	return Snapshot{Form: form, Baseline: formedit.Baseline(object)}, err
}

func (s *Service) Preview(ctx context.Context, request Request) (manifest.Preview, error) {
	target, err := formedit.Target(request.Target, "configmaps", request.Form.Name, request.Form.Namespace)
	if err != nil {
		return manifest.Preview{}, err
	}
	return s.editor.Preview(ctx, target, request.Baseline, request.Mode, "ConfigMap", func(object *unstructured.Unstructured) error {
		return editConfigMap(object, request.Form)
	})
}

func configMapForm(object *unstructured.Unstructured) (ConfigMap, error) {
	data, _, err := unstructured.NestedStringMap(object.Object, "data")
	if err != nil {
		return ConfigMap{}, errors.New("ConfigMap data 格式无效，请使用 YAML 编辑")
	}
	binary, _, err := unstructured.NestedStringMap(object.Object, "binaryData")
	if err != nil {
		return ConfigMap{}, errors.New("ConfigMap binaryData 格式无效，请使用 YAML 编辑")
	}
	immutable, _, err := unstructured.NestedBool(object.Object, "immutable")
	if err != nil {
		return ConfigMap{}, errors.New("ConfigMap immutable 格式无效")
	}
	return ConfigMap{Name: object.GetName(), Namespace: object.GetNamespace(), Data: entries(data), BinaryData: entries(binary), Immutable: immutable}, nil
}

func entries(values map[string]string) []Entry {
	result := make([]Entry, 0, len(values))
	for key, value := range values {
		result = append(result, Entry{Key: key, Value: value})
	}
	slices.SortFunc(result, func(a, b Entry) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	return result
}

func editConfigMap(object *unstructured.Unstructured, form ConfigMap) error {
	data, dataSize, err := entryMap(form.Data, false)
	if err != nil {
		return err
	}
	binary, binarySize, err := entryMap(form.BinaryData, true)
	if err != nil {
		return err
	}
	if dataSize+binarySize > 1<<20 {
		return errors.New("ConfigMap 数据总量不能超过 1 MiB")
	}
	for key := range data {
		if _, ok := binary[key]; ok {
			return errors.New("data 与 binaryData 不能使用相同键名")
		}
	}
	previous, err := configMapForm(object)
	if err != nil {
		return err
	}
	if previous.Immutable && (!form.Immutable || !reflect.DeepEqual(entries(data), previous.Data) || !reflect.DeepEqual(entries(binary), previous.BinaryData)) {
		return errors.New("不可变 ConfigMap 不能修改数据或恢复为可变，请创建新资源")
	}
	if err := setData(object, "data", data); err != nil {
		return err
	}
	if err := setData(object, "binaryData", binary); err != nil {
		return err
	}
	if form.Immutable != previous.Immutable {
		object.Object["immutable"] = form.Immutable
	}
	return nil
}

func setData(object *unstructured.Unstructured, field string, values map[string]string) error {
	if len(values) == 0 {
		delete(object.Object, field)
		return nil
	}
	return unstructured.SetNestedStringMap(object.Object, values, field)
}

func entryMap(entries []Entry, binary bool) (map[string]string, int, error) {
	result := make(map[string]string, len(entries))
	size := 0
	for _, entry := range entries {
		if len(validation.IsConfigMapKey(entry.Key)) != 0 {
			return nil, 0, errors.New("配置键名仅支持字母、数字、点、短横线和下划线，且长度不超过 253")
		}
		if _, exists := result[entry.Key]; exists {
			return nil, 0, errors.New("配置键名不能重复")
		}
		length := len(entry.Value)
		if binary {
			data, err := base64.StdEncoding.DecodeString(entry.Value)
			if err != nil {
				return nil, 0, errors.New("binaryData 必须填写有效的标准 Base64")
			}
			length = len(data)
		}
		size += length
		result[entry.Key] = entry.Value
	}
	return result, size, nil
}
