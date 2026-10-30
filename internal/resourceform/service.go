package resourceform

import (
	"context"
	"errors"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workloadstate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

type Service struct {
	resolver manifest.Resolver
	editor   *formedit.Editor
	writer   writer
}

func NewService(resolver manifest.Resolver, writer writer) *Service {
	return &Service{resolver: resolver, editor: formedit.New(resolver, writer), writer: writer}
}

func kind(target resource.Ref) string {
	if target.Version != "v1" {
		return ""
	}
	switch target.Group {
	case "batch":
		return map[string]string{"jobs": "Job", "cronjobs": "CronJob"}[target.Resource]
	case "networking.k8s.io":
		return map[string]string{"ingresses": "Ingress", "networkpolicies": "NetworkPolicy"}[target.Resource]
	case "":
		if target.Resource == "secrets" {
			return "Secret"
		}
	}
	return ""
}

func validateTarget(target resource.Ref, existing bool) error {
	if target.ClusterID == "" || kind(target) == "" || target.Namespace == "" || (existing && target.Name == "") {
		return errors.New("请选择明确集群和命名空间中的受支持资源")
	}
	return nil
}

func (s *Service) Snapshot(ctx context.Context, target resource.Ref, reveal bool) (Snapshot, error) {
	if err := validateTarget(target, true); err != nil {
		return Snapshot{}, err
	}
	object, err := s.editor.Read(ctx, target)
	if err != nil {
		return Snapshot{}, err
	}
	result, err := snapshot(object, target, reveal)
	if err == nil && target.Group == "batch" {
		client, resolveErr := s.resolver.ManifestClient(target.ClusterID)
		if resolveErr != nil {
			return Snapshot{}, resolveErr
		}
		if observer, ok := client.(workloadstate.Observer); ok {
			result.State, result.Detail = observer.WorkloadState(ctx, *object)
		}
	}
	return result, err
}

func snapshot(object *unstructured.Unstructured, target resource.Ref, reveal bool) (Snapshot, error) {
	projection := map[string]interface{}{"name": object.GetName(), "namespace": object.GetNamespace()}
	for _, key := range []string{"labels", "annotations"} {
		if key == "annotations" && kind(target) == "Secret" && !reveal {
			continue
		}
		if value, found, _ := unstructured.NestedFieldCopy(object.Object, "metadata", key); found {
			projection[key] = value
		}
	}
	keys := []string{"spec"}
	if kind(target) == "Secret" {
		keys = []string{"type", "immutable"}
	}
	for _, key := range keys {
		if value, found := object.Object[key]; found {
			projection[key] = value
		}
	}
	value, err := formvalue.Encode(projection)
	if err != nil {
		return Snapshot{}, err
	}
	status, err := formvalue.Encode(object.Object["status"])
	if err != nil {
		return Snapshot{}, err
	}
	result := Snapshot{Baseline: formedit.Baseline(object), Value: value, Status: status, Entries: []SecretEntry{}, Revealed: reveal}
	result.State, result.Detail, _ = workloadstate.Classify(*object)
	if kind(target) == "Secret" {
		result.Entries, err = secretEntries(object, reveal)
	}
	return result, err
}

func (s *Service) Preview(ctx context.Context, request Request) (manifest.Preview, error) {
	decoded, err := formvalue.Decode(request.Value)
	if err != nil {
		return manifest.Preview{}, err
	}
	values, ok := decoded.(map[string]interface{})
	if !ok {
		return manifest.Preview{}, errors.New("表单必须是对象")
	}
	target := request.Target
	name, _ := values["name"].(string)
	namespace, _ := values["namespace"].(string)
	if len(validation.IsDNS1123Subdomain(name)) != 0 || namespace == "" || (target.Name != "" && target.Name != name) || target.Namespace != namespace {
		return manifest.Preview{}, errors.New("名称无效或表单操作范围发生变化")
	}
	target.Name = name
	if err := validateTarget(target, true); err != nil {
		return manifest.Preview{}, err
	}
	return s.editor.Preview(ctx, target, request.Baseline, request.Mode, kind(target), func(object *unstructured.Unstructured) error {
		if object.GetDeletionTimestamp() != nil {
			return errors.New("资源正在删除")
		}
		object.SetAPIVersion(target.Version)
		if target.Group != "" {
			object.SetAPIVersion(target.Group + "/" + target.Version)
		}
		for _, key := range []string{"labels", "annotations"} {
			if value, found := values[key]; found {
				if err := unstructured.SetNestedField(object.Object, value, "metadata", key); err != nil {
					return err
				}
			} else {
				unstructured.RemoveNestedField(object.Object, "metadata", key)
			}
		}
		if kind(target) == "Secret" {
			return editSecret(object, values, request)
		}
		spec, ok := values["spec"].(map[string]interface{})
		if !ok {
			return errors.New("请配置资源 spec")
		}
		object.Object["spec"] = spec
		return nil
	})
}
