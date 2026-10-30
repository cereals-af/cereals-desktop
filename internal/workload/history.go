package workload

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const revisionAnnotation = "deployment.kubernetes.io/revision"

type historyReader interface {
	ListObjects(context.Context, resource.Ref, metav1.ListOptions) (*unstructured.UnstructuredList, error)
}
type Revision struct {
	Name        string            `json:"name"`
	Baseline    manifest.Baseline `json:"baseline"`
	Revision    string            `json:"revision"`
	CreatedAt   string            `json:"createdAt"`
	Images      []string          `json:"images"`
	Replicas    int64             `json:"replicas"`
	Ready       int64             `json:"ready"`
	Current     bool              `json:"current"`
	Unavailable string            `json:"unavailable"`
}
type History struct {
	Baseline        manifest.Baseline `json:"baseline"`
	CurrentRevision string            `json:"currentRevision"`
	Paused          bool              `json:"paused"`
	Revisions       []Revision        `json:"revisions"`
	Warnings        []string          `json:"warnings"`
	Truncated       bool              `json:"truncated"`
}

func (s *Service) History(ctx context.Context, target resource.Ref) (History, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if err := validateExistingTarget(target); err != nil {
		return History{}, err
	}
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return History{}, err
	}
	deployment, err := client.GetObject(ctx, target)
	if err != nil {
		return History{}, err
	}
	reader, ok := client.(historyReader)
	if !ok {
		return History{}, errors.New("该连接不支持发布历史查询")
	}
	selector, err := deploymentSelector(deployment)
	if err != nil {
		return History{}, err
	}
	paused, _, _ := unstructured.NestedBool(deployment.Object, "spec", "paused")
	history := History{Baseline: formedit.Baseline(deployment), CurrentRevision: deployment.GetAnnotations()[revisionAnnotation], Paused: paused, Revisions: []Revision{}, Warnings: []string{}}
	s.collectHistory(ctx, reader, target, deployment, selector, &history)
	sort.SliceStable(history.Revisions, func(i, j int) bool {
		a, b := revisionNumber(history.Revisions[i].Revision), revisionNumber(history.Revisions[j].Revision)
		if a != b {
			return a > b
		}
		return history.Revisions[i].Name < history.Revisions[j].Name
	})
	return history, nil
}
func (s *Service) collectHistory(ctx context.Context, reader historyReader, target resource.Ref, deployment *unstructured.Unstructured, selector string, history *History) {
	target.Resource = "replicasets"
	options := metav1.ListOptions{Limit: 100, LabelSelector: selector}
	seenPages, seenUIDs := map[string]bool{}, map[string]bool{}
	scanned := 0
	for page := 0; page < 20; page++ {
		objects, err := reader.ListObjects(ctx, target, options)
		if err != nil {
			history.Warnings = append(history.Warnings, "发布历史读取不完整："+err.Error())
			return
		}
		for i := range objects.Items {
			scanned++
			object := &objects.Items[i]
			if !seenUIDs[string(object.GetUID())] && ownsRevision(deployment, object) {
				history.Revisions = append(history.Revisions, summarizeRevision(deployment, object))
				seenUIDs[string(object.GetUID())] = true
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
func deploymentSelector(object *unstructured.Unstructured) (string, error) {
	data, found, err := unstructured.NestedMap(object.Object, "spec", "selector")
	if err != nil || !found {
		return "", errors.New("Deployment 没有有效选择器")
	}
	var selector metav1.LabelSelector
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(data, &selector); err != nil {
		return "", errors.New("Deployment 选择器无效")
	}
	query, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil || query.Empty() {
		return "", errors.New("Deployment 选择器为空或无效，无法安全查询历史")
	}
	return query.String(), nil
}
func ownsRevision(deployment, rs *unstructured.Unstructured) bool {
	if deployment.GetUID() == "" || rs.GetUID() == "" || rs.GetNamespace() != deployment.GetNamespace() || rs.GetKind() != "ReplicaSet" || rs.GetAPIVersion() != "apps/v1" {
		return false
	}
	for _, owner := range rs.GetOwnerReferences() {
		if owner.Controller != nil && *owner.Controller && owner.APIVersion == "apps/v1" && owner.Kind == "Deployment" && owner.UID == deployment.GetUID() && owner.Name == deployment.GetName() {
			return true
		}
	}
	return false
}
func revisionNumber(value string) int64 {
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number < 1 {
		return 0
	}
	return number
}
func podTemplate(object *unstructured.Unstructured) (map[string]interface{}, error) {
	template, found, err := unstructured.NestedMap(object.Object, "spec", "template")
	if err != nil || !found {
		return nil, errors.New("历史版本缺少有效 Pod 模板")
	}
	// This is a controller-generated selector label, not part of the Deployment template.
	unstructured.RemoveNestedField(template, "metadata", "labels", "pod-template-hash")
	return template, nil
}
func summarizeRevision(deployment, rs *unstructured.Unstructured) Revision {
	result := Revision{Name: rs.GetName(), Baseline: formedit.Baseline(rs), Revision: rs.GetAnnotations()[revisionAnnotation], CreatedAt: rs.GetCreationTimestamp().Format(time.RFC3339), Images: []string{}}
	result.Replicas, _, _ = unstructured.NestedInt64(rs.Object, "spec", "replicas")
	result.Ready, _, _ = unstructured.NestedInt64(rs.Object, "status", "readyReplicas")
	template, err := podTemplate(rs)
	if err != nil {
		result.Unavailable = err.Error()
	}
	current, _ := podTemplate(deployment)
	result.Current = err == nil && reflect.DeepEqual(template, current)
	if revisionNumber(result.Revision) == 0 {
		result.Unavailable = "修订号缺失或无效"
	}
	if rs.GetDeletionTimestamp() != nil {
		result.Unavailable = "ReplicaSet 正在删除"
	}
	containers, _, _ := unstructured.NestedSlice(rs.Object, "spec", "template", "spec", "containers")
	for _, value := range containers[:min(len(containers), 16)] {
		if container, ok := value.(map[string]interface{}); ok {
			name, _, _ := unstructured.NestedString(container, "name")
			image, _, _ := unstructured.NestedString(container, "image")
			if len(image) > 512 {
				image = image[:512] + "…"
			}
			result.Images = append(result.Images, name+": "+image)
		}
	}
	return result
}
