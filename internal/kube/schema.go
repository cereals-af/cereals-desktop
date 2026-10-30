package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

var _ schema.Client = (*Client)(nil)

func (c *Client) ResourceSchema(ctx context.Context, target resource.Ref) (schema.Document, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	kind, err := c.Describe(ctx, target)
	if err != nil {
		return schema.Document{}, err
	}
	root, definitions, err := c.openAPISchema(ctx, kind)
	source := "OpenAPI v3"
	if err != nil {
		if ctx.Err() != nil {
			return schema.Document{}, requestError(ctx.Err())
		}
		root, err = c.crdSchema(ctx, kind)
		if err != nil {
			return schema.Document{}, errors.New("无法读取此资源的 OpenAPI v3 或 CRD Schema，请检查 API 支持和读取权限；仍可使用服务端预检")
		}
		definitions, source = nil, "CRD OpenAPI v3"
	}
	value, err := schema.EditorSchema(root, definitions)
	if err != nil {
		return schema.Document{}, err
	}
	version := kind.Version
	if kind.Group != "" {
		version = kind.Group + "/" + version
	}
	return schema.Document{APIVersion: version, Kind: kind.Kind, JSON: value, Source: source,
		Warnings: []string{"Schema 提示仅作编辑辅助；CEL、准入规则、默认值和权限以服务端预检为准。"}}, nil
}

func (c *Client) openAPISchema(ctx context.Context, kind resource.Type) (map[string]interface{}, map[string]interface{}, error) {
	path := "/openapi/v3/api/" + kind.Version
	if kind.Group != "" {
		path = "/openapi/v3/apis/" + kind.Group + "/" + kind.Version
	}
	// A hashless URL always resolves to the current group/version document.
	stream, err := c.rest.Get().AbsPath(path).SetHeader("Accept", "application/json").Stream(ctx)
	if err != nil {
		return nil, nil, requestError(err)
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, (16<<20)+1))
	if err != nil || len(data) > 16<<20 {
		return nil, nil, errors.New("OpenAPI 文档读取失败或超过 16 MiB")
	}
	var document struct {
		Components struct {
			Schemas map[string]interface{} `json:"schemas"`
		} `json:"components"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, nil, errors.New("OpenAPI 文档格式无效")
	}
	for _, candidate := range document.Components.Schemas {
		object, ok := candidate.(map[string]interface{})
		if ok && matchesKind(object, kind) {
			return object, document.Components.Schemas, nil
		}
	}
	return nil, nil, errors.New("OpenAPI 未包含此资源版本")
}

func matchesKind(object map[string]interface{}, kind resource.Type) bool {
	gvks, _ := object["x-kubernetes-group-version-kind"].([]interface{})
	for _, value := range gvks {
		gvk, ok := value.(map[string]interface{})
		if ok && gvk["group"] == kind.Group && gvk["version"] == kind.Version && gvk["kind"] == kind.Kind {
			return true
		}
	}
	return false
}

func (c *Client) crdSchema(ctx context.Context, kind resource.Type) (map[string]interface{}, error) {
	if kind.Group == "" {
		return nil, errors.New("内置资源没有 CRD Schema")
	}
	gvr := kubeschema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	object, err := c.dynamic.Resource(gvr).Get(ctx, kind.Resource+"."+kind.Group, metav1.GetOptions{})
	if err != nil {
		return nil, requestError(err)
	}
	versions, _, _ := unstructured.NestedSlice(object.Object, "spec", "versions")
	for _, value := range versions {
		version, ok := value.(map[string]interface{})
		if !ok || version["name"] != kind.Version || version["served"] != true {
			continue
		}
		root, found, err := unstructured.NestedMap(version, "schema", "openAPIV3Schema")
		if err == nil && found {
			// CRD schemas omit fields supplied by the API server's object envelope.
			properties, _, _ := unstructured.NestedMap(root, "properties")
			if properties == nil {
				properties = map[string]interface{}{}
			}
			properties["apiVersion"] = map[string]interface{}{"type": "string"}
			properties["kind"] = map[string]interface{}{"type": "string"}
			properties["metadata"] = map[string]interface{}{"type": "object", "additionalProperties": true}
			root["properties"] = properties
			return root, nil
		}
	}
	return nil, errors.New("CRD 没有此 served 版本的结构化 Schema")
}
