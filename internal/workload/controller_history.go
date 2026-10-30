package workload

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func validateControllerHistory(target resource.Ref) error {
	if err := validateController(target, true); err != nil {
		return err
	}
	if target.Resource == "replicasets" {
		return errors.New("ReplicaSet 没有独立发布历史，请查看所属 Deployment")
	}
	return nil
}
func (s *Service) ControllerHistory(ctx context.Context, target resource.Ref) (History, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if err := validateControllerHistory(target); err != nil {
		return History{}, err
	}
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return History{}, err
	}
	object, err := client.GetObject(ctx, target)
	if err != nil {
		return History{}, err
	}
	reader, ok := client.(historyReader)
	if !ok {
		return History{}, errors.New("该连接不支持发布历史查询")
	}
	history := History{Baseline: formedit.Baseline(object), CurrentRevision: stringField(object, "status", "updateRevision"), Revisions: []Revision{}, Warnings: []string{}}
	collectControllerHistory(ctx, reader, target, object, &history)
	sort.SliceStable(history.Revisions, func(i, j int) bool {
		a, b := revisionNumber(history.Revisions[i].Revision), revisionNumber(history.Revisions[j].Revision)
		if a != b {
			return a > b
		}
		return history.Revisions[i].Name < history.Revisions[j].Name
	})
	return history, nil
}
func collectControllerHistory(ctx context.Context, reader historyReader, target resource.Ref, object *unstructured.Unstructured, history *History) {
	target.Resource = "controllerrevisions"
	target.Name = ""
	options := metav1.ListOptions{Limit: 100}
	seenPages, seenUIDs := map[string]bool{}, map[string]bool{}
	scanned := 0
	for page := 0; page < 20; page++ {
		objects, err := reader.ListObjects(ctx, target, options)
		if err != nil {
			history.Warnings = append(history.Warnings, "历史查询不完整："+err.Error())
			return
		}
		for i := range objects.Items {
			revision := &objects.Items[i]
			scanned++
			uid := string(revision.GetUID())
			if !seenUIDs[uid] && ownsControllerRevision(object, revision) {
				seenUIDs[uid] = true
				history.Revisions = append(history.Revisions, controllerRevisionSummary(object, revision))
			}
			if scanned >= 2000 {
				history.Truncated = objects.GetContinue() != "" || i < len(objects.Items)-1
				return
			}
		}
		options.Continue = objects.GetContinue()
		if options.Continue == "" {
			return
		}
		if seenPages[options.Continue] {
			history.Truncated = true
			return
		}
		seenPages[options.Continue] = true
	}
	history.Truncated = true
}
func ownsControllerRevision(object, revision *unstructured.Unstructured) bool {
	if object.GetUID() == "" || revision.GetUID() == "" || revision.GetNamespace() != object.GetNamespace() || revision.GetKind() != "ControllerRevision" || revision.GetAPIVersion() != "apps/v1" {
		return false
	}
	for _, owner := range revision.GetOwnerReferences() {
		if owner.Controller != nil && *owner.Controller && owner.APIVersion == "apps/v1" && owner.Kind == object.GetKind() && owner.Name == object.GetName() && owner.UID == object.GetUID() {
			return true
		}
	}
	return false
}

// Kubernetes STS/DS controllers store a replacement Pod template patch. Only that
// documented shape is accepted; future or custom patch formats fail closed.
func controllerRevisionTemplate(revision *unstructured.Unstructured) (map[string]interface{}, error) {
	data, found, err := unstructured.NestedMap(revision.Object, "data")
	if err != nil || !found || len(data) != 1 {
		return nil, errors.New("不支持的 ControllerRevision 数据格式")
	}
	spec, ok := data["spec"].(map[string]interface{})
	if !ok || len(spec) != 1 {
		return nil, errors.New("历史包含非 Pod 模板变更，不能安全回退")
	}
	template, ok := spec["template"].(map[string]interface{})
	if !ok || template["$patch"] != "replace" {
		return nil, errors.New("历史缺少 Pod 模板 replace 标记")
	}
	delete(template, "$patch")
	if hasPatchDirective(template) {
		return nil, errors.New("历史包含额外补丁指令，不能安全回退")
	}
	containers, found, err := unstructured.NestedSlice(template, "spec", "containers")
	if err != nil || !found || len(containers) == 0 {
		return nil, errors.New("历史 Pod 模板缺少容器")
	}
	normalizeTemplate(template)
	return template, nil
}
func hasPatchDirective(value interface{}) bool {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if strings.HasPrefix(key, "$") || hasPatchDirective(child) {
				return true
			}
		}
	case []interface{}:
		for _, child := range typed {
			if hasPatchDirective(child) {
				return true
			}
		}
	}
	return false
}
func normalizeTemplate(template map[string]interface{}) {
	if value, found, _ := unstructured.NestedFieldNoCopy(template, "metadata", "creationTimestamp"); found && value == nil {
		unstructured.RemoveNestedField(template, "metadata", "creationTimestamp")
	}
}
func controllerRevisionSummary(object, revision *unstructured.Unstructured) Revision {
	result := Revision{Name: revision.GetName(), Baseline: formedit.Baseline(revision), Revision: strconv.FormatInt(numberField(revision, "revision"), 10), CreatedAt: revision.GetCreationTimestamp().Format(time.RFC3339), Images: []string{}}
	template, err := controllerRevisionTemplate(revision)
	if err != nil {
		result.Unavailable = err.Error()
	} else {
		current, _, _ := unstructured.NestedMap(object.Object, "spec", "template")
		normalizeTemplate(current)
		result.Current = reflect.DeepEqual(current, template)
		containers, _, _ := unstructured.NestedSlice(template, "spec", "containers")
		for _, raw := range containers[:min(len(containers), 16)] {
			if c, ok := raw.(map[string]interface{}); ok {
				u := &unstructured.Unstructured{Object: c}
				result.Images = append(result.Images, stringField(u, "name")+": "+stringField(u, "image"))
			}
		}
	}
	if revisionNumber(result.Revision) == 0 {
		result.Unavailable = "修订号无效"
	}
	if revision.GetDeletionTimestamp() != nil {
		result.Unavailable = "历史版本正在删除"
	}
	return result
}
