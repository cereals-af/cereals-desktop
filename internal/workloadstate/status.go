// Package workloadstate classifies reported workload state without treating
// missing observations as success. Rollout progress remains a separate concern.
package workloadstate

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	Ready      = "Ready"
	Completed  = "Completed"
	Processing = "Processing"
	Stopped    = "Stopped"
	Error      = "Error"
)

func Classify(object unstructured.Unstructured) (state, detail string, supported bool) {
	group := object.GroupVersionKind().Group
	kind := object.GetKind()
	supported = (group == "" && kind == "Pod") || (group == "apps" && (kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" || kind == "ReplicaSet")) || (group == "batch" && (kind == "Job" || kind == "CronJob"))
	if !supported {
		return "", "", false
	}
	if object.GetDeletionTimestamp() != nil {
		return Processing, "资源正在删除", true
	}
	switch kind {
	case "Pod":
		state, detail = pod(object)
	case "Job":
		state, detail = job(object)
	case "CronJob":
		state, detail = cronJob(object)
	default:
		state, detail = replicas(object)
	}
	return state, detail, true
}

func number(object unstructured.Unstructured, path ...string) int64 {
	value, _, _ := unstructured.NestedInt64(object.Object, path...)
	return value
}

func text(object unstructured.Unstructured, path ...string) string {
	value, _, _ := unstructured.NestedString(object.Object, path...)
	return value
}

func flag(object unstructured.Unstructured, path ...string) bool {
	value, _, _ := unstructured.NestedBool(object.Object, path...)
	return value
}

func condition(object unstructured.Unstructured, kind, status string) (string, bool) {
	values, _, _ := unstructured.NestedSlice(object.Object, "status", "conditions")
	for _, raw := range values {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if item["type"] == kind && item["status"] == status {
			reason, _ := item["reason"].(string)
			if reason == "" {
				reason = kind
			}
			return reason, true
		}
	}
	return "", false
}

func replicas(object unstructured.Unstructured) (string, string) {
	desired, found, _ := unstructured.NestedInt64(object.Object, "spec", "replicas")
	if !found {
		desired = 1
	}
	current, ready := number(object, "status", "replicas"), number(object, "status", "readyReplicas")
	if object.GetKind() == "DaemonSet" {
		desired, found, _ = unstructured.NestedInt64(object.Object, "status", "desiredNumberScheduled")
		if !found {
			return Processing, "等待控制器上报节点调度状态"
		}
		current, ready = number(object, "status", "currentNumberScheduled"), number(object, "status", "numberReady")
	}
	detail := fmt.Sprintf("%d / %d 就绪 · 当前 %d", ready, desired, current)
	observed := number(object, "status", "observedGeneration") >= object.GetGeneration()
	if desired == 0 {
		if current == 0 && observed {
			return Stopped, detail
		}
		return Processing, detail + " · 正在停止"
	}
	if observed {
		if reason, ok := condition(object, "ReplicaFailure", "True"); ok {
			return Error, detail + " · " + reason
		}
		if reason, ok := condition(object, "Progressing", "False"); ok && reason == "ProgressDeadlineExceeded" {
			return Error, detail + " · " + reason
		}
	}
	if ready > 0 {
		return Ready, detail
	}
	return Processing, detail
}

func job(object unstructured.Unstructured) (string, string) {
	active, done := number(object, "status", "active"), number(object, "status", "succeeded")
	detail := fmt.Sprintf("运行 %d · 成功 %d · 失败 %d", active, done, number(object, "status", "failed"))
	for _, kind := range []string{"Failed", "FailureTarget"} {
		if reason, ok := condition(object, kind, "True"); ok {
			return Error, detail + " · " + reason
		}
	}
	if _, ok := condition(object, "Complete", "True"); ok || text(object, "status", "completionTime") != "" {
		return Completed, detail
	}
	if flag(object, "spec", "suspend") {
		if active == 0 {
			return Stopped, detail + " · 已暂停"
		}
		return Processing, detail + " · 正在暂停"
	}
	if number(object, "spec", "parallelism") == 0 && hasField(object, "spec", "parallelism") {
		if active == 0 {
			return Stopped, detail + " · 并行度为 0"
		}
		return Processing, detail + " · 并行度为 0，等待已有 Pod 结束"
	}
	if number(object, "status", "ready") > 0 {
		return Ready, detail
	}
	return Processing, detail
}

func cronJob(object unstructured.Unstructured) (string, string) {
	active, _, _ := unstructured.NestedSlice(object.Object, "status", "active")
	detail := fmt.Sprintf("计划 %s · 执行中 %d", text(object, "spec", "schedule"), len(active))
	if flag(object, "spec", "suspend") {
		if len(active) == 0 {
			return Stopped, detail + " · 已暂停"
		}
		return Processing, detail + " · 已暂停后续调度"
	}
	if len(active) > 0 {
		return Processing, detail
	}
	succeeded, successErr := time.Parse(time.RFC3339, text(object, "status", "lastSuccessfulTime"))
	scheduled, scheduleErr := time.Parse(time.RFC3339, text(object, "status", "lastScheduleTime"))
	if successErr == nil && (scheduleErr != nil || !succeeded.Before(scheduled)) {
		return Completed, detail + " · 最近任务已完成"
	}
	return Processing, detail + " · 等待调度或任务状态"
}

func pod(object unstructured.Unstructured) (string, string) {
	node := text(object, "spec", "nodeName")
	detail := func(reason string) string { return strings.Trim(strings.Join([]string{node, reason}, " · "), " · ") }
	phase := text(object, "status", "phase")
	switch phase {
	case "Succeeded":
		return Completed, detail("运行完成")
	case "Failed", "Unknown":
		return Error, detail(phase + " " + text(object, "status", "reason"))
	}
	for _, field := range []string{"initContainerStatuses", "containerStatuses"} {
		values, _, _ := unstructured.NestedSlice(object.Object, "status", field)
		for _, raw := range values {
			value, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			container := unstructured.Unstructured{Object: value}
			reason := text(container, "state", "waiting", "reason")
			if reason != "" && reason != "ContainerCreating" && reason != "PodInitializing" {
				return Error, detail(reason)
			}
			if number(container, "state", "terminated", "exitCode") != 0 {
				return Error, detail(text(container, "state", "terminated", "reason"))
			}
		}
	}
	if reason, ok := condition(object, "PodScheduled", "False"); ok && reason == "Unschedulable" {
		return Error, detail(reason)
	}
	if phase == "Running" {
		if _, ok := condition(object, "Ready", "True"); ok {
			return Ready, detail("Running")
		}
		statuses, _, _ := unstructured.NestedSlice(object.Object, "status", "containerStatuses")
		ready := 0
		for _, raw := range statuses {
			if item, ok := raw.(map[string]interface{}); ok && item["ready"] == true {
				ready++
			}
		}
		// Preserve explicit Pod readiness gates; all containers ready alone cannot override them.
		if _, notReady := condition(object, "Ready", "False"); !notReady && len(statuses) > 0 && ready == len(statuses) {
			return Ready, detail("Running")
		}
	}
	return Processing, detail(phase)
}

func hasField(object unstructured.Unstructured, path ...string) bool {
	_, found, _ := unstructured.NestedFieldNoCopy(object.Object, path...)
	return found
}
