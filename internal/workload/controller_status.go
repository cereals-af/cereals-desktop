package workload

import (
	"encoding/json"

	"github.com/cereals-af/cereals-desktop/internal/workloadstate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func controllerStatus(object *unstructured.Unstructured, kind string) ControllerStatus {
	status := ControllerStatus{
		Observed:        numberField(object, "status", "observedGeneration") >= object.GetGeneration(),
		Desired:         numberField(object, "spec", "replicas"),
		Current:         numberField(object, "status", "replicas"),
		Ready:           numberField(object, "status", "readyReplicas"),
		Updated:         numberField(object, "status", "updatedReplicas"),
		Available:       numberField(object, "status", "availableReplicas"),
		CurrentRevision: stringField(object, "status", "currentRevision"),
		UpdateRevision:  stringField(object, "status", "updateRevision"),
		Conditions:      []Condition{},
	}

	if _, found, _ := unstructured.NestedInt64(object.Object, "spec", "replicas"); !found {
		status.Desired = 1
	}
	if kind == "daemonsets" {
		status.Desired = numberField(object, "status", "desiredNumberScheduled")
		status.Current = numberField(object, "status", "currentNumberScheduled")
		status.Ready = numberField(object, "status", "numberReady")
		status.Updated = numberField(object, "status", "updatedNumberScheduled")
		status.Available = numberField(object, "status", "numberAvailable")
		status.Unavailable = numberField(object, "status", "numberUnavailable")
		status.Misscheduled = numberField(object, "status", "numberMisscheduled")
	} else {
		status.Unavailable = max(0, status.Desired-status.Available)
	}
	conditions, _, _ := unstructured.NestedSlice(object.Object, "status", "conditions")
	for _, raw := range conditions {
		if c, ok := raw.(map[string]interface{}); ok {
			u := &unstructured.Unstructured{Object: c}
			status.Conditions = append(status.Conditions, Condition{Type: stringField(u, "type"), Status: stringField(u, "status"), Reason: stringField(u, "reason"), Message: stringField(u, "message")})
		}
	}
	scheduling := map[string]interface{}{}
	for _, key := range []string{"nodeSelector", "affinity", "tolerations", "topologySpreadConstraints", "nodeName", "schedulerName"} {
		value, found, _ := unstructured.NestedFieldCopy(object.Object, "spec", "template", "spec", key)
		if found {
			scheduling[key] = value
		}
	}
	data, _ := json.MarshalIndent(scheduling, "", "  ")
	status.Scheduling = string(data)
	status.Phase = "处理中"
	switch {
	case object.GetDeletionTimestamp() != nil:
		status.Phase = "删除中"
	case !status.Observed:
		status.Phase = "等待控制器处理"
	case status.Ready == status.Desired && status.Current == status.Desired && (kind == "replicasets" || status.Updated == status.Desired):
		status.Phase = "已就绪"
	}
	if status.Observed && status.Updated < status.Desired && kind != "replicasets" && object.GetDeletionTimestamp() == nil {
		if stringField(object, "spec", "updateStrategy", "type") == "OnDelete" {
			status.Phase = "等待手动替换"
		} else if kind == "statefulsets" && numberField(object, "spec", "updateStrategy", "rollingUpdate", "partition") > 0 {
			status.Phase = "分区更新"
		}
	}
	status.State, status.Detail, _ = workloadstate.Classify(*object)
	return status
}
