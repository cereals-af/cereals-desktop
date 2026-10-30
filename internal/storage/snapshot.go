package storage

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func claimSnapshot(object *unstructured.Unstructured) (Claim, error) {
	var pvc corev1.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &pvc); err != nil {
		return Claim{}, errors.New("PVC 字段格式无效，请查看 YAML")
	}
	result := Claim{Baseline: formedit.Baseline(object), Phase: string(pvc.Status.Phase), Requested: quantity(pvc.Spec.Resources.Requests), Capacity: quantity(pvc.Status.Capacity), Allocated: quantity(pvc.Status.AllocatedResources), ResizeStatus: string(pvc.Status.AllocatedResourceStatuses[corev1.ResourceStorage]), AccessModes: accessModes(pvc.Spec.AccessModes), VolumeMode: volumeMode(pvc.Spec.VolumeMode), VolumeName: pvc.Spec.VolumeName, ClassMode: "default", Conditions: []Condition{}, Deleting: pvc.DeletionTimestamp != nil}
	if pvc.Spec.StorageClassName != nil {
		result.ClassName = *pvc.Spec.StorageClassName
		result.ClassMode = "named"
		if result.ClassName == "" {
			result.ClassMode = "none"
		}
	}
	for _, c := range pvc.Status.Conditions {
		result.Conditions = append(result.Conditions, Condition{Type: string(c.Type), Status: string(c.Status), Reason: c.Reason, Message: c.Message})
	}
	return result, nil
}

func volumeSnapshot(object *unstructured.Unstructured) (Volume, error) {
	var pv corev1.PersistentVolume
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &pv); err != nil {
		return Volume{}, errors.New("PV 字段格式无效，请查看 YAML")
	}
	result := Volume{Phase: string(pv.Status.Phase), Capacity: quantity(pv.Spec.Capacity), AccessModes: accessModes(pv.Spec.AccessModes), VolumeMode: volumeMode(pv.Spec.VolumeMode), ClassName: pv.Spec.StorageClassName, ReclaimPolicy: string(pv.Spec.PersistentVolumeReclaimPolicy), MountOptions: pv.Spec.MountOptions, Reason: pv.Status.Reason, Message: pv.Status.Message, Driver: volumeDriver(object)}
	if pv.Spec.ClaimRef != nil {
		result.ClaimName, result.ClaimNamespace, result.ClaimUID = pv.Spec.ClaimRef.Name, pv.Spec.ClaimRef.Namespace, string(pv.Spec.ClaimRef.UID)
	}
	if pv.Spec.NodeAffinity != nil {
		data, err := json.Marshal(pv.Spec.NodeAffinity)
		if err != nil {
			return Volume{}, err
		}
		result.NodeAffinity = string(data)
	}
	return result, nil
}

func classSnapshot(object *unstructured.Unstructured) (Class, error) {
	var sc storagev1.StorageClass
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &sc); err != nil {
		return Class{}, errors.New("StorageClass 字段格式无效，请查看 YAML")
	}
	result := Class{Provisioner: sc.Provisioner, ReclaimPolicy: "Delete", BindingMode: "Immediate", AllowExpansion: sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion, Default: sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" || sc.Annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true", MountOptions: sc.MountOptions}
	if sc.ReclaimPolicy != nil {
		result.ReclaimPolicy = string(*sc.ReclaimPolicy)
	}
	if sc.VolumeBindingMode != nil {
		result.BindingMode = string(*sc.VolumeBindingMode)
	}
	if len(sc.AllowedTopologies) != 0 {
		data, err := json.Marshal(sc.AllowedTopologies)
		if err != nil {
			return Class{}, err
		}
		result.AllowedTopologies = string(data)
	}
	return result, nil
}

func quantity(resources corev1.ResourceList) string {
	if value, ok := resources[corev1.ResourceStorage]; ok {
		return value.String()
	}
	return ""
}

func accessModes(modes []corev1.PersistentVolumeAccessMode) []string {
	result := make([]string, 0, len(modes))
	for _, mode := range modes {
		result = append(result, string(mode))
	}
	return result
}

func volumeMode(mode *corev1.PersistentVolumeMode) string {
	if mode == nil {
		return "Filesystem"
	}
	return string(*mode)
}

func volumeDriver(object *unstructured.Unstructured) string {
	if driver, _, _ := unstructured.NestedString(object.Object, "spec", "csi", "driver"); driver != "" {
		return driver
	}
	// Only identify the source type; provider parameters and credential references stay in YAML.
	for _, source := range strings.Fields("local hostPath nfs iscsi fc flexVolume portworxVolume rbd cephfs glusterfs awsElasticBlockStore azureDisk azureFile gcePersistentDisk cinder vsphereVolume scaleIO storageos photonPersistentDisk quobyte flocker") {
		if _, found, _ := unstructured.NestedFieldNoCopy(object.Object, "spec", source); found {
			return source
		}
	}
	return "未知来源（查看 YAML）"
}
