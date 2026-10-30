package network

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
)

func editPorts(spec map[string]interface{}, form ServiceForm, creating bool) error {
	if len(form.Ports) == 0 {
		return errors.New("至少需要一个 Service 端口")
	}
	existing, _, err := unstructured.NestedSlice(spec, "ports")
	if err != nil {
		return errors.New("Service 端口格式无效")
	}
	ports := make([]interface{}, 0, len(form.Ports))
	usedKeys, usedNames := map[string]bool{}, map[string]bool{}
	for _, port := range form.Ports {
		if port.Name == "" && len(form.Ports) > 1 {
			return errors.New("多个端口必须分别命名")
		}
		if usedNames[port.Name] {
			return errors.New("端口名称不能重复")
		}
		usedNames[port.Name] = true
		raw, err := originalPort(existing, port.OriginalKey, creating, usedKeys)
		if err != nil {
			return err
		}
		if err := editPort(raw, port, form.Type); err != nil {
			return err
		}
		ports = append(ports, raw)
	}
	spec["ports"] = ports
	return nil
}

func originalPort(existing []interface{}, key string, creating bool, used map[string]bool) (map[string]interface{}, error) {
	if key == "" {
		return map[string]interface{}{}, nil
	}
	if creating || used[key] {
		return nil, errors.New("端口编辑基线无效，请重新读取资源")
	}
	used[key] = true
	for _, value := range existing {
		raw, ok := value.(map[string]interface{})
		if !ok {
			return nil, errors.New("端口格式无效，请使用 YAML 编辑")
		}
		name, _ := raw["name"].(string)
		number, _ := raw["port"].(int64)
		protocol, _ := raw["protocol"].(string)
		if portKey(name, number, protocol) == key {
			return runtime.DeepCopyJSON(raw), nil
		}
	}
	return nil, errors.New("原端口已删除或重命名，请重新打开表单后比较修改")
}

func portKey(name string, number int64, protocol string) string {
	if name != "" {
		return "name:" + name
	}
	if protocol == "" {
		protocol = "TCP"
	}
	return fmt.Sprintf("port:%d/%s", number, protocol)
}

func editPort(raw map[string]interface{}, port Port, serviceType string) error {
	if port.Port < 1 || port.Port > 65535 {
		return errors.New("Service 端口必须在 1–65535 之间")
	}
	if port.Name != "" && len(validation.IsDNS1123Label(port.Name)) != 0 {
		return errors.New("端口名称需要有效的小写 DNS 标签")
	}
	if !slices.Contains([]string{"TCP", "UDP", "SCTP"}, port.Protocol) {
		return errors.New("端口协议必须为 TCP、UDP 或 SCTP")
	}
	if port.AppProtocol != "" && len(validation.IsQualifiedName(port.AppProtocol)) != 0 {
		return errors.New("应用协议需要有效的协议标识")
	}
	optionalString(raw, "name", port.Name)
	optionalString(raw, "appProtocol", port.AppProtocol)
	raw["port"], raw["protocol"] = int64(port.Port), port.Protocol
	if err := targetPort(raw, port.TargetPort); err != nil {
		return err
	}
	if serviceType == "NodePort" || serviceType == "LoadBalancer" {
		if port.NodePort < 0 || port.NodePort > 65535 {
			return errors.New("节点端口须为 0（自动分配）或有效端口；集群允许范围由服务端校验")
		}
		if port.NodePort == 0 {
			delete(raw, "nodePort")
		} else {
			raw["nodePort"] = int64(port.NodePort)
		}
	} else {
		delete(raw, "nodePort")
	}
	return nil
}

func targetPort(raw map[string]interface{}, text string) error {
	if text == "" {
		delete(raw, "targetPort")
		return nil
	}
	if number, err := strconv.ParseInt(text, 10, 32); err == nil {
		if number < 1 || number > 65535 {
			return errors.New("目标端口必须在 1–65535 之间")
		}
		raw["targetPort"] = number
		return nil
	}
	if len(validation.IsValidPortName(text)) != 0 {
		return errors.New("目标端口必须是有效的端口号或端口名称")
	}
	raw["targetPort"] = text
	return nil
}

func optionalString(object map[string]interface{}, key, value string) {
	if value == "" {
		delete(object, key)
	} else {
		object[key] = value
	}
}
