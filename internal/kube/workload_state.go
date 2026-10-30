package kube

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workloadstate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// workloadChildren is shared by one list page, avoiding a Pod list per workload.
type workloadChildren struct {
	pods     map[types.UID][]unstructured.Unstructured
	replicas map[types.UID][]unstructured.Unstructured
	jobs     map[types.UID][]unstructured.Unstructured
}

func controllerObject(object unstructured.Unstructured) bool {
	_, _, ok := workloadstate.Classify(object)
	return ok && object.GetKind() != "Pod"
}

func (c *Client) workloadChildren(ctx context.Context, namespace, kind string) (workloadChildren, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	result := workloadChildren{}
	var err error
	result.pods, err = c.childrenByOwner(ctx, resource.Ref{Version: "v1", Resource: "pods", Namespace: namespace})
	if err != nil {
		return result, err
	}
	switch kind {
	case "Deployment":
		result.replicas, err = c.childrenByOwner(ctx, resource.Ref{Group: "apps", Version: "v1", Resource: "replicasets", Namespace: namespace})
	case "CronJob":
		result.jobs, err = c.childrenByOwner(ctx, resource.Ref{Group: "batch", Version: "v1", Resource: "jobs", Namespace: namespace})
	}
	return result, err
}

func (c *Client) childrenByOwner(ctx context.Context, target resource.Ref) (map[types.UID][]unstructured.Unstructured, error) {
	result := make(map[types.UID][]unstructured.Unstructured)
	options := metav1.ListOptions{Limit: 500}
	count := 0
	for {
		page, err := c.ListObjects(ctx, target, options)
		if err != nil {
			return nil, err
		}
		count += len(page.Items)
		if count > 10000 {
			return nil, errors.New("关联资源超过单次统计上限")
		}
		for _, item := range page.Items {
			for _, owner := range item.GetOwnerReferences() {
				if owner.Controller != nil && *owner.Controller && owner.UID != "" {
					result[owner.UID] = append(result[owner.UID], item)
					break
				}
			}
		}
		if page.GetContinue() == "" {
			return result, nil
		}
		if page.GetContinue() == options.Continue {
			return nil, errors.New("关联资源分页未推进")
		}
		options.Continue = page.GetContinue()
	}
}

func (children workloadChildren) state(object unstructured.Unstructured) (string, string) {
	state, detail, _ := workloadstate.Classify(object)
	if state == workloadstate.Error || (state == workloadstate.Completed && object.GetKind() != "CronJob") || state == workloadstate.Stopped || object.GetDeletionTimestamp() != nil {
		return state, detail
	}
	pods := children.pods[object.GetUID()]
	if object.GetKind() == "Deployment" {
		for _, rs := range children.replicas[object.GetUID()] {
			pods = append(pods, children.pods[rs.GetUID()]...)
		}
	}
	if object.GetKind() == "CronJob" {
		return children.cronState(object, state, detail)
	}
	ready := 0
	for _, pod := range pods {
		// Failed Job Pods can be retained while the controller retries. The
		// Job's terminal conditions decide failure; current container faults
		// (for example ImagePullBackOff) still surface immediately.
		if object.GetKind() == "Job" {
			phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
			if phase == "Failed" {
				continue
			}
		}
		podState, podDetail, _ := workloadstate.Classify(pod)
		if podState == workloadstate.Error {
			return workloadstate.Error, detail + " · Pod " + pod.GetName() + " · " + podDetail
		}
		if podState == workloadstate.Ready {
			ready++
		}
	}
	detail += fmt.Sprintf(" · 关联 Pod 就绪 %d / %d", ready, len(pods))
	if ready > 0 {
		return workloadstate.Ready, detail
	}
	return workloadstate.Processing, detail
}

func (c *Client) WorkloadState(ctx context.Context, object unstructured.Unstructured) (string, string) {
	state, detail, _ := workloadstate.Classify(object)
	if !controllerObject(object) || object.GetUID() == "" {
		return state, detail
	}
	children, err := c.workloadChildren(ctx, object.GetNamespace(), object.GetKind())
	if err != nil {
		return state, detail + " · 关联状态未验证：" + err.Error()
	}
	return children.state(object)
}

func (c *Client) enrichWorkloadPage(ctx context.Context, namespace string, objects []unstructured.Unstructured, items []resource.Item) {
	if len(objects) == 0 || !controllerObject(objects[0]) {
		return
	}
	children, err := c.workloadChildren(ctx, namespace, objects[0].GetKind())
	for i, object := range objects {
		if err != nil {
			items[i].Detail += " · 关联状态未验证：" + err.Error()
			continue
		}
		items[i].Status, items[i].Detail = children.state(object)
	}
}

func (children workloadChildren) cronState(object unstructured.Unstructured, state, detail string) (string, string) {
	jobs := children.jobs[object.GetUID()]
	if len(jobs) == 0 {
		return state, detail
	}
	active, _, _ := unstructured.NestedSlice(object.Object, "status", "active")
	activeIDs := make(map[types.UID]bool)
	for _, raw := range active {
		if item, ok := raw.(map[string]interface{}); ok {
			if uid, ok := item["uid"].(string); ok {
				activeIDs[types.UID(uid)] = true
			}
		}
	}
	latest := jobs[0]
	ready := false
	for _, job := range jobs {
		if job.GetCreationTimestamp().Time.After(latest.GetCreationTimestamp().Time) {
			latest = job
		}
		if !activeIDs[job.GetUID()] {
			continue
		}
		jobState, jobDetail := children.state(job)
		if jobState == workloadstate.Error {
			return jobState, detail + " · Job " + job.GetName() + " · " + jobDetail
		}
		if jobState == workloadstate.Ready {
			ready = true
		}
	}
	if ready {
		return workloadstate.Ready, detail
	}
	if len(active) > 0 {
		return workloadstate.Processing, detail
	}
	jobState, jobDetail := children.state(latest)
	return jobState, detail + " · 最近 Job " + latest.GetName() + " · " + jobDetail
}
