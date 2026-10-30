package node

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/validation"
)

func validateTarget(target resource.Ref) error {
	if target.ClusterID == "" || target.Group != "" || target.Version != "v1" || target.Resource != "nodes" || target.Namespace != "" || target.Name == "" || len(validation.IsDNS1123Subdomain(target.Name)) != 0 {
		return errors.New("请选择明确的集群级 Node")
	}
	return nil
}

func readNode(ctx context.Context, client Reader, target resource.Ref) (*unstructured.Unstructured, error) {
	if err := validateTarget(target); err != nil {
		return nil, err
	}
	object, err := client.GetObject(ctx, target)
	if err != nil {
		return nil, err
	}
	if object.GetName() != target.Name || object.GetKind() != "Node" || object.GetAPIVersion() != "v1" || object.GetNamespace() != "" || object.GetUID() == "" || object.GetResourceVersion() == "" || object.GetDeletionTimestamp() != nil {
		return nil, errors.New("节点身份无效或正在删除，请刷新")
	}
	return object, nil
}

// Inspect loads node metadata only. A denied Pod list does not hide node details.
func (s *Service) Inspect(ctx context.Context, target resource.Ref) (Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	client, err := s.resolver.NodeClient(target.ClusterID)
	if err != nil {
		return Snapshot{}, err
	}
	object, err := readNode(ctx, client, target)
	if err != nil {
		return Snapshot{}, err
	}
	return summarizeNode(object), nil
}

func summarizeNode(object *unstructured.Unstructured) Snapshot {
	result := Snapshot{Baseline: formedit.Baseline(object), Roles: []string{}, Addresses: map[string]string{}, Labels: object.GetLabels(), Taints: []Taint{}, Conditions: []Condition{}}
	result.Unschedulable, _, _ = unstructured.NestedBool(object.Object, "spec", "unschedulable")
	result.Info, _, _ = unstructured.NestedStringMap(object.Object, "status", "nodeInfo")
	result.Capacity, _, _ = unstructured.NestedStringMap(object.Object, "status", "capacity")
	result.Allocatable, _, _ = unstructured.NestedStringMap(object.Object, "status", "allocatable")
	for key, value := range result.Labels {
		if role, ok := strings.CutPrefix(key, "node-role.kubernetes.io/"); ok && role != "" {
			result.Roles = append(result.Roles, role)
		}
		if key == "kubernetes.io/role" && value != "" {
			result.Roles = append(result.Roles, value)
		}
	}
	sort.Strings(result.Roles)
	for _, item := range mapsAt(object, "status", "addresses") {
		result.Addresses[stringAt(item, "type")] = stringAt(item, "address")
	}
	for _, item := range mapsAt(object, "spec", "taints") {
		result.Taints = append(result.Taints, Taint{Key: stringAt(item, "key"), Value: stringAt(item, "value"), Effect: stringAt(item, "effect")})
	}
	for _, item := range mapsAt(object, "status", "conditions") {
		result.Conditions = append(result.Conditions, Condition{Type: stringAt(item, "type"), Status: stringAt(item, "status"), Reason: stringAt(item, "reason"), Message: stringAt(item, "message"), Updated: stringAt(item, "lastTransitionTime")})
	}
	return result
}

func stringAt(object map[string]interface{}, path ...string) string {
	value, _, _ := unstructured.NestedString(object, path...)
	return value
}

func mapsAt(object *unstructured.Unstructured, path ...string) []map[string]interface{} {
	values, _, _ := unstructured.NestedSlice(object.Object, path...)
	result := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]interface{}); ok {
			result = append(result, item)
		}
	}
	return result
}

func podTarget(node resource.Ref, pod Pod) resource.Ref {
	return resource.Ref{ClusterID: node.ClusterID, Version: "v1", Resource: "pods", Namespace: pod.Namespace, Name: pod.Name}
}

func summarizePod(object *unstructured.Unstructured) Pod {
	result := Pod{UID: string(object.GetUID()), Name: object.GetName(), Namespace: object.GetNamespace(), Phase: stringAt(object.Object, "status", "phase"), Category: "unmanaged", LocalData: []string{}, Terminating: object.GetDeletionTimestamp() != nil}
	for _, owner := range object.GetOwnerReferences() {
		if owner.Controller != nil && *owner.Controller {
			result.Owner = owner.Kind + "/" + owner.Name
			result.Category = "managed"
			if owner.Kind == "DaemonSet" && strings.HasPrefix(owner.APIVersion, "apps/") {
				result.Category = "daemonset"
			}
			break
		}
	}
	if _, ok := object.GetAnnotations()["kubernetes.io/config.mirror"]; ok {
		result.Category = "static"
	}
	for _, volume := range mapsAt(object, "spec", "volumes") {
		for _, kind := range []string{"emptyDir", "hostPath"} {
			if _, ok := volume[kind]; ok {
				result.LocalData = append(result.LocalData, stringAt(volume, "name")+" ("+kind+")")
			}
		}
	}
	return result
}

func listPods(ctx context.Context, client Reader, target resource.Ref, namespace, cursor string) (*unstructured.UnstructuredList, error) {
	ref := resource.Ref{ClusterID: target.ClusterID, Version: "v1", Resource: "pods", Namespace: namespace}
	page, err := client.ListObjects(ctx, ref, metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("spec.nodeName", target.Name).String(), Limit: 100, Continue: cursor})
	if err != nil {
		return nil, err
	}
	if len(page.Items) > 100 {
		return nil, errors.New("Pod 分页超过边界，请缩小范围")
	}
	for _, item := range page.Items {
		if item.GetUID() == "" || item.GetResourceVersion() == "" || item.GetName() == "" || item.GetNamespace() == "" || stringAt(item.Object, "spec", "nodeName") != target.Name || (namespace != "" && item.GetNamespace() != namespace) {
			return nil, errors.New("Pod 身份或节点归属不一致")
		}
	}
	return page, nil
}

func (s *Service) Pods(ctx context.Context, target resource.Ref, namespace, cursor string) (PodPage, error) {
	if err := validateTarget(target); err != nil {
		return PodPage{}, err
	}
	if namespace != "" && len(validation.IsDNS1123Label(namespace)) != 0 {
		return PodPage{}, errors.New("命名空间无效")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	client, err := s.resolver.NodeClient(target.ClusterID)
	if err != nil {
		return PodPage{}, err
	}
	page, err := listPods(ctx, client, target, namespace, cursor)
	if err != nil {
		return PodPage{}, err
	}
	result := PodPage{Pods: []Pod{}, Continue: page.GetContinue()}
	for i := range page.Items {
		result.Pods = append(result.Pods, summarizePod(&page.Items[i]))
	}
	return result, nil
}
