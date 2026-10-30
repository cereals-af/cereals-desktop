package workload

import (
	"errors"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func editProbe(container map[string]interface{}, name string, old, next Probe) error {
	if next.Kind == "none" || next.Kind == "" {
		delete(container, name)
		return nil
	}
	if next.Kind == "keep" {
		return errors.New("此探针类型请使用 YAML 编辑")
	}
	if next.InitialDelay < 0 || next.Timeout < 1 || next.Period < 1 || next.SuccessThreshold < 1 || next.FailureThreshold < 1 {
		return errors.New("探针延迟须非负，其余时间和阈值须大于零")
	}
	if name != "readinessProbe" && next.SuccessThreshold != 1 {
		return errors.New("存活及启动探针的成功阈值必须为 1")
	}
	probe, _, err := unstructured.NestedMap(container, name)
	if err != nil {
		return err
	}
	if probe == nil {
		probe = map[string]interface{}{}
	}
	if next.Kind != old.Kind {
		for _, handler := range []string{"httpGet", "tcpSocket", "exec", "grpc"} {
			delete(probe, handler)
		}
	}
	if err := setHandler(probe, next); err != nil {
		return err
	}
	probe["initialDelaySeconds"], probe["timeoutSeconds"], probe["periodSeconds"] = int64(next.InitialDelay), int64(next.Timeout), int64(next.Period)
	probe["successThreshold"], probe["failureThreshold"] = int64(next.SuccessThreshold), int64(next.FailureThreshold)
	container[name] = probe
	return nil
}

func setHandler(probe map[string]interface{}, next Probe) error {
	switch next.Kind {
	case "http", "tcp":
		if next.Port == "" {
			return errors.New("探针端口不能为空")
		}
		var port interface{} = next.Port
		if number, err := strconv.ParseInt(next.Port, 10, 32); err == nil {
			if number < 1 || number > 65535 {
				return errors.New("探针端口必须在 1–65535 之间")
			}
			port = number
		}
		field := "tcpSocket"
		if next.Kind == "http" {
			field = "httpGet"
		}
		if err := unstructured.SetNestedField(probe, port, field, "port"); err != nil {
			return err
		}
		if next.Kind == "http" {
			if next.Scheme != "HTTP" && next.Scheme != "HTTPS" {
				return errors.New("HTTP 探针需要 HTTP 或 HTTPS 协议")
			}
			if err := unstructured.SetNestedField(probe, next.Path, field, "path"); err != nil {
				return err
			}
			return unstructured.SetNestedField(probe, next.Scheme, field, "scheme")
		}
	case "exec":
		if len(next.Command) == 0 || next.Command[0] == "" {
			return errors.New("Exec 探针命令不能为空")
		}
		return unstructured.SetNestedStringSlice(probe, next.Command, "exec", "command")
	default:
		return errors.New("不支持的探针类型")
	}
	return nil
}
