package relations

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (r reader) claimStorage(ctx context.Context, claim *unstructured.Unstructured) []Section {
	volume := newSection("volume", "绑定 PV")
	volume.Note = "同时核对 PVC volumeName 与 PV claimRef UID；绑定关系不代表挂载健康。"
	name, _, _ := unstructured.NestedString(claim.Object, "spec", "volumeName")
	if name != "" {
		ref := resource.Ref{ClusterID: r.target.ClusterID, Version: "v1", Resource: "persistentvolumes", Name: name}
		pv := r.storageObject(ctx, &volume, ref, "PersistentVolume")
		if pv != nil {
			if boundTo(pv, claim) {
				volume.Items = append(volume.Items, summarize(*pv, ref, "双向绑定已核对"))
			} else {
				volume.Error = "PV claimRef 与当前 PVC UID 不一致，请刷新并检查绑定"
			}
		}
	}
	return []Section{volume, r.storageClass(ctx, claim), r.claimPods(ctx, claim)}
}

func (r reader) volumeStorage(ctx context.Context, pv *unstructured.Unstructured) []Section {
	section := newSection("claim", "绑定 PVC")
	section.Note = "按 claimRef UID 与 PVC volumeName 双向核对；Released PV 可能引用已删除的 PVC。"
	name, _, _ := unstructured.NestedString(pv.Object, "spec", "claimRef", "name")
	namespace, _, _ := unstructured.NestedString(pv.Object, "spec", "claimRef", "namespace")
	var claim *unstructured.Unstructured
	if name != "" && namespace != "" {
		ref := resource.Ref{ClusterID: r.target.ClusterID, Version: "v1", Resource: "persistentvolumeclaims", Name: name, Namespace: namespace}
		claim = r.storageObject(ctx, &section, ref, "PersistentVolumeClaim")
		if claim != nil {
			if boundTo(pv, claim) {
				section.Items = append(section.Items, summarize(*claim, ref, "双向绑定已核对"))
			} else {
				section.Error = "claimRef 与当前 PVC 不一致，未关联同名新资源"
				claim = nil
			}
		}
	}
	sections := []Section{section, r.storageClass(ctx, pv)}
	if claim != nil {
		sections = append(sections, r.claimPods(ctx, claim))
	}
	return sections
}

func boundTo(pv, claim *unstructured.Unstructured) bool {
	uid, _, _ := unstructured.NestedString(pv.Object, "spec", "claimRef", "uid")
	name, _, _ := unstructured.NestedString(pv.Object, "spec", "claimRef", "name")
	namespace, _, _ := unstructured.NestedString(pv.Object, "spec", "claimRef", "namespace")
	volume, _, _ := unstructured.NestedString(claim.Object, "spec", "volumeName")
	return uid != "" && uid == string(claim.GetUID()) && name == claim.GetName() && namespace == claim.GetNamespace() && volume == pv.GetName()
}

func (r reader) storageClass(ctx context.Context, object *unstructured.Unstructured) Section {
	section := newSection("storageclass", "StorageClass")
	name, _, _ := unstructured.NestedString(object.Object, "spec", "storageClassName")
	section.Note = "按 storageClassName 引用查询当前对象。未指定类不会推断默认类或历史存储驱动。"
	if name == "" {
		return section
	}
	ref := resource.Ref{ClusterID: r.target.ClusterID, Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses", Name: name}
	if sc := r.storageObject(ctx, &section, ref, "StorageClass"); sc != nil {
		section.Items = append(section.Items, summarize(*sc, ref, "存储类名称引用"))
	}
	return section
}

func (r reader) storageObject(ctx context.Context, section *Section, ref resource.Ref, kind string) *unstructured.Unstructured {
	object, err := r.client.GetObject(ctx, ref)
	if err != nil {
		section.Error = err.Error()
		return nil
	}
	if object == nil || object.GetUID() == "" || object.GetKind() != kind || object.GetName() != ref.Name || object.GetNamespace() != ref.Namespace {
		section.Error = "关联对象身份无效"
		return nil
	}
	return object
}

func (r reader) claimPods(ctx context.Context, claim *unstructured.Unstructured) Section {
	section := newSection("consumers", "引用此 PVC 的 Pod")
	section.Note = "扫描 PVC 同命名空间内 Pod 的 volumes 引用，包含通用临时卷的属主核对；名称引用不能证明挂载成功。最多扫描 2000 项，权限失败或截断会单独提示。"
	namespaced := reader{client: r.client, target: r.target}
	namespaced.target.Namespace = claim.GetNamespace()
	objects := namespaced.collect(ctx, &section, "", "pods", "", "", func(pod unstructured.Unstructured) bool {
		volumes, _, _ := unstructured.NestedSlice(pod.Object, "spec", "volumes")
		for _, raw := range volumes {
			volume, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			name, _, _ := unstructured.NestedString(volume, "persistentVolumeClaim", "claimName")
			if name == claim.GetName() {
				return true
			}
			if _, exists := volume["ephemeral"]; exists {
				volumeName, _, _ := unstructured.NestedString(volume, "name")
				if pod.GetName()+"-"+volumeName == claim.GetName() && ownerIDs(&pod).controls(*claim) {
					return true
				}
			}
		}
		return false
	})
	namespaced.nodes(&section, objects, "", "pods", "PVC 卷引用；节点见详情列")
	return section
}

func (r reader) classStorage(ctx context.Context, class *unstructured.Unstructured) []Section {
	sections := []Section{}
	for _, entry := range []struct{ resource, title string }{{"persistentvolumeclaims", "使用此存储类的 PVC"}, {"persistentvolumes", "使用此存储类的 PV"}} {
		section := newSection(entry.resource, entry.title)
		section.Note = "跨命名空间按 storageClassName 名称匹配，最多扫描 2000 项；不推断未指定类的 PVC。"
		objects := r.collect(ctx, &section, "", entry.resource, "", "", func(object unstructured.Unstructured) bool {
			name, _, _ := unstructured.NestedString(object.Object, "spec", "storageClassName")
			return name == class.GetName()
		})
		r.nodes(&section, objects, "", entry.resource, "存储类名称引用")
		sections = append(sections, section)
	}
	return sections
}
