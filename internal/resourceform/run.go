package resourceform

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

// PreviewRun freezes a CronJob template and creates a separately named Job after review.
func (s *Service) PreviewRun(ctx context.Context, request RunRequest) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if err := validateTarget(request.Target, true); err != nil {
		return manifest.Preview{}, err
	}
	if kind(request.Target) != "CronJob" || len(validation.IsDNS1123Subdomain(request.Name)) != 0 {
		return manifest.Preview{}, errors.New("请选择 CronJob 并填写有效的新 Job 名称")
	}
	source, err := s.editor.Read(ctx, request.Target)
	if err != nil {
		return manifest.Preview{}, err
	}
	if request.Baseline.UID == "" || request.Baseline.ResourceVersion == "" || formedit.Baseline(source) != request.Baseline || source.GetDeletionTimestamp() != nil {
		return manifest.Preview{}, errors.New("CronJob 已变化或正在删除，请刷新后重新预检")
	}
	template, found, err := unstructured.NestedMap(source.Object, "spec", "jobTemplate")
	if err != nil || !found {
		return manifest.Preview{}, errors.New("CronJob 缺少有效 Job 模板")
	}
	job := &unstructured.Unstructured{Object: template}
	job.SetAPIVersion("batch/v1")
	job.SetKind("Job")
	job.SetName(request.Name)
	job.SetNamespace(request.Target.Namespace)
	// The owner UID keeps manually started Jobs discoverable from the CronJob.
	if err = unstructured.SetNestedSlice(job.Object, []interface{}{map[string]interface{}{"apiVersion": "batch/v1", "kind": "CronJob", "name": source.GetName(), "uid": string(source.GetUID()), "controller": true}}, "metadata", "ownerReferences"); err != nil {
		return manifest.Preview{}, err
	}
	data, err := json.Marshal(job.Object)
	if err != nil {
		return manifest.Preview{}, err
	}
	target := request.Target
	target.Resource = "jobs"
	target.Name = request.Name
	return s.writer.PreviewGuarded(ctx, manifest.Request{Target: target, Mode: "create", YAML: string(data)}, []manifest.Dependency{{Target: request.Target, Baseline: request.Baseline}})
}
