package kube

import (
	"context"
	"errors"

	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

var _ manifest.Client = (*Client)(nil)

func (c *Client) objectAPI(ref resource.Ref) dynamic.ResourceInterface {
	gvr := schema.GroupVersionResource{Group: ref.Group, Version: ref.Version, Resource: ref.Resource}
	return c.dynamic.Resource(gvr).Namespace(ref.Namespace)
}

func (c *Client) Describe(ctx context.Context, ref resource.Ref) (resource.Type, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	version := ref.Version
	if ref.Group != "" {
		version = ref.Group + "/" + version
	}
	items, err := c.discoverVersion(ctx, version)
	if err != nil {
		return resource.Type{}, requestError(err)
	}
	for _, item := range items {
		if item.Resource == ref.Resource {
			return item, nil
		}
	}
	return resource.Type{}, errors.New("当前集群未发现指定资源类型，请重新发现 API")
}

func (c *Client) GetObject(ctx context.Context, ref resource.Ref) (*unstructured.Unstructured, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	object, err := c.objectAPI(ref).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, requestError(err)
	}
	return object, nil
}

func (c *Client) WriteObject(ctx context.Context, operation manifest.Operation, dryRun bool) (*unstructured.Unstructured, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	var dry []string
	if dryRun {
		dry = []string{metav1.DryRunAll}
	}
	api := c.objectAPI(operation.Target)
	var subresources []string
	if operation.Subresource != "" {
		if operation.Subresource != "scale" || (operation.Mode != "update" && operation.Mode != "patch") {
			return nil, errors.New("不支持的子资源写入")
		}
		subresources = []string{"scale"}
	}
	var result *unstructured.Unstructured
	var err error
	switch operation.Mode {
	case "update":
		if operation.Subresource != "scale" {
			return nil, errors.New("完整更新仅用于 Scale")
		}
		object := &unstructured.Unstructured{}
		if err := object.UnmarshalJSON(operation.Data); err != nil {
			return nil, errors.New("Scale 更新内容无效")
		}
		result, err = api.Update(ctx, object, metav1.UpdateOptions{DryRun: dry, FieldManager: "creals", FieldValidation: "Strict"}, subresources...)

	case "create":
		object := &unstructured.Unstructured{}
		if err := object.UnmarshalJSON(operation.Data); err != nil {
			return nil, errors.New("创建内容不是有效资源")
		}
		result, err = api.Create(ctx, object, metav1.CreateOptions{DryRun: dry, FieldManager: "creals", FieldValidation: "Strict"})
	case "patch", "apply", "jsonpatch":
		patchType := types.MergePatchType
		options := metav1.PatchOptions{DryRun: dry, FieldManager: "creals", FieldValidation: "Strict"}
		if operation.Mode == "jsonpatch" {
			patchType = types.JSONPatchType
		}
		if operation.Mode == "apply" {
			patchType = types.ApplyPatchType
			force := false
			options.Force = &force
		}
		result, err = api.Patch(ctx, operation.Target.Name, patchType, operation.Data, options, subresources...)
	default:
		return nil, errors.New("不支持的写入方式")
	}
	if err != nil {
		return nil, requestError(err)
	}
	return result, nil
}

func (c *Client) DeleteObject(ctx context.Context, request manifest.DeleteRequest, dryRun bool) error {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	uid := types.UID(request.Baseline.UID)
	policy := metav1.DeletionPropagation(request.Propagation)
	options := metav1.DeleteOptions{PropagationPolicy: &policy, Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &request.Baseline.ResourceVersion}}
	if request.UseGrace {
		options.GracePeriodSeconds = &request.GraceSeconds
	}
	if dryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}
	if err := c.objectAPI(request.Target).Delete(ctx, request.Target.Name, options); err != nil {
		return requestError(err)
	}
	return nil
}

func (c *Client) GetScale(ctx context.Context, target resource.Ref) (*unstructured.Unstructured, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	object, err := c.objectAPI(target).Get(ctx, target.Name, metav1.GetOptions{}, "scale")
	if err != nil {
		return nil, requestError(err)
	}
	return object, nil
}
