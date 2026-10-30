package kube

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

func (c *Client) List(ctx context.Context, query resource.Query) (resource.Page, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	gvr := schema.GroupVersionResource{Group: query.Group, Version: query.Version, Resource: query.Resource}
	objects, err := c.dynamic.Resource(gvr).Namespace(query.Namespace).List(ctx, metav1.ListOptions{
		Limit: 100, Continue: query.Continue, LabelSelector: query.LabelSelector,
	})
	if err != nil {
		return resource.Page{}, requestError(err)
	}
	page := resource.Page{Items: make([]resource.Item, 0, len(objects.Items)), Continue: objects.GetContinue(), ResourceVersion: objects.GetResourceVersion()}
	for _, object := range objects.Items {
		page.Items = append(page.Items, summarize(object))
	}
	c.enrichWorkloadPage(ctx, query.Namespace, objects.Items, page.Items)
	return page, nil
}

func (c *Client) YAML(ctx context.Context, ref resource.Ref) (string, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	gvr := schema.GroupVersionResource{Group: ref.Group, Version: ref.Version, Resource: ref.Resource}
	object, err := c.dynamic.Resource(gvr).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return "", requestError(err)
	}
	object.SetManagedFields(nil)
	if ref.Group == "" && ref.Resource == "secrets" {
		redactSecret(object)
	}
	data, err := json.Marshal(object.Object)
	if err != nil {
		return "", requestError(err)
	}
	manifest, err := yaml.JSONToYAML(data)
	if err != nil {
		return "", requestError(err)
	}
	return string(manifest), nil
}

func redactSecret(object *unstructured.Unstructured) {
	for _, field := range []string{"data", "stringData"} {
		values, found, _ := unstructured.NestedMap(object.Object, field)
		if !found {
			continue
		}
		for key := range values {
			values[key] = "<已隐藏>"
		}
		_ = unstructured.SetNestedMap(object.Object, values, field)
	}
	// Last-applied annotations may contain a complete copy of a Secret.
	object.SetAnnotations(nil)
}

func summarize(object unstructured.Unstructured) resource.Item {
	status, detail := objectStatus(object)
	created := ""
	if timestamp := object.GetCreationTimestamp(); !timestamp.IsZero() {
		created = timestamp.Format(time.RFC3339)
	}
	labels := object.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	return resource.Item{Node: nodeSummary(object), UID: string(object.GetUID()), Name: object.GetName(), Namespace: object.GetNamespace(),
		CreatedAt: created, Status: status, Detail: detail, Labels: labels}
}
