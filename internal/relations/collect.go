package relations

import (
	"context"
	"sort"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const maxScanned = 2000

// collect bounds namespace scans even when most objects do not match an owner.
// A failed later page preserves earlier matches and explicitly marks incompleteness.
func (r reader) collect(ctx context.Context, section *Section, group, resourceName, labels, fields string, matches func(unstructured.Unstructured) bool) []unstructured.Unstructured {
	target := resource.Ref{ClusterID: r.target.ClusterID, Group: group, Version: "v1", Resource: resourceName, Namespace: r.target.Namespace}
	options := metav1.ListOptions{Limit: 100, LabelSelector: labels, FieldSelector: fields}
	result := []unstructured.Unstructured{}
	seen := map[string]bool{}
	seenUIDs := map[string]bool{}
	scanned := 0
	for pages := 0; ; pages++ {
		page, err := r.client.ListObjects(ctx, target, options)
		if err != nil {
			section.Error = err.Error()
			return result
		}
		for _, object := range page.Items {
			if scanned >= maxScanned {
				section.Truncated = true
				return result
			}
			scanned++
			uid := string(object.GetUID())
			if uid != "" && !seenUIDs[uid] && (target.Namespace == "" || object.GetNamespace() == target.Namespace) && (matches == nil || matches(object)) {
				result = append(result, object)
				seenUIDs[uid] = true
			}
		}
		options.Continue = page.GetContinue()
		if options.Continue == "" {
			return result
		}
		if scanned >= maxScanned || pages >= 19 || seen[options.Continue] {
			section.Truncated = true
			return result
		}
		seen[options.Continue] = true
	}
}

func (r reader) nodes(section *Section, objects []unstructured.Unstructured, group, resourceName, relation string) {
	for _, object := range objects {
		ref := resource.Ref{ClusterID: r.target.ClusterID, Group: group, Version: "v1", Resource: resourceName,
			Namespace: object.GetNamespace(), Name: object.GetName()}
		section.Items = append(section.Items, summarize(object, ref, relation))
	}
	sort.Slice(section.Items, func(i, j int) bool { return section.Items[i].Target.Name < section.Items[j].Target.Name })
}

type owners map[string]string // UID -> Kind, never name-only matching.

func ownerIDs(objects ...*unstructured.Unstructured) owners {
	result := owners{}
	for _, object := range objects {
		result[string(object.GetUID())] = object.GetKind()
	}
	return result
}

func (ids owners) controls(object unstructured.Unstructured) bool {
	for _, owner := range object.GetOwnerReferences() {
		if owner.Controller != nil && *owner.Controller && owner.UID != "" && ids[string(owner.UID)] == owner.Kind {
			return true
		}
	}
	return false
}
