package session

import (
	"errors"
	"strings"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

func validateTarget(target resource.Ref) error {
	if target.ClusterID == "" || target.Group != "" || target.Version != "v1" || target.Resource != "pods" || target.Namespace == "" || target.Name == "" {
		return errors.New("请选择明确集群和命名空间中的 Pod")
	}
	return nil
}
func validate(request Request, pod Pod) error {
	if request.UID == "" || request.UID != pod.UID {
		return errors.New("Pod 已被重建，请关闭面板后重新打开")
	}
	switch request.Kind {
	case "logs":
		if request.TailLines < 1 || request.TailLines > 10000 || request.SinceSeconds < 0 || request.SinceSeconds > 7*24*3600 {
			return errors.New("日志行数应为 1–10000，时间范围不超过 7 天")
		}
		if request.Previous && request.Follow {
			return errors.New("历史容器日志不能持续跟随")
		}
	case "exec":
		if len(request.Command) == 0 || len(request.Command) > 32 {
			return errors.New("请提供终端命令")
		}
		size := 0
		for _, arg := range request.Command {
			size += len(arg)
			if strings.ContainsRune(arg, 0) {
				return errors.New("终端命令包含无效字符")
			}
		}
		if size > 4096 || request.Command[0] == "" {
			return errors.New("终端命令无效或过长")
		}
		if err := validateSize(Size{request.Columns, request.Rows}); err != nil {
			return err
		}
	case "forward":
		if request.RemotePort == 0 {
			return errors.New("远端端口应为 1–65535")
		}
		if pod.Phase != "Running" {
			return errors.New("端口转发需要 Running 状态的 Pod")
		}
		return nil
	default:
		return errors.New("不支持的会话类型")
	}
	for _, container := range pod.Containers {
		if container.Name != request.Container {
			continue
		}
		if request.Kind == "exec" && container.State != "running" {
			return errors.New("只能连接正在运行的容器")
		}
		return nil
	}
	return errors.New("所选容器不存在，请刷新 Pod")
}
func validateSize(size Size) error {
	if size.Columns < 2 || size.Columns > 500 || size.Rows < 1 || size.Rows > 300 {
		return errors.New("终端尺寸超出范围")
	}
	return nil
}
