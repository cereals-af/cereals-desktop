package storage

import (
	"context"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

func (s *Service) PreviewCreate(ctx context.Context, request CreateRequest) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	target, err := formedit.Target(request.Target, "persistentvolumeclaims", request.Form.Name, request.Form.Namespace)
	if err != nil {
		return manifest.Preview{}, err
	}
	if err := validateTarget(target, "persistentvolumeclaims"); err != nil {
		return manifest.Preview{}, err
	}
	spec, err := claimSpec(request.Form)
	if err != nil {
		return manifest.Preview{}, err
	}
	dependencies := []manifest.Dependency{}
	if request.Form.ClassMode == "named" {
		ref := classTarget(target, request.Form.ClassName)
		object, err := s.read(ctx, ref, "storageclasses", "StorageClass")
		if err != nil {
			return manifest.Preview{}, err
		}
		if object.GetDeletionTimestamp() != nil {
			return manifest.Preview{}, errors.New("StorageClass 正在删除")
		}
		dependencies = append(dependencies, manifest.Dependency{Target: ref, Baseline: formedit.Baseline(object)})
	}
	object := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "kind": "PersistentVolumeClaim", "metadata": map[string]interface{}{"name": target.Name, "namespace": target.Namespace}, "spec": spec}}
	yaml, err := manifest.RenderDraft(object)
	if err != nil {
		return manifest.Preview{}, err
	}
	return s.writer.PreviewGuarded(ctx, manifest.Request{Target: target, Mode: "create", YAML: yaml}, dependencies)
}

func claimSpec(form ClaimForm) (map[string]interface{}, error) {
	size, err := positiveQuantity(form.Capacity)
	if err != nil {
		return nil, err
	}
	if form.VolumeMode != "Filesystem" && form.VolumeMode != "Block" {
		return nil, errors.New("卷模式仅支持 Filesystem 或 Block")
	}
	if len(form.AccessModes) == 0 {
		return nil, errors.New("至少选择一种访问模式")
	}
	modes, seen := []interface{}{}, map[string]bool{}
	for _, mode := range form.AccessModes {
		if seen[mode] || (mode != "ReadWriteOnce" && mode != "ReadOnlyMany" && mode != "ReadWriteMany" && mode != "ReadWriteOncePod") {
			return nil, errors.New("访问模式无效或重复")
		}
		seen[mode] = true
		modes = append(modes, mode)
	}
	if seen["ReadWriteOncePod"] && len(modes) != 1 {
		return nil, errors.New("ReadWriteOncePod 不能与其他访问模式组合")
	}
	spec := map[string]interface{}{"accessModes": modes, "volumeMode": form.VolumeMode, "resources": map[string]interface{}{"requests": map[string]interface{}{"storage": size.String()}}}
	switch form.ClassMode {
	case "default", "none":
		if form.ClassName != "" {
			return nil, errors.New("默认或无存储类模式不能同时指定 StorageClass 名称")
		}
		if form.ClassMode == "none" {
			spec["storageClassName"] = ""
		}
	case "named":
		if len(validation.IsDNS1123Subdomain(form.ClassName)) != 0 {
			return nil, errors.New("需要有效的 StorageClass 名称")
		}
		spec["storageClassName"] = form.ClassName
	default:
		return nil, errors.New("请选择默认、指定或无 StorageClass")
	}
	return spec, nil
}

func (s *Service) PreviewExpand(ctx context.Context, request ExpandRequest) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	object, err := s.read(ctx, request.Target, "persistentvolumeclaims", "PersistentVolumeClaim")
	if err != nil {
		return manifest.Preview{}, err
	}
	if request.Baseline.UID == "" || request.Baseline.ResourceVersion == "" || formedit.Baseline(object) != request.Baseline {
		return manifest.Preview{}, errors.New("PVC 已变化或被重建，请重新读取基线后预检")
	}
	claim, err := claimSnapshot(object)
	if err != nil {
		return manifest.Preview{}, err
	}
	size, err := expansionSize(claim, request.Capacity)
	if err != nil {
		return manifest.Preview{}, err
	}
	ref := classTarget(request.Target, claim.ClassName)
	classObject, err := s.read(ctx, ref, "storageclasses", "StorageClass")
	if err != nil {
		return manifest.Preview{}, err
	}
	sc, err := classSnapshot(classObject)
	if err != nil {
		return manifest.Preview{}, err
	}
	if classObject.GetDeletionTimestamp() != nil || !sc.AllowExpansion {
		return manifest.Preview{}, errors.New("StorageClass 未允许扩容或正在删除")
	}
	desired := object.DeepCopy()
	if err := unstructured.SetNestedField(desired.Object, size.String(), "spec", "resources", "requests", "storage"); err != nil {
		return manifest.Preview{}, err
	}
	yaml, err := manifest.RenderDraft(desired)
	if err != nil {
		return manifest.Preview{}, err
	}
	return s.writer.PreviewGuarded(ctx, manifest.Request{Target: request.Target, Mode: "patch", Baseline: request.Baseline, YAML: yaml}, []manifest.Dependency{{Target: ref, Baseline: formedit.Baseline(classObject)}})
}

func expansionSize(claim Claim, capacity string) (apiresource.Quantity, error) {
	size, err := positiveQuantity(capacity)
	if err != nil {
		return size, err
	}
	if claim.Deleting || claim.Phase != "Bound" {
		return size, errors.New("仅支持扩容已绑定且未删除的 PVC")
	}
	if claim.ClassMode != "named" || claim.ClassName == "" {
		return size, errors.New("PVC 未指定可验证扩容能力的 StorageClass")
	}
	current, err := positiveQuantity(claim.Requested)
	if err != nil {
		return size, errors.New("PVC 当前申请容量无效")
	}
	if size.Cmp(current) <= 0 {
		return size, errors.New("扩容容量必须大于当前申请容量；不支持缩容")
	}
	if claim.Capacity != "" {
		actual, err := positiveQuantity(claim.Capacity)
		if err != nil {
			return size, errors.New("PVC 已报告容量无效")
		}
		if size.Cmp(actual) <= 0 {
			return size, errors.New("扩容容量必须大于当前已报告容量")
		}
	}
	return size, nil
}

func positiveQuantity(text string) (apiresource.Quantity, error) {
	quantity, err := apiresource.ParseQuantity(text)
	if err != nil || quantity.Sign() <= 0 {
		return quantity, errors.New("容量必须是正数 Kubernetes Quantity，例如 10Gi 或 500Mi")
	}
	return quantity, nil
}
