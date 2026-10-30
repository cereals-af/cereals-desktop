package workload

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

// updateContainers matches existing containers by identity, so removing a row never
// transfers another container's unexposed fields to its new position in the form.
func updateContainers(object *unstructured.Unstructured, originalForms, desiredForms []Container) error {
	if len(desiredForms) == 0 {
		return errors.New("至少需要一个容器")
	}
	existing, err := containersByName(object, "containers")
	if err != nil {
		return err
	}
	initContainers, err := containersByName(object, "initContainers")
	if err != nil {
		return err
	}
	originals := make(map[string]Container, len(originalForms))
	for _, form := range originalForms {
		originals[form.Name] = form
	}
	result := make([]interface{}, 0, len(desiredForms))
	seen := map[string]bool{}
	for _, desired := range desiredForms {
		if len(validation.IsDNS1123Label(desired.Name)) != 0 || seen[desired.Name] || initContainers[desired.Name] != nil {
			return errors.New("容器名称必须是有效的小写 DNS 标签，且不能与其他容器或初始化容器重复")
		}
		seen[desired.Name] = true
		raw, original, err := originalContainer(existing, originals, desired)
		if err != nil {
			return err
		}
		if err := editContainer(raw, original, desired); err != nil {
			return fmt.Errorf("容器 %s: %w", desired.Name, err)
		}
		result = append(result, raw)
	}
	return unstructured.SetNestedSlice(object.Object, result, "spec", "template", "spec", "containers")
}

func containersByName(object *unstructured.Unstructured, field string) (map[string]map[string]interface{}, error) {
	containers, _, err := unstructured.NestedSlice(object.Object, "spec", "template", "spec", field)
	if err != nil {
		return nil, err
	}
	result := make(map[string]map[string]interface{}, len(containers))
	for _, value := range containers {
		raw, ok := value.(map[string]interface{})
		if !ok {
			return nil, errors.New("容器格式无效，请使用 YAML 编辑")
		}
		name, _ := raw["name"].(string)
		if name == "" || result[name] != nil {
			return nil, errors.New("容器名称缺失或重复，请使用 YAML 编辑")
		}
		result[name] = raw
	}
	return result, nil
}

func originalContainer(existing map[string]map[string]interface{}, originals map[string]Container, desired Container) (map[string]interface{}, Container, error) {
	if desired.OriginalName == "" {
		if existing[desired.Name] != nil {
			return nil, Container{}, errors.New("新容器不能覆盖已有同名容器，请重新读取资源")
		}
		return map[string]interface{}{"name": desired.Name}, Container{}, nil
	}
	original, found := originals[desired.OriginalName]
	if !found || existing[desired.OriginalName] == nil || desired.Name != desired.OriginalName {
		return nil, Container{}, errors.New("原容器已删除或名称被修改，请重新读取资源")
	}
	return existing[desired.OriginalName], original, nil
}
