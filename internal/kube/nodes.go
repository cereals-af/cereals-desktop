package kube

import (
	"context"
	"github.com/cereals-af/cereals-desktop/internal/node"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sort"
	"strings"
)

var _ node.Client = (*Client)(nil)

// ReadDrainPod distinguishes absence from permission or transport failures.
func (c *Client) ReadDrainPod(ctx context.Context, target resource.Ref) (*unstructured.Unstructured, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	object, err := c.objectAPI(target).Get(ctx, target.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, requestError(err)
	}
	return object, nil
}
func (c *Client) EvictPod(ctx context.Context, request node.Eviction) (node.EvictionResult, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	uid := types.UID(request.Baseline.UID)
	options := &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &request.Baseline.ResourceVersion}}
	if request.DryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}
	eviction := &policyv1.Eviction{TypeMeta: metav1.TypeMeta{APIVersion: "policy/v1", Kind: "Eviction"}, ObjectMeta: metav1.ObjectMeta{Name: request.Target.Name, Namespace: request.Target.Namespace}, DeleteOptions: options}
	err := c.core.Pods(request.Target.Namespace).EvictV1(ctx, eviction)
	if apierrors.IsTooManyRequests(err) {
		return node.EvictionResult{Waiting: true}, nil
	}
	if err != nil {
		return node.EvictionResult{}, requestError(err)
	}
	return node.EvictionResult{}, nil
}

func nodeSummary(object unstructured.Unstructured) *resource.NodeSummary {
	if object.GetKind() != "Node" || object.GetAPIVersion() != "v1" {
		return nil
	}
	result := &resource.NodeSummary{}
	roles := []string{}
	for key, value := range object.GetLabels() {
		if role, ok := strings.CutPrefix(key, "node-role.kubernetes.io/"); ok && role != "" {
			roles = append(roles, role)
		}
		if key == "kubernetes.io/role" && value != "" {
			roles = append(roles, value)
		}
	}
	sort.Strings(roles)
	result.Roles = strings.Join(roles, ", ")
	result.Unschedulable, _, _ = unstructured.NestedBool(object.Object, "spec", "unschedulable")
	result.KubeletVersion, _, _ = unstructured.NestedString(object.Object, "status", "nodeInfo", "kubeletVersion")
	result.CPU, _, _ = unstructured.NestedString(object.Object, "status", "capacity", "cpu")
	result.Memory, _, _ = unstructured.NestedString(object.Object, "status", "capacity", "memory")
	addresses, _, _ := unstructured.NestedSlice(object.Object, "status", "addresses")
	for _, raw := range addresses {
		if address, ok := raw.(map[string]interface{}); ok && address["type"] == "InternalIP" {
			result.InternalIP, _ = address["address"].(string)
			break
		}
	}
	return result
}
