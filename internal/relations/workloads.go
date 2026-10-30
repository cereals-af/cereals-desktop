package relations

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (r reader) descendants(ctx context.Context, root *unstructured.Unstructured, group, resourceName, kind string) []Section {
	children := newSection(resourceName, kind)
	children.Note = "通过 controller ownerReference 的 UID 关联；同名或标签相同但不同所有者的资源不会混入。修订号只展示历史，不执行回滚。"
	objects := r.collect(ctx, &children, group, resourceName, "", "", ownerIDs(root).controls)
	r.nodes(&children, objects, group, resourceName, "控制器 UID")
	ids := owners{}
	for _, object := range objects {
		ids[string(object.GetUID())] = kind
	}
	pods := r.pods(ctx, ids, "由已发现的 "+kind+" UID 管理的 Pod")
	if children.Error != "" || children.Truncated {
		pods.Note += "；上游关联不完整，此处可能遗漏 Pod。"
		pods.Truncated = true
	}
	return []Section{children, pods}
}

func (r reader) pods(ctx context.Context, ids owners, note string) Section {
	section := newSection("pods", "Pod")
	section.Note = note
	if len(ids) == 0 {
		return section
	}
	objects := r.collect(ctx, &section, "", "pods", "", "", ids.controls)
	r.nodes(&section, objects, "", "pods", "控制器 UID")
	return section
}
