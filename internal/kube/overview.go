package kube

import (
	"context"
	"errors"

	"github.com/cereals-af/cereals-desktop/internal/overview"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ overview.Client = (*Client)(nil)

func (c *Client) CountResources(ctx context.Context, kind resource.Type) (int64, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	api := c.metadata.Resource(schema.GroupVersionResource{Group: kind.Group, Version: kind.Version, Resource: kind.Resource})
	options := metav1.ListOptions{Limit: 500}
	var count int64
	for {
		page, err := api.List(ctx, options)
		if err != nil {
			return 0, requestError(err)
		}
		count += int64(len(page.Items))
		if page.Continue == "" {
			return count, nil
		}
		// remainingItemCount belongs to this list snapshot and avoids reading every
		// object's metadata on API servers that provide an exact remaining count.
		if page.RemainingItemCount != nil && *page.RemainingItemCount >= 0 {
			return count + *page.RemainingItemCount, nil
		}
		if page.Continue == options.Continue {
			return 0, errors.New("资源统计分页未推进，请刷新重试")
		}
		options.Continue = page.Continue
	}
}

func (c *Client) OverviewNodes(ctx context.Context) (overview.Nodes, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	result := overview.Nodes{}
	capacity, allocatable := corev1.ResourceList{}, corev1.ResourceList{}
	options := metav1.ListOptions{Limit: 500}
	for {
		page, err := c.core.Nodes().List(ctx, options)
		if err != nil {
			// Never return an incomplete total as the full cluster capacity.
			return overview.Nodes{}, requestError(err)
		}
		for _, node := range page.Items {
			result.Count++
			if node.Spec.Unschedulable {
				result.Unschedulable++
			}
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					result.Ready++
					break
				}
			}
			addResources(capacity, node.Status.Capacity)
			addResources(allocatable, node.Status.Allocatable)
		}
		if page.Continue == "" {
			result.Capacity, result.Allocatable = quantities(capacity), quantities(allocatable)
			return result, nil
		}
		if page.Continue == options.Continue {
			return overview.Nodes{}, errors.New("节点统计分页未推进，请刷新重试")
		}
		options.Continue = page.Continue
	}
}

func addResources(total, resources corev1.ResourceList) {
	for name, value := range resources {
		quantity := total[name]
		quantity.Add(value)
		total[name] = quantity
	}
}

func quantities(resources corev1.ResourceList) map[string]string {
	values := make(map[string]string, len(resources))
	for name, value := range resources {
		if name == corev1.ResourceCPU {
			values[string(name)] = value.AsDec().String()
		} else {
			values[string(name)] = value.String()
		}
	}
	return values
}
