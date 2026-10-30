package customresource

import (
	"context"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Service struct {
	resolver manifest.Resolver
	writer   Previewer
}

func NewService(resolver manifest.Resolver, writer Previewer) *Service {
	return &Service{resolver: resolver, writer: writer}
}
func (s *Service) Snapshot(ctx context.Context, target resource.Ref) (Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return Snapshot{}, err
	}
	definition, _, err := readDefinition(ctx, client, target)
	if err != nil {
		return Snapshot{}, err
	}
	root, err := versionSchema(definition, target)
	if err != nil {
		return Snapshot{}, err
	}
	info := summarizeDefinition(definition)
	if info.Namespaced && target.Name != "" && target.Namespace == "" {
		return Snapshot{}, errors.New("请明确实例命名空间")
	}
	object := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": target.Group + "/" + target.Version, "kind": info.Kind, "metadata": map[string]interface{}{"name": target.Name}}}
	if info.Namespaced {
		object.SetNamespace(target.Namespace)
	}
	base := manifest.Baseline{}
	if target.Name != "" {
		object, err = client.GetObject(ctx, target)
		if err != nil {
			return Snapshot{}, err
		}
		base = formedit.Baseline(object)
	}
	// Use the same editable envelope as YAML; status and managed fields never become form state.
	draft, err := makeDraft(object)
	return Snapshot{Baseline: base, Definition: formedit.Baseline(definition), Fields: fields(root), Draft: draft}, err
}
func (s *Service) Preview(ctx context.Context, request Request) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if request.Mode != "create" && request.Mode != "update" {
		return manifest.Preview{}, errors.New("请选择创建或更新")
	}
	client, err := s.resolver.ManifestClient(request.Target.ClusterID)
	if err != nil {
		return manifest.Preview{}, err
	}
	definition, ref, err := readDefinition(ctx, client, request.Target)
	if err != nil {
		return manifest.Preview{}, err
	}
	if request.Definition.UID == "" || request.Definition.ResourceVersion == "" || formedit.Baseline(definition) != request.Definition {
		return manifest.Preview{}, errors.New("CRD 定义已变化，请保留 YAML 草稿，刷新表单后重新检查")
	}
	if _, err = versionSchema(definition, request.Target); err != nil {
		return manifest.Preview{}, err
	}
	object, err := draftObject(request.Value, request.YAML)
	if err != nil {
		return manifest.Preview{}, err
	}
	text, err := manifest.RenderDraft(object)
	if err != nil {
		return manifest.Preview{}, err
	}
	mode := "jsonpatch"
	if request.Mode == "create" {
		mode = "create"
	}
	return s.writer.PreviewGuarded(ctx, manifest.Request{Target: request.Target, Baseline: request.Baseline, Mode: mode, YAML: text}, []manifest.Dependency{{Target: ref, Baseline: request.Definition}})
}
func (s *Service) List(ctx context.Context, query resource.Query) (Page, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	client, err := s.resolver.ManifestClient(query.ClusterID)
	if err != nil {
		return Page{}, err
	}
	target := resource.Ref{ClusterID: query.ClusterID, Group: query.Group, Version: query.Version, Resource: query.Resource, Namespace: query.Namespace}
	definition, _, err := readDefinition(ctx, client, target)
	if err != nil {
		return Page{}, err
	}
	if _, err = versionSchema(definition, target); err != nil {
		return Page{}, err
	}
	reader, ok := client.(tableReader)
	if !ok {
		return Page{}, errors.New("此连接不支持服务端 Table；请使用通用资源列表")
	}
	return reader.CustomTable(ctx, query)
}
