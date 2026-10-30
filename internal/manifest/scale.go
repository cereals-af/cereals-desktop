package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ScaleReader needs no read access to the parent workload.
type ScaleReader interface {
	GetScale(context.Context, resource.Ref) (*unstructured.Unstructured, error)
}
type ScaleSnapshot struct {
	Baseline Baseline `json:"baseline"`
	Replicas int32    `json:"replicas"`
	Actual   int32    `json:"actual"`
	Selector string   `json:"selector"`
}
type ScaleRequest struct {
	Target   resource.Ref `json:"target"`
	Baseline Baseline     `json:"baseline"`
	Replicas int32        `json:"replicas"`
	Method   string       `json:"method"` // update or patch: independent RBAC verbs.
}

func scaleTarget(target resource.Ref) error {
	supported := target.Group == "apps" && slices.Contains([]string{"deployments", "statefulsets", "replicasets"}, target.Resource) || target.Group == "" && target.Resource == "replicationcontrollers"
	if !supported || target.Version != "v1" || target.ClusterID == "" || target.Namespace == "" || target.Name == "" {
		return errors.New("请选择明确命名空间中的 Deployment、StatefulSet、ReplicaSet 或 ReplicationController")
	}
	return nil
}
func readScale(ctx context.Context, client Client, target resource.Ref) (*unstructured.Unstructured, error) {
	if err := scaleTarget(target); err != nil {
		return nil, err
	}
	reader, ok := client.(ScaleReader)
	if !ok {
		return nil, errors.New("该连接不支持 scale 子资源")
	}
	object, err := reader.GetScale(ctx, target)
	if err != nil {
		return nil, err
	}
	if object.GetKind() != "Scale" || object.GetAPIVersion() != "autoscaling/v1" || object.GetName() != target.Name || object.GetNamespace() != target.Namespace || object.GetUID() == "" || object.GetResourceVersion() == "" {
		return nil, errors.New("服务端返回的 Scale 身份无效")
	}
	return object, nil
}
func (s *Service) Scale(ctx context.Context, target resource.Ref) (ScaleSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return ScaleSnapshot{}, err
	}
	object, err := readScale(ctx, client, target)
	if err != nil {
		return ScaleSnapshot{}, err
	}
	desired, _, err := unstructured.NestedInt64(object.Object, "spec", "replicas")
	if err != nil || desired < 0 || desired > 2147483647 {
		return ScaleSnapshot{}, errors.New("Scale 副本数无效")
	}
	actual, _, _ := unstructured.NestedInt64(object.Object, "status", "replicas")
	selector, _, _ := unstructured.NestedString(object.Object, "status", "selector")
	return ScaleSnapshot{Baseline: baseline(object), Replicas: int32(desired), Actual: int32(actual), Selector: selector}, nil
}
func (s *Service) PreviewScale(ctx context.Context, request ScaleRequest) (Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if request.Replicas < 0 || !slices.Contains([]string{"update", "patch"}, request.Method) {
		return Preview{}, errors.New("副本数不能为负，请选择 update 或 patch 权限对应的提交方式")
	}
	client, err := s.resolver.ManifestClient(request.Target.ClusterID)
	if err != nil {
		return Preview{}, err
	}
	object, err := readScale(ctx, client, request.Target)
	if err != nil {
		return Preview{}, err
	}
	if err = checkBaseline(object, request.Baseline); err != nil {
		return Preview{}, err
	}
	// Only Scale identity and spec are written; parent template/annotations never enter this operation.
	clean := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "autoscaling/v1", "kind": "Scale",
		"metadata": map[string]interface{}{"name": object.GetName(), "namespace": object.GetNamespace()},
		"spec":     map[string]interface{}{"replicas": int64(request.Replicas)},
	}}
	before, err := render(editableObject(object))
	if err != nil {
		return Preview{}, err
	}
	desired, err := render(clean)
	if err != nil {
		return Preview{}, err
	}
	clean.SetUID(object.GetUID())
	clean.SetResourceVersion(object.GetResourceVersion())
	data, err := json.Marshal(clean.Object)
	if err != nil {
		return Preview{}, errors.New("Scale 不能序列化")
	}
	operation := Operation{Target: request.Target, Mode: request.Method, Subresource: "scale", Data: data}
	return s.previewOperation(ctx, client, operation, before, desired, "scale", nil)
}
