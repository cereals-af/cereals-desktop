package workload

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

func controllerKind(resource string) string {
	return map[string]string{"statefulsets": "StatefulSet", "daemonsets": "DaemonSet", "replicasets": "ReplicaSet"}[resource]
}
func newController(request ControllerRequest) (*unstructured.Unstructured, error) {
	object, err := newPodController(request.Target, request.Form.Workload, controllerKind(request.Target.Resource), request.Target.Resource != "daemonsets")
	if err != nil {
		return nil, err
	}
	if err = editControllerFields(object, ControllerForm{}, request.Form, request.Target.Resource, true); err != nil {
		return nil, err
	}
	return object, nil
}
func updateController(object *unstructured.Unstructured, request ControllerRequest) error {
	current, err := controllerSnapshot(object, request.Target)
	if err != nil {
		return err
	}
	old, next := current.Form, request.Form
	if old.Workload.Name != next.Workload.Name || old.Workload.Namespace != next.Workload.Namespace {
		return errors.New("编辑时不能更改名称或命名空间")
	}
	if old.Workload.Replicas != next.Workload.Replicas {
		return errors.New("副本管理请使用独立的扩缩容操作（scale 子资源）")
	}
	if err := editPodOptions(object, next.Workload.PodOptions); err != nil {
		return err
	}
	if err = updateContainers(object, old.Workload.Containers, next.Workload.Containers); err != nil {
		return err
	}
	return editControllerFields(object, old, next, request.Target.Resource, false)
}
func editControllerFields(object *unstructured.Unstructured, old, next ControllerForm, kind string, creating bool) error {
	if creating || !reflect.DeepEqual(old.NodeSelector, next.NodeSelector) {
		labels := map[string]interface{}{}
		for _, label := range next.NodeSelector {
			if len(validation.IsQualifiedName(label.Key)) > 0 || len(validation.IsValidLabelValue(label.Value)) > 0 {
				return errors.New("节点选择器标签无效")
			}
			if _, exists := labels[label.Key]; exists {
				return errors.New("节点选择器键重复")
			}
			labels[label.Key] = label.Value
		}
		if err := unstructured.SetNestedMap(object.Object, labels, "spec", "template", "spec", "nodeSelector"); err != nil {
			return err
		}
	}
	if kind == "replicasets" {
		return nil
	}
	if creating || old.Strategy != next.Strategy || old.Partition != next.Partition || old.MaxUnavailable != next.MaxUnavailable || old.MaxSurge != next.MaxSurge {
		if err := editStrategy(object, next, kind); err != nil {
			return err
		}
	}
	if kind != "statefulsets" {
		return nil
	}
	if !creating && (old.ServiceName != next.ServiceName || old.PodManagementPolicy != next.PodManagementPolicy || !reflect.DeepEqual(old.Claims, next.Claims)) {
		return errors.New("StatefulSet 的 Service、Pod 顺序和 PVC 模板创建后不可通过此表单修改；已有 PVC 扩容请进入存储管理")
	}
	if creating {
		if next.ServiceName == "" {
			return errors.New("请填写治理 Service 名称")
		}
		if next.PodManagementPolicy != "OrderedReady" && next.PodManagementPolicy != "Parallel" {
			return errors.New("Pod 管理策略无效")
		}
		if err := unstructured.SetNestedField(object.Object, next.ServiceName, "spec", "serviceName"); err != nil {
			return err
		}
		if err := unstructured.SetNestedField(object.Object, next.PodManagementPolicy, "spec", "podManagementPolicy"); err != nil {
			return err
		}
		if err := createClaims(object, next.Claims); err != nil {
			return err
		}
	}
	for _, field := range []struct{ key, old, next string }{{"whenDeleted", old.RetainDeleted, next.RetainDeleted}, {"whenScaled", old.RetainScaled, next.RetainScaled}} {
		if !creating && field.old == field.next {
			continue
		}
		if field.next != "Retain" && field.next != "Delete" {
			return errors.New("PVC 保留策略必须为 Retain 或 Delete")
		}
		if err := unstructured.SetNestedField(object.Object, field.next, "spec", "persistentVolumeClaimRetentionPolicy", field.key); err != nil {
			return err
		}
	}
	return nil
}
func editStrategy(object *unstructured.Unstructured, form ControllerForm, kind string) error {
	if form.Strategy != "RollingUpdate" && form.Strategy != "OnDelete" {
		return errors.New("此表单仅编辑 RollingUpdate / OnDelete 策略，其他策略请使用 YAML")
	}
	if err := unstructured.SetNestedField(object.Object, form.Strategy, "spec", "updateStrategy", "type"); err != nil {
		return err
	}
	if form.Strategy == "OnDelete" {
		unstructured.RemoveNestedField(object.Object, "spec", "updateStrategy", "rollingUpdate")
		return nil
	}
	if kind == "statefulsets" {
		if form.Partition < 0 {
			return errors.New("partition 不能小于零")
		}
		return unstructured.SetNestedField(object.Object, int64(form.Partition), "spec", "updateStrategy", "rollingUpdate", "partition")
	}
	for _, field := range []struct{ key, value string }{{"maxUnavailable", form.MaxUnavailable}, {"maxSurge", form.MaxSurge}} {
		value, err := availabilityValue(field.value)
		if err != nil {
			return fmt.Errorf("%s: %w", field.key, err)
		}
		if err = unstructured.SetNestedField(object.Object, value, "spec", "updateStrategy", "rollingUpdate", field.key); err != nil {
			return err
		}
	}
	return nil
}
func availabilityValue(value string) (interface{}, error) {
	percent := strings.HasSuffix(value, "%")
	raw := strings.TrimSuffix(value, "%")
	number, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || number < 0 || (percent && number > 100) {
		return nil, errors.New("请输入非负整数或 0% 到 100%")
	}
	if percent {
		return value, nil
	}
	return number, nil
}
