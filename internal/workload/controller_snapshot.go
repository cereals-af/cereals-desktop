package workload

import (
	"fmt"
	"sort"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func controllerSnapshot(object *unstructured.Unstructured, target resource.Ref) (ControllerSnapshot, error) {
	var template corev1.PodTemplateSpec
	raw, _, err := unstructured.NestedMap(object.Object, "spec", "template")
	if err != nil {
		return ControllerSnapshot{}, err
	}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &template); err != nil {
		return ControllerSnapshot{}, fmt.Errorf("读取 Pod 模板: %w", err)
	}
	form := ControllerForm{Workload: Deployment{Name: object.GetName(), Namespace: object.GetNamespace(), Replicas: 1, Containers: []Container{}}, NodeSelector: []Label{}, Claims: []ClaimTemplate{}, RetainDeleted: "Retain", RetainScaled: "Retain"}
	form.Workload.PodOptions, err = podOptions(object)
	if err != nil {
		return ControllerSnapshot{}, err
	}
	for _, container := range template.Spec.Containers {
		form.Workload.Containers = append(form.Workload.Containers, containerForm(container))
	}
	if err := attachContainerOptions(object, form.Workload.Containers); err != nil {
		return ControllerSnapshot{}, err
	}
	if replicas, found, _ := unstructured.NestedInt64(object.Object, "spec", "replicas"); found {
		form.Workload.Replicas = int32(replicas)
	}
	for key, value := range template.Spec.NodeSelector {
		form.NodeSelector = append(form.NodeSelector, Label{key, value})
	}
	sort.Slice(form.NodeSelector, func(i, j int) bool { return form.NodeSelector[i].Key < form.NodeSelector[j].Key })
	form.Strategy = stringField(object, "spec", "updateStrategy", "type")
	if form.Strategy == "" && target.Resource != "replicasets" {
		form.Strategy = "RollingUpdate"
	}
	form.Partition = int32(numberField(object, "spec", "updateStrategy", "rollingUpdate", "partition"))
	form.MaxUnavailable = intOrString(object, "maxUnavailable", "1")
	form.MaxSurge = intOrString(object, "maxSurge", "0")
	form.ServiceName = stringField(object, "spec", "serviceName")
	form.PodManagementPolicy = stringField(object, "spec", "podManagementPolicy")
	if form.PodManagementPolicy == "" {
		form.PodManagementPolicy = "OrderedReady"
	}
	for _, field := range []struct {
		key  string
		dest *string
	}{{"whenDeleted", &form.RetainDeleted}, {"whenScaled", &form.RetainScaled}} {
		if value := stringField(object, "spec", "persistentVolumeClaimRetentionPolicy", field.key); value != "" {
			*field.dest = value
		}
	}
	form.Claims, err = controllerClaims(object, template)
	if err != nil {
		return ControllerSnapshot{}, err
	}
	result := ControllerSnapshot{Form: form, Baseline: formedit.Baseline(object), Status: controllerStatus(object, target.Resource)}
	for _, owner := range object.GetOwnerReferences() {
		if owner.Controller != nil && *owner.Controller && owner.APIVersion == "apps/v1" && owner.Kind == "Deployment" && target.Resource == "replicasets" {
			ref := target
			ref.Resource = "deployments"
			ref.Name = owner.Name
			result.Owner = &ref
			result.OwnerUID = string(owner.UID)
		}
	}
	return result, nil
}
func controllerClaims(object *unstructured.Unstructured, template corev1.PodTemplateSpec) ([]ClaimTemplate, error) {
	result := []ClaimTemplate{}
	claims, _, _ := unstructured.NestedSlice(object.Object, "spec", "volumeClaimTemplates")
	for _, raw := range claims {
		claim, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("PVC 模板格式无效")
		}
		var pvc corev1.PersistentVolumeClaim
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(claim, &pvc); err != nil {
			return nil, err
		}
		entry := ClaimTemplate{Name: pvc.Name, Capacity: quantity(pvc.Spec.Resources.Requests, corev1.ResourceStorage), ClassMode: "default", AccessModes: []string{}, VolumeMode: "Filesystem"}
		if pvc.Spec.StorageClassName != nil {
			entry.ClassMode = "named"
			entry.ClassName = *pvc.Spec.StorageClassName
			if entry.ClassName == "" {
				entry.ClassMode = "none"
			}
		}
		for _, mode := range pvc.Spec.AccessModes {
			entry.AccessModes = append(entry.AccessModes, string(mode))
		}
		if pvc.Spec.VolumeMode != nil {
			entry.VolumeMode = string(*pvc.Spec.VolumeMode)
		}
		for _, container := range template.Spec.Containers {
			for _, mount := range container.VolumeMounts {
				if mount.Name == pvc.Name {
					entry.MountPath = mount.MountPath
				}
			}
		}
		result = append(result, entry)
	}
	return result, nil
}

func stringField(object *unstructured.Unstructured, path ...string) string {
	value, _, _ := unstructured.NestedString(object.Object, path...)
	return value
}
func numberField(object *unstructured.Unstructured, path ...string) int64 {
	value, _, _ := unstructured.NestedInt64(object.Object, path...)
	return value
}
func intOrString(object *unstructured.Unstructured, key, fallback string) string {
	value, found, _ := unstructured.NestedFieldNoCopy(object.Object, "spec", "updateStrategy", "rollingUpdate", key)
	if !found {
		return fallback
	}
	return fmt.Sprint(value)
}
