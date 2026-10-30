package kube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/session"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
	streamhttp "k8s.io/streaming/pkg/httpstream"
)

func (c *Client) SessionDone() <-chan struct{} { return c.lifetime.Done() }
func (c *Client) Pod(ctx context.Context, target resource.Ref) (session.Pod, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	pod, err := c.core.Pods(target.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return session.Pod{}, session.ErrPodGone
	}
	if err != nil {
		return session.Pod{}, requestError(err)
	}
	result := session.Pod{UID: string(pod.UID), Phase: string(pod.Status.Phase), Node: pod.Spec.NodeName, Containers: []session.Container{}}
	for _, container := range pod.Spec.Containers {
		result.Containers = append(result.Containers, containerInfo(container.Name, container.Ports, "普通", pod.Status.ContainerStatuses))
	}
	for _, container := range pod.Spec.InitContainers {
		result.Containers = append(result.Containers, containerInfo(container.Name, container.Ports, "Init", pod.Status.InitContainerStatuses))
	}
	for _, container := range pod.Spec.EphemeralContainers {
		result.Containers = append(result.Containers, containerInfo(container.Name, container.Ports, "临时", pod.Status.EphemeralContainerStatuses))
	}
	return result, nil
}
func containerInfo(name string, ports []corev1.ContainerPort, kind string, statuses []corev1.ContainerStatus) session.Container {
	result := session.Container{Name: name, Kind: kind, State: "waiting", Ports: []int32{}}
	for _, port := range ports {
		if port.Protocol == "" || port.Protocol == corev1.ProtocolTCP {
			result.Ports = append(result.Ports, port.ContainerPort)
		}
	}
	for _, status := range statuses {
		if status.Name == name {
			result.Restarts = status.RestartCount
			if status.State.Running != nil {
				result.State = "running"
			}
			if status.State.Terminated != nil {
				result.State = "terminated"
			}
			break
		}
	}
	return result
}
func (c *Client) RunSession(ctx context.Context, request session.Request, streams session.Streams) (outcome session.Outcome, err error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	// Bound startup while allowing established sessions to live until cancelled.
	ctx, cancelCause := context.WithCancelCause(ctx)
	defer cancelCause(nil)
	timeout := errors.New("会话建立超时，请检查网络和集群流式接口后重新连接")
	defer func() {
		if errors.Is(context.Cause(ctx), timeout) {
			err = timeout
		}
	}()
	startup := time.AfterFunc(20*time.Second, func() { cancelCause(timeout) })
	defer startup.Stop()
	ready := func(port uint16) { startup.Stop(); streams.Ready(port) }
	pod, err := c.Pod(ctx, request.Target)
	if err != nil {
		return session.Outcome{ExitCode: -1}, err
	}
	if pod.UID != request.UID {
		return session.Outcome{ExitCode: -1}, errors.New("Pod 已被重建，请重新打开会话")
	}
	switch request.Kind {
	case "logs":
		return session.Outcome{ExitCode: -1}, c.logs(ctx, request, streams.Output, ready)
	case "exec":
		return c.exec(ctx, request, streams, ready)
	case "forward":
		return session.Outcome{ExitCode: -1}, c.forward(ctx, request, ready)
	default:
		return session.Outcome{ExitCode: -1}, errors.New("未知会话类型")
	}
}
func (c *Client) logs(ctx context.Context, request session.Request, output io.Writer, ready func(uint16)) error {
	options := &corev1.PodLogOptions{Container: request.Container, Follow: request.Follow, Previous: request.Previous, Timestamps: request.Timestamps, TailLines: &request.TailLines}
	if request.SinceSeconds > 0 {
		options.SinceSeconds = &request.SinceSeconds
	}
	stream, err := c.core.Pods(request.Target.Namespace).GetLogs(request.Target.Name, options).Stream(ctx)
	if err != nil {
		return requestError(err)
	}
	defer stream.Close()
	ready(0)
	if _, err = io.Copy(output, stream); err != nil {
		return requestError(err)
	}
	return nil
}

type resizeQueue struct {
	ctx   context.Context
	sizes <-chan session.Size
}

func (q resizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case size := <-q.sizes:
		return &remotecommand.TerminalSize{Width: size.Columns, Height: size.Rows}
	case <-q.ctx.Done():
		return nil
	}
}
func (c *Client) exec(ctx context.Context, request session.Request, streams session.Streams, ready func(uint16)) (session.Outcome, error) {
	options := &corev1.PodExecOptions{Container: request.Container, Command: request.Command, Stdin: true, Stdout: true, Stderr: false, TTY: true}
	url := c.core.RESTClient().Post().Resource("pods").Namespace(request.Target.Namespace).Name(request.Target.Name).SubResource("exec").VersionedParams(options, scheme.ParameterCodec).URL()
	var once sync.Once
	config := c.streamConfig(ctx, func() { once.Do(func() { ready(0) }) })
	primary, err := remotecommand.NewWebSocketExecutor(config, http.MethodGet, url.String())
	if err != nil {
		return session.Outcome{ExitCode: -1}, requestError(err)
	}
	transport, upgrader, err := cancellableSPDY(config, ctx)
	if err != nil {
		return session.Outcome{ExitCode: -1}, requestError(err)
	}
	fallback, err := remotecommand.NewSPDYExecutorRejectRedirects(transport, upgrader, http.MethodPost, url)
	if err != nil {
		return session.Outcome{ExitCode: -1}, requestError(err)
	}
	executor, err := remotecommand.NewFallbackExecutor(primary, fallback, upgradeFailed)
	if err != nil {
		return session.Outcome{ExitCode: -1}, requestError(err)
	}
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdin: streams.Input, Stdout: streams.Output, Tty: true, TerminalSizeQueue: resizeQueue{ctx, streams.Sizes}})
	if err == nil {
		return session.Outcome{ExitCode: 0}, nil
	}
	var exit utilexec.ExitError
	if errors.As(err, &exit) {
		return session.Outcome{ExitCode: exit.ExitStatus()}, fmt.Errorf("远端命令退出，退出码 %d", exit.ExitStatus())
	}
	return session.Outcome{ExitCode: -1}, requestError(err)
}
func upgradeFailed(err error) bool {
	return httpstream.IsUpgradeFailure(err) || streamhttp.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
}

type sessionTransport struct {
	base  http.RoundTripper
	ctx   context.Context
	ready func()
}

func (t sessionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	// Port-forward dialers do not accept a context. Apply it at the transport
	// boundary so disconnect also cancels an in-progress protocol negotiation.
	ctx := httptrace.WithClientTrace(t.ctx, &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
		// Gorilla cancels dialing, but a stalled HTTP upgrade read also needs the
		// socket closed explicitly. Keep the hook through the session lifetime.
		context.AfterFunc(t.ctx, func() { info.Conn.Close() })
	}})
	response, err := t.base.RoundTrip(request.Clone(ctx))
	if err == nil && response.StatusCode == http.StatusSwitchingProtocols && t.ready != nil {
		t.ready()
	}
	return response, err
}
func (c *Client) streamConfig(ctx context.Context, ready func()) *rest.Config {
	config := rest.CopyConfig(c.config)
	config.Wrap(func(base http.RoundTripper) http.RoundTripper { return sessionTransport{base, ctx, ready} })
	return config
}
