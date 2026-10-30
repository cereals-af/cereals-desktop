package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/session"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	streamspdy "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/rest"
)

func podRequest(kind string) session.Request {
	return session.Request{Target: resource.Ref{ClusterID: "cluster", Version: "v1", Resource: "pods", Namespace: "ns", Name: "demo"}, UID: "pod-1", Kind: kind, Container: "app", TailLines: 500, Command: []string{"/bin/sh"}, Columns: 80, Rows: 24, RemotePort: 8080}
}

const sessionPodJSON = `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"demo","namespace":"ns","uid":"pod-1"},"spec":{"nodeName":"node-1","containers":[{"name":"app","ports":[{"containerPort":8080}]}]},"status":{"phase":"Running","containerStatuses":[{"name":"app","state":{"running":{}},"restartCount":2}]}}`

func protocolClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/namespaces/ns/pods/demo" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, sessionPodJSON)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	client, err := (Factory{}).New(t.Context(), &rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client.(*Client)
}
func TestLogStreamOptionsAndCancel(t *testing.T) {
	ended := make(chan struct{})
	client := protocolClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/ns/pods/demo/log" {
			t.Errorf("path %s", r.URL.Path)
		}
		query := r.URL.Query()
		for key, want := range map[string]string{"container": "app", "follow": "true", "tailLines": "500", "sinceSeconds": "300", "timestamps": "true"} {
			if query.Get(key) != want {
				t.Errorf("%s=%s", key, query.Get(key))
			}
		}
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "启动成功\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(ended)
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	request := podRequest("logs")
	request.Follow = true
	request.Timestamps = true
	request.SinceSeconds = 300
	output := &cancelWriter{cancel: cancel}
	_, err := client.RunSession(ctx, request, session.Streams{Output: output, Ready: func(uint16) {}})
	if err == nil || output.text != "启动成功\n" {
		t.Fatalf("output %q, err %v", output.text, err)
	}
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("log HTTP stream not cancelled")
	}
}

type cancelWriter struct {
	cancel context.CancelFunc
	text   string
}

func (w *cancelWriter) Write(data []byte) (int, error) {
	w.text += string(data)
	w.cancel()
	return len(data), nil
}

func TestExecWebSocketInputOutputResizeAndExit(t *testing.T) {
	for _, code := range []int{0, 7} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			received := make(chan string, 1)
			resized := make(chan string, 1)
			client := protocolClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/api/v1/namespaces/ns/pods/demo/exec" || r.URL.Query().Get("tty") != "true" || r.URL.Query().Get("container") != "app" {
					t.Errorf("exec request %s %s", r.Method, r.URL)
				}
				upgrader := websocket.Upgrader{Subprotocols: []string{"v5.channel.k8s.io"}}
				connection, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer connection.Close()
				connection.SetReadDeadline(time.Now().Add(3 * time.Second))
				input, resize := false, false
				for !input || !resize {
					_, data, err := connection.ReadMessage()
					if err != nil {
						t.Error(err)
						return
					}
					if len(data) < 1 {
						continue
					}
					switch data[0] {
					case 0:
						received <- string(data[1:])
						input = true
					case 4:
						resized <- string(data[1:])
						resize = true
					}
				}
				output := []byte("你好\r\n")
				connection.WriteMessage(websocket.BinaryMessage, append([]byte{1}, output[:2]...))
				connection.WriteMessage(websocket.BinaryMessage, append([]byte{1}, output[2:]...))
				status := `{"status":"Success"}`
				if code != 0 {
					status = fmt.Sprintf(`{"status":"Failure","reason":"NonZeroExitCode","details":{"causes":[{"reason":"ExitCode","message":"%d"}]}}`, code)
				}
				connection.WriteMessage(websocket.BinaryMessage, append([]byte{3}, []byte(status)...))
				connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				for {
					if _, _, err := connection.ReadMessage(); err != nil {
						break
					}
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			sizes := make(chan session.Size, 1)
			sizes <- session.Size{Columns: 120, Rows: 40}
			var output bytes.Buffer
			ready := false
			outcome, err := client.RunSession(ctx, podRequest("exec"), session.Streams{Input: strings.NewReader("echo hello\n"), Output: &output, Sizes: sizes, Ready: func(uint16) { ready = true }})
			if outcome.ExitCode != code || (code == 0 && err != nil) || (code != 0 && err == nil) {
				t.Fatalf("outcome %#v %v", outcome, err)
			}
			if !ready || output.String() != "你好\r\n" {
				t.Fatalf("ready %v, output %q", ready, output.String())
			}
			if value := <-received; value != "echo hello\n" {
				t.Fatal(value)
			}
			var size struct{ Width, Height uint16 }
			if err := json.Unmarshal([]byte(<-resized), &size); err != nil || size.Width != 120 || size.Height != 40 {
				t.Fatalf("size %v %v", size, err)
			}
		})
	}
}
func TestSessionRejectsReplacementBeforeOpeningStream(t *testing.T) {
	client := protocolClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("opened stream for replacement Pod") })
	request := podRequest("logs")
	request.UID = "old-pod"
	if _, err := client.RunSession(t.Context(), request, session.Streams{}); err == nil {
		t.Fatal("accepted replaced Pod")
	}
}

func TestPortForwardSPDYFallbackDataAndRelease(t *testing.T) {
	var serverWorkers sync.WaitGroup
	client := protocolClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.Error(w, "WebSocket unavailable", http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/api/v1/namespaces/ns/pods/demo/portforward" {
			t.Errorf("path %s", r.URL.Path)
		}
		if _, err := httpstream.Handshake(r, w, []string{"portforward.k8s.io"}); err != nil {
			return
		}
		streams := make(chan httpstream.Stream, 32)
		connection := streamspdy.NewResponseUpgrader().UpgradeResponse(w, r, func(stream httpstream.Stream, reply <-chan struct{}) error { streams <- stream; return nil })
		if connection == nil {
			return
		}
		defer connection.Close()
		for {
			select {
			case <-connection.CloseChan():
				return
			case stream := <-streams:
				if stream.Headers().Get(corev1.PortHeader) != "8080" {
					t.Error("incorrect target port")
				}
				if stream.Headers().Get(corev1.StreamType) == corev1.StreamTypeError {
					stream.Close()
					continue
				}
				serverWorkers.Go(func() { defer stream.Close(); io.Copy(stream, stream) })
			}
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	portCh := make(chan uint16, 1)
	result := make(chan error, 1)
	go func() {
		_, err := client.RunSession(ctx, podRequest("forward"), session.Streams{Ready: func(port uint16) { portCh <- port }})
		result <- err
	}()
	var port uint16
	select {
	case port = <-portCh:
	case err := <-result:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("forward did not start")
	}
	address := fmt.Sprintf("127.0.0.1:%d", port)
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err = connection.Write([]byte("forward traffic")); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, 15)
	if _, err = io.ReadFull(connection, echo); err != nil || string(echo) != "forward traffic" {
		t.Fatalf("echo %q %v", echo, err)
	}
	cancel()
	select {
	case <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("forward did not stop")
	}
	if reopened, err := net.Listen("tcp4", address); err != nil {
		t.Fatalf("port not released: %v", err)
	} else {
		reopened.Close()
	}
	serverWorkers.Wait()
}

func TestPortForwardHandshakeCancellation(t *testing.T) {
	entered := make(chan struct{})
	released := make(chan struct{})
	client := protocolClient(t, func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(released) })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		client.RunSession(ctx, podRequest("forward"), session.Streams{Ready: func(uint16) {}})
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handshake not started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handshake ignored cancel")
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("server handshake left open")
	}
}
