package relations

import (
	"context"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
)

func (r reader) statefulClaims(ctx context.Context, root *unstructured.Unstructured) Section {
	section := newSection("claims", "StatefulSet PVC")
	section.Note = "按 volumeClaimTemplates 的模板名、StatefulSet 名称和序号匹配同命名空间 PVC（命名规则，不证明所有权）；包含缩容后保留的 PVC。"
	templates, _, _ := unstructured.NestedSlice(root.Object, "spec", "volumeClaimTemplates")
	prefixes := []string{}
	for _, raw := range templates {
		if template, ok := raw.(map[string]interface{}); ok {
			name, _, _ := unstructured.NestedString(template, "metadata", "name")
			if name != "" {
				prefixes = append(prefixes, name+"-"+root.GetName()+"-")
			}
		}
	}
	if len(prefixes) == 0 {
		return section
	}
	objects := r.collect(ctx, &section, "", "persistentvolumeclaims", "", "", func(object unstructured.Unstructured) bool {
		for _, prefix := range prefixes {
			if strings.HasPrefix(object.GetName(), prefix) {
				suffix := strings.TrimPrefix(object.GetName(), prefix)
				ordinal, err := strconv.ParseInt(suffix, 10, 32)
				if err == nil && ordinal >= 0 && strconv.FormatInt(ordinal, 10) == suffix {
					return true
				}
			}
		}
		return false
	})
	r.nodes(&section, objects, "", "persistentvolumeclaims", "PVC 模板命名规则")
	return section
}
func (r reader) nodePods(ctx context.Context, root *unstructured.Unstructured) Section {
	section := newSection("pods", "节点 Pod")
	section.Note = "按 spec.nodeName 关联当前节点；Pod 会话另行校验 Pod UID。"
	objects := r.collect(ctx, &section, "", "pods", "", fields.OneTermEqualSelector("spec.nodeName", root.GetName()).String(), func(object unstructured.Unstructured) bool {
		name, _, _ := unstructured.NestedString(object.Object, "spec", "nodeName")
		return name == root.GetName()
	})
	r.nodes(&section, objects, "", "pods", "spec.nodeName")
	return section
}
