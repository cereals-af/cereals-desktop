package customresource

import (
	"context"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

func definitionTarget(target resource.Ref) (resource.Ref, error) {
	name := target.Resource + "." + target.Group
	if target.Group == "apiextensions.k8s.io" && target.Resource == "customresourcedefinitions" {
		name = target.Name
	}
	if target.ClusterID == "" || len(validation.IsDNS1123Subdomain(name)) != 0 || target.Group == "" {
		return resource.Ref{}, errors.New("请选择明确的 CRD 或自定义资源类型")
	}
	return resource.Ref{ClusterID: target.ClusterID, Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions", Name: name}, nil
}
func readDefinition(ctx context.Context, client manifest.Client, target resource.Ref) (*unstructured.Unstructured, resource.Ref, error) {
	ref, err := definitionTarget(target)
	if err != nil {
		return nil, ref, err
	}
	object, err := client.GetObject(ctx, ref)
	if err != nil {
		return nil, ref, err
	}
	if object.GetAPIVersion() != "apiextensions.k8s.io/v1" || object.GetKind() != "CustomResourceDefinition" || object.GetUID() == "" || object.GetResourceVersion() == "" {
		return nil, ref, errors.New("CRD 身份无效")
	}
	return object, ref, nil
}
func (s *Service) Definition(ctx context.Context, target resource.Ref) (Definition, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return Definition{}, err
	}
	object, _, err := readDefinition(ctx, client, target)
	if err != nil {
		return Definition{}, err
	}
	return summarizeDefinition(object), nil
}
func summarizeDefinition(object *unstructured.Unstructured) Definition {
	result := Definition{Name: object.GetName(), Baseline: formedit.Baseline(object), Versions: []Version{}, Conditions: []Condition{}}
	result.Group, _, _ = unstructured.NestedString(object.Object, "spec", "group")
	result.Resource, _, _ = unstructured.NestedString(object.Object, "spec", "names", "plural")
	result.Kind, _, _ = unstructured.NestedString(object.Object, "spec", "names", "kind")
	scope, _, _ := unstructured.NestedString(object.Object, "spec", "scope")
	result.Namespaced = scope == "Namespaced"
	result.StoredVersions, _, _ = unstructured.NestedStringSlice(object.Object, "status", "storedVersions")
	versions, _, _ := unstructured.NestedSlice(object.Object, "spec", "versions")
	for _, raw := range versions {
		v, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		item := Version{Columns: []Column{}}
		item.Name, _, _ = unstructured.NestedString(v, "name")
		item.Served, _, _ = unstructured.NestedBool(v, "served")
		item.Storage, _, _ = unstructured.NestedBool(v, "storage")
		item.Deprecated, _, _ = unstructured.NestedBool(v, "deprecated")
		item.Warning, _, _ = unstructured.NestedString(v, "deprecationWarning")
		columns, _, _ := unstructured.NestedSlice(v, "additionalPrinterColumns")
		for _, raw := range columns {
			column, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			item.Columns = append(item.Columns, columnSummary(column))
		}
		result.Versions = append(result.Versions, item)
	}
	conditions, _, _ := unstructured.NestedSlice(object.Object, "status", "conditions")
	for _, raw := range conditions {
		v, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		item := Condition{}
		item.Type, _, _ = unstructured.NestedString(v, "type")
		item.Status, _, _ = unstructured.NestedString(v, "status")
		item.Reason, _, _ = unstructured.NestedString(v, "reason")
		item.Message, _, _ = unstructured.NestedString(v, "message")
		result.Conditions = append(result.Conditions, item)
	}
	return result
}
func columnSummary(data map[string]interface{}) Column {
	c := Column{}
	c.Name, _, _ = unstructured.NestedString(data, "name")
	c.Type, _, _ = unstructured.NestedString(data, "type")
	c.Description, _, _ = unstructured.NestedString(data, "description")
	c.JSONPath, _, _ = unstructured.NestedString(data, "jsonPath")
	c.Priority, _, _ = unstructured.NestedInt64(data, "priority")
	return c
}
func versionSchema(object *unstructured.Unstructured, target resource.Ref) (map[string]interface{}, error) {
	if target.Namespace != "" && len(validation.IsDNS1123Label(target.Namespace)) != 0 {
		return nil, errors.New("命名空间无效")
	}
	definition := summarizeDefinition(object)
	if definition.Group != target.Group || definition.Resource != target.Resource || target.Version == "" {
		return nil, errors.New("CRD 与所选资源类型不一致")
	}
	if !definition.Namespaced && target.Namespace != "" {
		return nil, errors.New("此 CRD 使用集群作用域，请清空命名空间")
	}
	if object.GetDeletionTimestamp() != nil {
		return nil, errors.New("CRD 正在删除")
	}
	versions, _, _ := unstructured.NestedSlice(object.Object, "spec", "versions")
	for _, raw := range versions {
		v, ok := raw.(map[string]interface{})
		if !ok || v["name"] != target.Version {
			continue
		}
		if v["served"] != true {
			return nil, errors.New("所选 CRD 版本未提供服务，请刷新定义后选择 served 版本")
		}
		root, found, err := unstructured.NestedMap(v, "schema", "openAPIV3Schema")
		if err != nil || !found {
			return nil, errors.New("此版本没有结构化 Schema，请使用 YAML 编辑")
		}
		return root, nil
	}
	return nil, errors.New("CRD 不包含所选版本，请刷新定义")
}
