package kube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/session"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func (c *Client) forward(ctx context.Context, request session.Request, ready func(uint16)) error {
	config := c.streamConfig(ctx, nil)
	url := c.core.RESTClient().Post().Resource("pods").Namespace(request.Target.Namespace).Name(request.Target.Name).SubResource("portforward").URL()
	transport, upgrader, err := cancellableSPDY(config, ctx)
	if err != nil {
		return requestError(err)
	}
	fallback := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, url)
	primary, err := portforward.NewSPDYOverWebsocketDialer(url, config)
	if err != nil {
		return requestError(err)
	}
	dialer := portforward.NewFallbackDialer(primary, fallback, upgradeFailed)
	connection, protocol, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		return requestError(err)
	}
	defer connection.Close()
	if protocol != portforward.PortForwardProtocolV1Name {
		return errors.New("服务端不支持端口转发协议")
	}
	return serveForward(ctx, connection, request.LocalPort, request.RemotePort, ready)
}

// serveForward binds only IPv4 loopback, with at most 16 concurrent local
// connections. Every accepted socket and upstream stream belongs to this context.
func serveForward(ctx context.Context, upstream httpstream.Connection, local, remote uint16, ready func(uint16)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", local))
	if err != nil {
		return errors.New("无法监听本地端口，请检查端口占用或选择自动分配")
	}
	defer listener.Close()
	stop := context.AfterFunc(ctx, func() { listener.Close(); upstream.Close() })
	defer stop()
	watcherDone := make(chan struct{})
	defer close(watcherDone)
	go func() {
		select {
		case <-upstream.CloseChan():
			cancel()
		case <-watcherDone:
		case <-ctx.Done():
		}
	}()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return errors.New("无法读取本地监听端口")
	}
	ready(uint16(address.Port))
	var workers sync.WaitGroup
	defer workers.Wait()
	slots := make(chan struct{}, 16)
	var requestID uint64
	for {
		socket, err := listener.Accept()
		if err != nil {
			cancel()
			if ctx.Err() != nil {
				return errors.New("端口转发已断开，请重新建立连接")
			}
			return errors.New("本地转发监听失败")
		}
		select {
		case slots <- struct{}{}:
		default:
			socket.Close()
			continue
		}
		requestID++
		id := requestID
		workers.Go(func() { defer func() { <-slots }(); pipeForward(ctx, upstream, socket, remote, id) })
	}
}
func pipeForward(ctx context.Context, upstream httpstream.Connection, socket net.Conn, port uint16, id uint64) {
	defer socket.Close()
	stop := context.AfterFunc(ctx, func() { socket.Close() })
	defer stop()
	// A peer that never acknowledges stream creation must not retain a slot.
	deadline := time.AfterFunc(15*time.Second, func() { upstream.Close() })
	headers := http.Header{}
	headers.Set(corev1.PortHeader, fmt.Sprint(port))
	headers.Set(corev1.PortForwardRequestIDHeader, fmt.Sprint(id))
	headers.Set(corev1.StreamType, corev1.StreamTypeError)
	errorStream, err := upstream.CreateStream(headers)
	if err != nil {
		deadline.Stop()
		return
	}
	defer upstream.RemoveStreams(errorStream)
	defer errorStream.Reset()
	errorStream.Close() // Close the local write side; the remote reports errors here.
	headers.Set(corev1.StreamType, corev1.StreamTypeData)
	data, err := upstream.CreateStream(headers)
	deadline.Stop()
	if err != nil {
		return
	}
	defer upstream.RemoveStreams(data)
	defer data.Reset()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	closePipes := context.AfterFunc(child, func() { socket.Close(); data.Reset(); errorStream.Reset() })
	defer closePipes()
	var copies sync.WaitGroup
	copies.Go(func() { _, _ = io.Copy(data, socket); data.Close() })
	copies.Go(func() { _, _ = io.Copy(socket, data); cancel() })
	copies.Go(func() {
		// Never log raw kubelet messages; close this socket on a remote error.
		message, _ := io.ReadAll(io.LimitReader(errorStream, 4096))
		if len(message) > 0 {
			cancel()
		}
	})
	copies.Wait()
}
