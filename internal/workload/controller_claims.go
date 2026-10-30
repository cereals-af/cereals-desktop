package workload

import (
	"errors"
	"path"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

func createClaims(object *unstructured.Unstructured, claims []ClaimTemplate) error {
	if len(claims) == 0 {
		return nil
	}
	templates := []interface{}{}
	mounts := []interface{}{}
	names, paths := map[string]bool{}, map[string]bool{}
	for _, claim := range claims {
		if len(validation.IsDNS1123Label(claim.Name)) > 0 || names[claim.Name] {
			return errors.New("PVC 模板名称无效或重复")
		}
		names[claim.Name] = true
		size, err := resource.ParseQuantity(claim.Capacity)
		if err != nil || size.Sign() <= 0 {
			return errors.New("PVC 容量必须是正资源量")
		}
		if claim.VolumeMode != "Filesystem" {
			return errors.New("可视化创建支持 Filesystem PVC，Block 设备请使用 YAML 创建")
		}
		if !path.IsAbs(claim.MountPath) || paths[claim.MountPath] {
			return errors.New("PVC 挂载路径必须是唯一的绝对路径")
		}
		paths[claim.MountPath] = true
		modes := []interface{}{}
		seen := map[string]bool{}
		for _, mode := range claim.AccessModes {
			if seen[mode] || (mode != "ReadWriteOnce" && mode != "ReadWriteMany" && mode != "ReadOnlyMany" && mode != "ReadWriteOncePod") {
				return errors.New("PVC 访问模式无效或重复")
			}
			seen[mode] = true
			modes = append(modes, mode)
		}
		if len(modes) == 0 {
			return errors.New("请选择 PVC 访问模式")
		}
		spec := map[string]interface{}{"accessModes": modes, "volumeMode": claim.VolumeMode, "resources": map[string]interface{}{"requests": map[string]interface{}{"storage": claim.Capacity}}}
		switch claim.ClassMode {
		case "default":
		case "none":
			spec["storageClassName"] = ""
		case "named":
			if claim.ClassName == "" {
				return errors.New("请指定 StorageClass")
			}
			spec["storageClassName"] = claim.ClassName
		default:
			return errors.New("StorageClass 选择模式无效")
		}
		templates = append(templates, map[string]interface{}{"metadata": map[string]interface{}{"name": claim.Name}, "spec": spec})
		mounts = append(mounts, map[string]interface{}{"name": claim.Name, "mountPath": claim.MountPath})
	}
	if err := unstructured.SetNestedSlice(object.Object, templates, "spec", "volumeClaimTemplates"); err != nil {
		return err
	}
	containers, _, err := unstructured.NestedSlice(object.Object, "spec", "template", "spec", "containers")
	if err != nil {
		return err
	}
	for _, raw := range containers {
		container, ok := raw.(map[string]interface{})
		if !ok {
			return errors.New("容器格式无效")
		}
		if err := unstructured.SetNestedSlice(container, mounts, "volumeMounts"); err != nil {
			return err
		}
	}
	return unstructured.SetNestedSlice(object.Object, containers, "spec", "template", "spec", "containers")
}
