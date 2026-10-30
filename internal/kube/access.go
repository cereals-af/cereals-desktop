package kube

import (
	"context"
	"encoding/json"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workload"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) ReviewAccess(ctx context.Context, target resource.Ref, verb, subresource string) (workload.Access, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	request := authorizationv1.SelfSubjectAccessReview{TypeMeta: metav1.TypeMeta{APIVersion: "authorization.k8s.io/v1", Kind: "SelfSubjectAccessReview"}, Spec: authorizationv1.SelfSubjectAccessReviewSpec{ResourceAttributes: &authorizationv1.ResourceAttributes{Group: target.Group, Version: target.Version, Resource: target.Resource, Namespace: target.Namespace, Name: target.Name, Subresource: subresource, Verb: verb}}}
	data, err := json.Marshal(request)
	if err != nil {
		return workload.Access{}, err
	}
	response, err := c.rest.Post().AbsPath("/apis/authorization.k8s.io/v1/selfsubjectaccessreviews").Body(data).Do(ctx).Raw()
	if err != nil {
		return workload.Access{}, requestError(err)
	}
	var review authorizationv1.SelfSubjectAccessReview
	if err = json.Unmarshal(response, &review); err != nil {
		return workload.Access{}, requestError(err)
	}
	result := workload.Access{Verb: verb, Subresource: subresource, State: "unknown"}
	switch {
	case review.Status.Allowed:
		result.State = "allowed"
	case review.Status.Denied || review.Status.EvaluationError == "":
		result.State = "denied"
	default:
		result.Message = "集群未能完整判定权限"
	}
	return result, nil
}
