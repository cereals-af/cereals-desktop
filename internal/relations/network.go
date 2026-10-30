package relations

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (r reader) service(ctx context.Context, root *unstructured.Unstructured) []Section {
	slices := newSection("endpointslices", "EndpointSlice")
	slices.Note = "按 kubernetes.io/service-name 标签关联，包含手动维护的切片；就绪端点不等于实际连通性。"
	selector := labels.Set{"kubernetes.io/service-name": root.GetName()}.String()
	objects := r.collect(ctx, &slices, "discovery.k8s.io", "endpointslices", selector, "", func(object unstructured.Unstructured) bool {
		return object.GetLabels()["kubernetes.io/service-name"] == root.GetName()
	})
	r.nodes(&slices, objects, "discovery.k8s.io", "endpointslices", "Service 名称标签")
	pods := newSection("pods", "选择器匹配的 Pod")
	pods.Note = "此处按 Service 选择器匹配，不表示所有权；实际后端以 EndpointSlice 为准。"
	match, _, _ := unstructured.NestedStringMap(root.Object, "spec", "selector")
	kind, _, _ := unstructured.NestedString(root.Object, "spec", "type")
	if len(match) == 0 || kind == "ExternalName" {
		pods.Note = "Service 没有 Pod 选择器或为 ExternalName，不执行全命名空间 Pod 查询。"
		return []Section{slices, pods}
	}
	selected := labels.SelectorFromSet(match)
	objects = r.collect(ctx, &pods, "", "pods", selected.String(), "", func(object unstructured.Unstructured) bool {
		return selected.Matches(labels.Set(object.GetLabels()))
	})
	r.nodes(&pods, objects, "", "pods", "标签选择器")
	return []Section{slices, pods}
}
