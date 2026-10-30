package workload

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type RollbackRequest struct {
	Target     resource.Ref      `json:"target"`
	Baseline   manifest.Baseline `json:"baseline"`
	ReplicaSet string            `json:"replicaSet"`
	Source     manifest.Baseline `json:"source"`
	Revision   string            `json:"revision"`
}
type guardedPreviewer interface {
	PreviewGuarded(context.Context, manifest.Request, []manifest.Dependency) (manifest.Preview, error)
}

func (s *Service) PreviewRollback(ctx context.Context, request RollbackRequest) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if err := validateExistingTarget(request.Target); err != nil {
		return manifest.Preview{}, err
	}
	if request.ReplicaSet == "" || request.Source.UID == "" || request.Source.ResourceVersion == "" || revisionNumber(request.Revision) == 0 {
		return manifest.Preview{}, errors.New("请选择明确的历史 ReplicaSet 和修订号")
	}
	client, err := s.resolver.ManifestClient(request.Target.ClusterID)
	if err != nil {
		return manifest.Preview{}, err
	}
	deployment, err := client.GetObject(ctx, request.Target)
	if err != nil {
		return manifest.Preview{}, err
	}
	if request.Baseline.UID == "" || request.Baseline.ResourceVersion == "" || formedit.Baseline(deployment) != request.Baseline {
		return manifest.Preview{}, errors.New("Deployment 已变化，请重新读取基线后检查回滚差异")
	}
	source := request.Target
	source.Resource = "replicasets"
	source.Name = request.ReplicaSet
	rs, err := client.GetObject(ctx, source)
	if err != nil {
		return manifest.Preview{}, err
	}
	if formedit.Baseline(rs) != request.Source || rs.GetAnnotations()[revisionAnnotation] != request.Revision {
		return manifest.Preview{}, errors.New("发布历史已变化，请刷新历史并重新选择版本")
	}
	desired, err := rollbackObject(deployment, rs)
	if err != nil {
		return manifest.Preview{}, err
	}
	data, err := json.Marshal(desired.Object)
	if err != nil {
		return manifest.Preview{}, errors.New("回滚内容不能序列化")
	}
	writer, ok := s.writer.(guardedPreviewer)
	if !ok {
		return manifest.Preview{}, errors.New("写入服务不支持关联版本保护")
	}
	return writer.PreviewGuarded(ctx, manifest.Request{Target: request.Target, Mode: "patch", Baseline: request.Baseline, YAML: string(data)}, []manifest.Dependency{{Target: source, Baseline: request.Source}})
}
func rollbackObject(deployment, rs *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if !ownsRevision(deployment, rs) {
		return nil, errors.New("历史 ReplicaSet 不属于当前 Deployment，不能回滚")
	}
	paused, _, _ := unstructured.NestedBool(deployment.Object, "spec", "paused")
	if paused {
		return nil, errors.New("Deployment 已暂停，请先恢复发布后再回滚")
	}
	if deployment.GetDeletionTimestamp() != nil || rs.GetDeletionTimestamp() != nil {
		return nil, errors.New("Deployment 或历史版本正在删除，不能回滚")
	}
	template, err := podTemplate(rs)
	if err != nil {
		return nil, err
	}
	current, err := podTemplate(deployment)
	if err != nil {
		return nil, err
	}
	if reflect.DeepEqual(template, current) {
		return nil, errors.New("所选版本与当前 Pod 模板相同，无需回滚")
	}
	copy := deployment.DeepCopy()
	if err = unstructured.SetNestedMap(copy.Object, template, "spec", "template"); err != nil {
		return nil, err
	}
	return copy, nil
}
