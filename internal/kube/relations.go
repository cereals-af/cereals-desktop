package kube

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/relations"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ relations.Client = (*Client)(nil)

func (c *Client) ListObjects(ctx context.Context, target resource.Ref, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	result, err := c.objectAPI(target).List(ctx, options)
	if err != nil {
		return nil, requestError(err)
	}
	return result, nil
}
