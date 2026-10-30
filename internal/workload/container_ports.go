package workload

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

func containerPortKey(name string, number int32, protocol string) string {
	if name != "" {
		return "name:" + name
	}
	return fmt.Sprintf("port:%d/%s", number, protocol)
}

func editContainerPorts(raw map[string]interface{}, old, next []ContainerPort) error {
	existing, _, err := unstructured.NestedSlice(raw, "ports")
	if err != nil {
		return errors.New("容器端口格式无效")
	}
	originals := make(map[string]int, len(old))
	for index, port := range old {
		if _, exists := originals[port.OriginalKey]; exists {
			return errors.New("原容器端口标识重复，请使用 YAML 编辑")
		}
		originals[port.OriginalKey] = index
	}
	seenKeys, seenNames, seenPorts := map[string]bool{}, map[string]bool{}, map[string]bool{}
	result := make([]interface{}, 0, len(next))
	for _, port := range next {
		if err := validateContainerPort(port); err != nil {
			return err
		}
		if port.Name != "" && seenNames[port.Name] {
			return errors.New("容器端口名称不能重复")
		}
		seenNames[port.Name] = true
		address := fmt.Sprintf("%d/%s", port.ContainerPort, port.Protocol)
		if seenPorts[address] {
			return errors.New("同一容器的端口号与协议组合不能重复")
		}
		seenPorts[address] = true
		entry := map[string]interface{}{}
		var original ContainerPort
		if port.OriginalKey != "" {
			index, found := originals[port.OriginalKey]
			if !found || index >= len(existing) || seenKeys[port.OriginalKey] {
				return errors.New("容器端口编辑基线无效，请重新读取资源")
			}
			seenKeys[port.OriginalKey] = true
			var ok bool
			entry, ok = existing[index].(map[string]interface{})
			if !ok {
				return errors.New("容器端口格式无效")
			}
			original = old[index]
		}
		if !reflect.DeepEqual(original, port) {
			writeContainerPort(entry, original, port)
		}
		result = append(result, entry)
	}
	if len(result) == 0 {
		delete(raw, "ports")
	} else {
		raw["ports"] = result
	}
	return nil
}

func validateContainerPort(port ContainerPort) error {
	if port.ContainerPort < 1 || port.ContainerPort > 65535 {
		return errors.New("容器端口必须在 1–65535 之间")
	}
	if port.HostPort < 0 || port.HostPort > 65535 {
		return errors.New("主机端口必须为 0（不映射）或 1–65535")
	}
	if port.HostIP != "" && net.ParseIP(port.HostIP) == nil {
		return errors.New("主机 IP 必须是有效的 IPv4 或 IPv6 地址")
	}
	if port.Name != "" && len(validation.IsValidPortName(port.Name)) != 0 {
		return errors.New("端口名称须为不超过 15 字符的小写字母、数字或连字符，并至少包含一个字母")
	}
	if !slices.Contains([]string{"TCP", "UDP", "SCTP"}, port.Protocol) {
		return errors.New("端口协议必须为 TCP、UDP 或 SCTP")
	}
	return nil
}

func writeContainerPort(raw map[string]interface{}, old, next ContainerPort) {
	for _, field := range []struct{ name, old, next string }{
		{"name", old.Name, next.Name}, {"protocol", old.Protocol, next.Protocol}, {"hostIP", old.HostIP, next.HostIP},
	} {
		if field.old == field.next {
			continue
		}
		if field.next == "" {
			delete(raw, field.name)
		} else {
			raw[field.name] = field.next
		}
	}
	if old.ContainerPort != next.ContainerPort {
		raw["containerPort"] = int64(next.ContainerPort)
	}
	if old.HostPort != next.HostPort {
		if next.HostPort == 0 {
			delete(raw, "hostPort")
		} else {
			raw["hostPort"] = int64(next.HostPort)
		}
	}
}
