package workload

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workloadstate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func validateController(target resource.Ref, existing bool) error {
	if target.ClusterID == "" || target.Group != "apps" || target.Version != "v1" || controllerKind(target.Resource) == "" {
		return errors.New("此表单支持 apps/v1 StatefulSet、DaemonSet、ReplicaSet")
	}
	if existing && (target.Name == "" || target.Namespace == "") {
		return errors.New("请选择明确命名空间中的工作负载")
	}
	return nil
}
func (s *Service) Controller(ctx context.Context, target resource.Ref) (ControllerSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := validateController(target, true); err != nil {
		return ControllerSnapshot{}, err
	}
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return ControllerSnapshot{}, err
	}
	object, err := client.GetObject(ctx, target)
	if err != nil {
		return ControllerSnapshot{}, err
	}
	result, err := controllerSnapshot(object, target)
	if observer, ok := client.(workloadstate.Observer); ok && err == nil {
		result.Status.State, result.Status.Detail = observer.WorkloadState(ctx, *object)
	}
	return result, err
}
func (s *Service) PreviewController(ctx context.Context, request ControllerRequest) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if err := validateController(request.Target, request.Action != "create"); err != nil {
		return manifest.Preview{}, err
	}
	object, err := s.desiredController(ctx, request)
	if err != nil {
		return manifest.Preview{}, err
	}
	data, err := json.Marshal(object.Object)
	if err != nil {
		return manifest.Preview{}, err
	}
	mode := "patch"
	if request.Action == "create" {
		mode = "create"
	}
	return s.writer.Preview(ctx, manifest.Request{Target: request.Target, Mode: mode, YAML: string(data), Baseline: request.Baseline})
}
func (s *Service) desiredController(ctx context.Context, request ControllerRequest) (*unstructured.Unstructured, error) {
	if request.Action == "create" {
		return newController(request)
	}
	client, err := s.resolver.ManifestClient(request.Target.ClusterID)
	if err != nil {
		return nil, err
	}
	object, err := client.GetObject(ctx, request.Target)
	if err != nil {
		return nil, err
	}
	if request.Baseline.UID == "" || request.Baseline.ResourceVersion == "" || formedit.Baseline(object) != request.Baseline {
		return nil, errors.New("工作负载已变化，请重新读取基线后检查差异；当前草稿已保留")
	}
	if object.GetDeletionTimestamp() != nil {
		return nil, errors.New("工作负载正在删除")
	}
	copy := object.DeepCopy()
	switch request.Action {
	case "update":
		err = updateController(copy, request)
	case "restart":
		if request.Target.Resource == "replicasets" {
			return nil, errors.New("ReplicaSet 不支持滚动重启；请修改所属 Deployment 或显式管理 Pod")
		}
		err = unstructured.SetNestedField(copy.Object, time.Now().UTC().Format(time.RFC3339Nano), "spec", "template", "metadata", "annotations", "kubectl.kubernetes.io/restartedAt")
	default:
		return nil, errors.New("不支持的工作负载操作")
	}
	return copy, err
}
