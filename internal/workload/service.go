package workload

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workloadstate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Service struct {
	resolver manifest.Resolver
	writer   Previewer
}

func NewService(resolver manifest.Resolver, writer Previewer) *Service {
	return &Service{resolver: resolver, writer: writer}
}

func (s *Service) Deployment(ctx context.Context, target resource.Ref) (Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := validateExistingTarget(target); err != nil {
		return Snapshot{}, err
	}
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return Snapshot{}, err
	}
	object, err := client.GetObject(ctx, target)
	if err != nil {
		return Snapshot{}, err
	}
	result, err := snapshot(object)
	if observer, ok := client.(workloadstate.Observer); ok && err == nil {
		result.Status.State, result.Status.Detail = observer.WorkloadState(ctx, *object)
	}
	return result, err
}

// Preview edits a full server object and delegates validation, dry run, tickets and
// commit preconditions to ManifestService. Typed DTOs are never used as replacements.
func (s *Service) Preview(ctx context.Context, request Request) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if err := validateTarget(request.Target); err != nil {
		return manifest.Preview{}, err
	}
	if request.Action == "scale" {
		writer, ok := s.writer.(interface {
			PreviewScale(context.Context, manifest.ScaleRequest) (manifest.Preview, error)
		})
		if !ok {
			return manifest.Preview{}, errors.New("写入服务不支持 scale 子资源")
		}
		return writer.PreviewScale(ctx, manifest.ScaleRequest{Target: request.Target, Baseline: request.Baseline, Replicas: request.Form.Replicas, Method: "update"})
	}
	object, err := s.desiredObject(ctx, request)
	if err != nil {
		return manifest.Preview{}, err
	}
	data, err := json.Marshal(object.Object)
	if err != nil {
		return manifest.Preview{}, errors.New("Deployment 不能序列化")
	}
	mode := "patch"
	if request.Action == "create" {
		mode = "create"
	}
	// JSON is also YAML; serialization in Go keeps all int64 values exact.
	return s.writer.Preview(ctx, manifest.Request{Target: request.Target, Mode: mode, YAML: string(data), Baseline: request.Baseline})
}

func (s *Service) desiredObject(ctx context.Context, request Request) (*unstructured.Unstructured, error) {
	if request.Action == "create" {
		return newDeployment(request)
	}
	client, err := s.resolver.ManifestClient(request.Target.ClusterID)
	if err != nil {
		return nil, err
	}
	object, err := client.GetObject(ctx, request.Target)
	if err != nil {
		return nil, err
	}
	if request.Baseline.UID == "" || request.Baseline.ResourceVersion == "" || string(object.GetUID()) != request.Baseline.UID || object.GetResourceVersion() != request.Baseline.ResourceVersion {
		return nil, errors.New("资源已变化或被重新创建，请重新读取基线后比较差异；当前草稿已保留")
	}
	copy := object.DeepCopy()
	switch request.Action {
	case "update":
		err = updateDeployment(copy, request.Form)
	case "pause", "resume":
		paused, _, _ := unstructured.NestedBool(copy.Object, "spec", "paused")
		next := request.Action == "pause"
		if paused == next {
			return nil, errors.New("发布暂停状态已变化，请刷新后重新选择操作")
		}
		err = unstructured.SetNestedField(copy.Object, next, "spec", "paused")
	case "restart":
		paused, _, _ := unstructured.NestedBool(copy.Object, "spec", "paused")
		if paused {
			return nil, errors.New("Deployment 已暂停，请先恢复发布，再重启")
		}
		err = unstructured.SetNestedField(copy.Object, time.Now().UTC().Format(time.RFC3339Nano), "spec", "template", "metadata", "annotations", "kubectl.kubernetes.io/restartedAt")
	default:
		return nil, errors.New("不支持的 Deployment 操作")
	}
	return copy, err
}

func validateTarget(target resource.Ref) error {
	if target.ClusterID == "" || target.Group != "apps" || target.Version != "v1" || target.Resource != "deployments" {
		return errors.New("此表单仅支持 apps/v1 Deployment")
	}
	return nil
}

func validateExistingTarget(target resource.Ref) error {
	if err := validateTarget(target); err != nil {
		return err
	}
	if target.Namespace == "" || target.Name == "" {
		return errors.New("请选择明确命名空间中的 Deployment")
	}
	return nil
}
