package kube

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	streamspdy "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport/spdy"
)

// client-go's SPDY round trip reads upgrade headers directly from the socket.
// Retain its TLS/proxy dialer while making that read cancellable as well.
type spdyTransport struct {
	dialer interface {
		Dial(*http.Request) (net.Conn, error)
	}
	ctx  context.Context
	conn net.Conn
}

func cancellableSPDY(config *rest.Config, ctx context.Context) (http.RoundTripper, spdy.Upgrader, error) {
	tls, err := rest.TLSConfigFor(config)
	if err != nil {
		return nil, nil, err
	}
	proxy := config.Proxy
	if proxy == nil {
		proxy = http.ProxyFromEnvironment
	}
	dialer, err := streamspdy.NewRoundTripperWithConfig(streamspdy.RoundTripperConfig{TLS: tls, Proxier: proxy, PingPeriod: 5 * time.Second})
	if err != nil {
		return nil, nil, err
	}
	upgrade := &spdyTransport{dialer: dialer, ctx: ctx}
	wrapper, err := rest.HTTPWrappersForConfig(config, upgrade)
	return wrapper, upgrade, err
}
func (t *spdyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(t.ctx)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "SPDY/3.1")
	conn, err := t.dialer.Dial(request)
	if err != nil {
		return nil, err
	}
	context.AfterFunc(t.ctx, func() { conn.Close() })
	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		conn.Close()
		return nil, err
	}

	t.conn = conn
	return response, nil
}
func (t *spdyTransport) NewConnection(response *http.Response) (httpstream.Connection, error) {
	if t.conn == nil {
		return nil, errors.New("SPDY connection unavailable")
	}
	if response.StatusCode != http.StatusSwitchingProtocols || !strings.EqualFold(response.Header.Get("Upgrade"), "SPDY/3.1") {
		defer t.conn.Close()
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		var status metav1.Status
		if json.Unmarshal(body, &status) == nil && status.Kind == "Status" {
			return nil, &apierrors.StatusError{ErrStatus: status}
		}
		return nil, &apierrors.StatusError{ErrStatus: metav1.Status{Code: int32(response.StatusCode), Status: metav1.StatusFailure}}
	}
	return streamspdy.NewClientConnectionWithPings(t.conn, 5*time.Second)
}
