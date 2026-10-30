package workload

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (s *Service) PreviewControllerRollback(ctx context.Context, request ControllerRollbackRequest) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if err := validateControllerHistory(request.Target); err != nil {
		return manifest.Preview{}, err
	}
	if request.Name == "" || request.Source.UID == "" || request.Source.ResourceVersion == "" || revisionNumber(request.Revision) == 0 {
		return manifest.Preview{}, errors.New("请选择明确的 ControllerRevision 和修订号")
	}
	client, err := s.resolver.ManifestClient(request.Target.ClusterID)
	if err != nil {
		return manifest.Preview{}, err
	}
	object, err := client.GetObject(ctx, request.Target)
	if err != nil {
		return manifest.Preview{}, err
	}
	if request.Baseline.UID == "" || request.Baseline.ResourceVersion == "" || formedit.Baseline(object) != request.Baseline {
		return manifest.Preview{}, errors.New("工作负载已变化，请刷新基线后重新检查回退差异")
	}
	source := request.Target
	source.Resource = "controllerrevisions"
	source.Name = request.Name
	revision, err := client.GetObject(ctx, source)
	if err != nil {
		return manifest.Preview{}, err
	}
	if formedit.Baseline(revision) != request.Source || strconv.FormatInt(numberField(revision, "revision"), 10) != request.Revision {
		return manifest.Preview{}, errors.New("历史版本已变化，请刷新后重新选择")
	}
	if !ownsControllerRevision(object, revision) {
		return manifest.Preview{}, errors.New("ControllerRevision 不属于当前工作负载 UID")
	}
	if object.GetDeletionTimestamp() != nil || revision.GetDeletionTimestamp() != nil {
		return manifest.Preview{}, errors.New("工作负载或历史版本正在删除")
	}
	summary := controllerRevisionSummary(object, revision)
	if summary.Unavailable != "" {
		return manifest.Preview{}, errors.New(summary.Unavailable)
	}
	if summary.Current {
		return manifest.Preview{}, errors.New("该版本与当前模板相同，无需回退")
	}
	template, err := controllerRevisionTemplate(revision)
	if err != nil {
		return manifest.Preview{}, err
	}
	copy := object.DeepCopy()
	if err = unstructured.SetNestedMap(copy.Object, template, "spec", "template"); err != nil {
		return manifest.Preview{}, err
	}
	data, err := json.Marshal(copy.Object)
	if err != nil {
		return manifest.Preview{}, err
	}
	writer, ok := s.writer.(guardedPreviewer)
	if !ok {
		return manifest.Preview{}, errors.New("写入服务不支持关联版本保护")
	}
	return writer.PreviewGuarded(ctx, manifest.Request{Target: request.Target, Mode: "patch", YAML: string(data), Baseline: request.Baseline}, []manifest.Dependency{{Target: source, Baseline: request.Source}})
}
