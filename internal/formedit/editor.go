// Package formedit shares the full-object, optimistic editing boundary used by resource forms.
package formedit

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Previewer interface {
	Preview(context.Context, manifest.Request) (manifest.Preview, error)
}

type Editor struct {
	resolver manifest.Resolver
	writer   Previewer
}

func New(resolver manifest.Resolver, writer Previewer) *Editor {
	return &Editor{resolver: resolver, writer: writer}
}

func (e *Editor) Read(ctx context.Context, target resource.Ref) (*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	client, err := e.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return nil, err
	}
	return client.GetObject(ctx, target)
}

// Preview loads and copies the complete object before invoking a resource-specific
// edit. ManifestService independently rechecks the baseline and performs dry run.
func (e *Editor) Preview(ctx context.Context, target resource.Ref, baseline manifest.Baseline, mode, kind string, edit func(*unstructured.Unstructured) error) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": kind,
		"metadata": map[string]interface{}{"name": target.Name, "namespace": target.Namespace},
	}}
	switch mode {
	case "create":
	case "patch":
		current, err := e.Read(ctx, target)
		if err != nil {
			return manifest.Preview{}, err
		}
		if baseline.UID == "" || baseline.ResourceVersion == "" || Baseline(current) != baseline {
			return manifest.Preview{}, errors.New("资源已变化或被重新创建，请重新读取基线后比较差异；当前草稿已保留")
		}
		object = current.DeepCopy()
	default:
		return manifest.Preview{}, errors.New("表单仅支持创建或差量更新")
	}
	if err := edit(object); err != nil {
		return manifest.Preview{}, err
	}
	data, err := json.Marshal(object.Object)
	if err != nil {
		return manifest.Preview{}, errors.New("资源不能序列化")
	}
	return e.writer.Preview(ctx, manifest.Request{Target: target, Mode: mode, YAML: string(data), Baseline: baseline, RevealSecret: target.Group == "" && target.Resource == "secrets"})
}

func Baseline(object *unstructured.Unstructured) manifest.Baseline {
	return manifest.Baseline{UID: string(object.GetUID()), ResourceVersion: object.GetResourceVersion()}
}

// Target rejects form input that would silently redirect an existing editing session.
func Target(ref resource.Ref, resourceName, name, namespace string) (resource.Ref, error) {
	if ref.ClusterID == "" || ref.Group != "" || ref.Version != "v1" || ref.Resource != resourceName {
		return resource.Ref{}, errors.New("资源类型与表单不匹配")
	}
	if name == "" || namespace == "" || (ref.Name != "" && ref.Name != name) || (ref.Namespace != "" && ref.Namespace != namespace) {
		return resource.Ref{}, errors.New("名称和命名空间不能为空，且必须与当前操作范围一致")
	}
	ref.Name, ref.Namespace = name, namespace
	return ref, nil
}
