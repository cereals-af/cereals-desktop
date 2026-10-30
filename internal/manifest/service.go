package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Service struct {
	resolver Resolver
	mu       sync.Mutex
	tickets  map[string]ticket
	closed   bool
}

func NewService(resolver Resolver) *Service {
	return &Service{resolver: resolver, tickets: make(map[string]ticket)}
}

func (s *Service) Snapshot(ctx context.Context, target resource.Ref, revealSecret bool) (Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return Snapshot{}, err
	}
	object, err := client.GetObject(ctx, target)
	if err != nil {
		return Snapshot{}, err
	}
	clean := editableObject(object)
	editable := !isSecret(target) || revealSecret
	if !editable {
		redact(clean)
	}
	text, err := render(clean)
	return Snapshot{YAML: text, Baseline: baseline(object), Editable: editable}, err
}

func (s *Service) Preview(ctx context.Context, request Request) (Preview, error) {
	return s.PreviewGuarded(ctx, request, nil)
}

func prepare(ctx context.Context, client Client, request Request) (Operation, string, string, error) {
	if !slices.Contains([]string{"create", "patch", "apply", "jsonpatch"}, request.Mode) {
		return Operation{}, "", "", errors.New("请选择创建、差量更新或声明式应用")
	}
	if isSecret(request.Target) && !request.RevealSecret {
		return Operation{}, "", "", errors.New("请先明确显示 Secret 内容，再进行编辑或创建")
	}
	object, err := parse(request.YAML)
	if err != nil {
		return Operation{}, "", "", err
	}
	target, err := validateTarget(ctx, client, request.Target, object, request.Mode)
	if err != nil {
		return Operation{}, "", "", err
	}
	clean := editableObject(object)
	desired, err := render(clean)
	if err != nil {
		return Operation{}, "", "", err
	}
	var data interface{} = clean.Object
	before := ""
	if request.Mode != "create" {
		current, err := client.GetObject(ctx, target)
		if err != nil {
			return Operation{}, "", "", err
		}
		if err := checkBaseline(current, request.Baseline); err != nil {
			return Operation{}, "", "", err
		}
		base := editableObject(current)
		before, err = render(base)
		if err != nil {
			return Operation{}, "", "", err
		}
		if request.Mode == "jsonpatch" {
			data = exactPatch(base.Object, clean.Object, request.Baseline)
		} else {
			fields := clean.Object
			if request.Mode == "patch" {
				fields = mergeDiff(base.Object, clean.Object)
			}
			if err := unstructured.SetNestedField(fields, request.Baseline.UID, "metadata", "uid"); err != nil {
				return Operation{}, "", "", err
			}
			if err := unstructured.SetNestedField(fields, request.Baseline.ResourceVersion, "metadata", "resourceVersion"); err != nil {
				return Operation{}, "", "", err
			}
			data = fields
		}
	}
	encoded, err := json.Marshal(data)
	return Operation{Target: target, Mode: request.Mode, Data: encoded}, before, desired, err
}

func validateTarget(ctx context.Context, client Client, target resource.Ref, object *unstructured.Unstructured, mode string) (resource.Ref, error) {
	typeInfo, err := client.Describe(ctx, target)
	if err != nil {
		return target, err
	}
	version := target.Version
	if target.Group != "" {
		version = target.Group + "/" + version
	}
	if object.GetAPIVersion() != version || object.GetKind() != typeInfo.Kind {
		return target, errors.New("apiVersion / kind 必须与当前选择的资源类型一致")
	}
	if object.GetName() == "" || (target.Name != "" && target.Name != object.GetName()) {
		return target, errors.New("metadata.name 不能为空，且不能更改已有资源名称")
	}
	if mode != "create" && target.Name == "" {
		return target, errors.New("更新必须指定已有资源")
	}
	if typeInfo.Namespaced && object.GetNamespace() == "" {
		return target, errors.New("请在 metadata.namespace 中明确指定命名空间")
	}
	if (!typeInfo.Namespaced && object.GetNamespace() != "") || (target.Namespace != "" && target.Namespace != object.GetNamespace()) {
		return target, errors.New("metadata.namespace 与操作范围不一致")
	}
	target.Name, target.Namespace = object.GetName(), object.GetNamespace()
	return target, nil
}
