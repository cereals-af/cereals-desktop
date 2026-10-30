package workload

import (
	"errors"

	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var podOptionFields = []string{"initContainers", "volumes", "nodeSelector", "affinity", "tolerations", "securityContext", "serviceAccountName", "automountServiceAccountToken", "imagePullSecrets", "terminationGracePeriodSeconds", "dnsPolicy", "dnsConfig", "hostNetwork", "hostPID", "hostIPC", "priorityClassName", "topologySpreadConstraints"}
var containerOptionFields = []string{"env", "envFrom", "volumeMounts", "volumeDevices", "securityContext", "workingDir", "imagePullPolicy"}

func selectedFields(raw map[string]interface{}, fields []string) (*formvalue.Value, error) {
	selected := map[string]interface{}{}
	for _, key := range fields {
		if value, found := raw[key]; found {
			selected[key] = value
		}
	}
	value, err := formvalue.Encode(selected)
	return &value, err
}

func podOptions(object *unstructured.Unstructured) (*formvalue.Value, error) {
	raw, _, err := unstructured.NestedMap(object.Object, "spec", "template", "spec")
	if err != nil {
		return nil, err
	}
	return selectedFields(raw, podOptionFields)
}

func attachContainerOptions(object *unstructured.Unstructured, forms []Container) error {
	raw, err := containersByName(object, "containers")
	if err != nil {
		return err
	}
	for i := range forms {
		forms[i].Advanced, err = selectedFields(raw[forms[i].Name], containerOptionFields)
		if err != nil {
			return err
		}
	}
	return nil
}

// A nil advanced block means an older caller has not edited those fields.
func editAdvanced(raw map[string]interface{}, value *formvalue.Value, fields []string) error {
	if value == nil {
		return nil
	}
	decoded, err := formvalue.Decode(*value)
	if err != nil {
		return err
	}
	values, ok := decoded.(map[string]interface{})
	if !ok {
		return errors.New("高级配置必须是对象")
	}
	allowed := map[string]bool{}
	for _, key := range fields {
		allowed[key] = true
	}
	for key := range values {
		if !allowed[key] {
			return errors.New("高级配置包含不支持的字段")
		}
	}
	for _, key := range fields {
		if value, found := values[key]; found {
			raw[key] = value
		} else {
			delete(raw, key)
		}
	}
	return nil
}

func editPodOptions(object *unstructured.Unstructured, value *formvalue.Value) error {
	if value == nil {
		return nil
	}
	raw, _, err := unstructured.NestedMap(object.Object, "spec", "template", "spec")
	if err != nil {
		return err
	}
	if raw == nil {
		raw = map[string]interface{}{}
	}
	if err = editAdvanced(raw, value, podOptionFields); err != nil {
		return err
	}
	return unstructured.SetNestedMap(object.Object, raw, "spec", "template", "spec")
}
