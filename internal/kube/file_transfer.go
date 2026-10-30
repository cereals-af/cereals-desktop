package kube

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/filetransfer"
	"github.com/cereals-af/cereals-desktop/internal/session"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

// FileExec keeps binary data out of PTY processing and pins every invocation to
// the original Pod UID. It uses the same cancellable transports as terminals.
func (c *Client) FileExec(ctx context.Context, target session.Request, command []string, input io.Reader, output io.Writer) error {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	startup := time.AfterFunc(20*time.Second, stop)
	defer startup.Stop()
	pod, err := c.Pod(ctx, target.Target)
	if err != nil {
		return err
	}
	if pod.UID != target.UID {
		return errors.New("Pod 已被重建，请重新打开终端")
	}
	options := &corev1.PodExecOptions{Container: target.Container, Command: command, Stdin: input != nil, Stdout: true, Stderr: true, TTY: false}
	url := c.core.RESTClient().Post().Resource("pods").Namespace(target.Target.Namespace).Name(target.Target.Name).SubResource("exec").VersionedParams(options, scheme.ParameterCodec).URL()
	config := c.streamConfig(ctx, func() { startup.Stop() })
	primary, err := remotecommand.NewWebSocketExecutor(config, http.MethodGet, url.String())
	if err != nil {
		return requestError(err)
	}
	transport, upgrader, err := cancellableSPDY(config, ctx)
	if err != nil {
		return requestError(err)
	}
	fallback, err := remotecommand.NewSPDYExecutorRejectRedirects(transport, upgrader, http.MethodPost, url)
	if err != nil {
		return requestError(err)
	}
	executor, err := remotecommand.NewFallbackExecutor(primary, fallback, upgradeFailed)
	if err != nil {
		return requestError(err)
	}
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdin: input, Stdout: output, Stderr: io.Discard, Tty: false})
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var exit utilexec.ExitError
	if errors.As(err, &exit) {
		switch exit.ExitStatus() {
		case 17:
			return filetransfer.ErrExists
		case 18:
			return errors.New("目标不是普通文件或是符号链接，请指定普通文件")
		case 19, 126, 127:
			return errors.New("容器缺少文件传输所需的 /bin/sh、tar、rm、mv 或 ln 工具")
		case 20:
			return errors.New("容器目标目录不存在或不可访问")
		default:
			return errors.New("容器文件操作失败，请检查文件路径、读写权限和 tar 支持")
		}
	}
	return requestError(err)
}
