package kube

import (
	"fmt"

	"github.com/cereals-af/cereals-desktop/internal/workloadstate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func objectStatus(object unstructured.Unstructured) (string, string) {
	if state, detail, ok := workloadstate.Classify(object); ok {
		return state, detail
	}
	if object.GetDeletionTimestamp() != nil {
		return "Terminating", "资源正在删除"
	}
	if status, detail, ok := storageStatus(object); ok {
		return status, detail
	}
	switch object.GetKind() {
	case "Service":
		kind, _, _ := unstructured.NestedString(object.Object, "spec", "type")
		ip, _, _ := unstructured.NestedString(object.Object, "spec", "clusterIP")
		return "—", kind + " · " + ip
	}
	if phase, ok, _ := unstructured.NestedString(object.Object, "status", "phase"); ok {
		return phase, ""
	}
	return conditionStatus(object)
}

func storageStatus(object unstructured.Unstructured) (string, string, bool) {
	field := func(path ...string) string {
		value, _, _ := unstructured.NestedString(object.Object, path...)
		return value
	}
	if object.GetAPIVersion() == "storage.k8s.io/v1" && object.GetKind() == "StorageClass" {
		return "—", field("provisioner"), true
	}
	if object.GetAPIVersion() != "v1" {
		return "", "", false
	}
	switch object.GetKind() {
	case "PersistentVolumeClaim":
		capacity := field("status", "capacity", "storage")
		if capacity == "" {
			capacity = "尚未报告"
		}
		return field("status", "phase"), fmt.Sprintf("申请 %s · 已报告 %s · %s", field("spec", "resources", "requests", "storage"), capacity, field("spec", "storageClassName")), true
	case "PersistentVolume":
		return field("status", "phase"), fmt.Sprintf("%s · %s · %s", field("spec", "capacity", "storage"), field("spec", "storageClassName"), field("spec", "persistentVolumeReclaimPolicy")), true
	}
	return "", "", false
}

func conditionStatus(object unstructured.Unstructured) (string, string) {
	conditions, _, _ := unstructured.NestedSlice(object.Object, "status", "conditions")
	for _, value := range conditions {
		condition, ok := value.(map[string]any)
		if !ok {
			continue
		}
		kind, _, _ := unstructured.NestedString(condition, "type")
		status, _, _ := unstructured.NestedString(condition, "status")
		if kind == "Ready" {
			if status == "True" {
				return "Ready", ""
			}
			return "NotReady", ""
		}
	}
	return "—", ""
}
