package relations

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func summarize(object unstructured.Unstructured, target resource.Ref, relation string) Node {
	node := Node{Target: target, UID: string(object.GetUID()), Kind: object.GetKind(), Relation: relation}
	node.Status, _, _ = unstructured.NestedString(object.Object, "status", "phase")
	node.Detail, _, _ = unstructured.NestedString(object.Object, "spec", "nodeName")
	if object.GetKind() == "Pod" {
		node.Pod = summarizePod(object)
	}
	if object.GetKind() == "ReplicaSet" || object.GetKind() == "StatefulSet" || object.GetKind() == "Deployment" {
		desired, _, _ := unstructured.NestedInt64(object.Object, "spec", "replicas")
		ready, _, _ := unstructured.NestedInt64(object.Object, "status", "readyReplicas")
		node.Status = fmt.Sprintf("%d / %d 就绪", ready, desired)
		node.Revision = object.GetAnnotations()["deployment.kubernetes.io/revision"]
	}
	if object.GetKind() == "DaemonSet" {
		desired, _, _ := unstructured.NestedInt64(object.Object, "status", "desiredNumberScheduled")
		ready, _, _ := unstructured.NestedInt64(object.Object, "status", "numberReady")
		node.Status = fmt.Sprintf("%d / %d 就绪", ready, desired)
	}
	if object.GetKind() == "Job" {
		active, _, _ := unstructured.NestedInt64(object.Object, "status", "active")
		succeeded, _, _ := unstructured.NestedInt64(object.Object, "status", "succeeded")
		failed, _, _ := unstructured.NestedInt64(object.Object, "status", "failed")
		node.Status = fmt.Sprintf("%d 活跃 · %d 成功 · %d 失败", active, succeeded, failed)
	}
	if object.GetKind() == "EndpointSlice" {
		node.Status, node.Detail = endpointSummary(object)
	}
	return node
}

func endpointSummary(object unstructured.Unstructured) (string, string) {
	endpoints, _, _ := unstructured.NestedSlice(object.Object, "endpoints")
	ready, unknown := 0, 0
	addresses := []string{}
	for _, value := range endpoints {
		endpoint, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		isReady, found, _ := unstructured.NestedBool(endpoint, "conditions", "ready")
		if !found {
			unknown++
		} else if isReady {
			ready++
		}
		values, _, _ := unstructured.NestedStringSlice(endpoint, "addresses")
		for _, address := range values {
			if len(addresses) < 12 {
				addresses = append(addresses, address)
			}
		}
	}
	addressType, _, _ := unstructured.NestedString(object.Object, "addressType")
	detail := addressType + " · " + strings.Join(addresses, ", ")
	if len(endpoints) > 12 {
		detail += " …"
	}
	return fmt.Sprintf("%d / %d 就绪 · %d 未知", ready, len(endpoints), unknown), detail
}

func eventSummary(object unstructured.Unstructured) Event {
	field := func(path ...string) string {
		value, _, _ := unstructured.NestedString(object.Object, path...)
		return value
	}
	event := Event{UID: string(object.GetUID()), Type: field("type"), Reason: field("reason"), Message: field("message")}
	if len([]rune(event.Message)) > 2000 {
		event.Message = string([]rune(event.Message)[:2000]) + "…"
	}
	event.Count, _, _ = unstructured.NestedInt64(object.Object, "count")
	if count, found, _ := unstructured.NestedInt64(object.Object, "series", "count"); found {
		event.Count = count
	}
	for _, timestamp := range []string{field("series", "lastObservedTime"), field("lastTimestamp"), field("eventTime"), field("metadata", "creationTimestamp")} {
		if timestamp != "" {
			event.Time = timestamp
			break
		}
	}
	return event
}

func sortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool { return events[i].Time > events[j].Time })
}

func summarizePod(object unstructured.Unstructured) *PodSummary {
	result := &PodSummary{Containers: []string{}}
	containers, _, _ := unstructured.NestedSlice(object.Object, "spec", "containers")
	for _, raw := range containers {
		if c, ok := raw.(map[string]interface{}); ok {
			name, _, _ := unstructured.NestedString(c, "name")
			result.Containers = append(result.Containers, name)
		}
	}
	ready := 0
	for _, field := range []string{"containerStatuses", "initContainerStatuses", "ephemeralContainerStatuses"} {
		statuses, _, _ := unstructured.NestedSlice(object.Object, "status", field)
		for _, raw := range statuses {
			if c, ok := raw.(map[string]interface{}); ok {
				restarts, _, _ := unstructured.NestedInt64(c, "restartCount")
				result.Restarts += restarts
				if isReady, _, _ := unstructured.NestedBool(c, "ready"); isReady && field == "containerStatuses" {
					ready++
				}
			}
		}
	}
	result.Ready = fmt.Sprintf("%d / %d", ready, len(containers))
	return result
}
