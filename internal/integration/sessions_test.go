//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/filetransfer"
	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/session"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	streamspdy "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Exercise real API-server authorization and WebSocket-to-SPDY proxying against
// a test-owned kubelet protocol endpoint. No container runtime is simulated here.
func TestAPIServerPodSessions(t *testing.T) {
	kubelet := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/containerLogs/"):
			if r.URL.Query().Get("previous") != "true" {
				t.Error("previous logs option missing")
			}
			io.WriteString(w, "previous container log\n")
		case strings.HasPrefix(r.URL.Path, "/exec/"):
			if strings.Contains(r.URL.RawQuery, "creals-file-test") {
				serveFileExec(t, w, r)
			} else {
				serveExec(t, w, r)
			}
		case strings.HasPrefix(r.URL.Path, "/portForward/"):
			servePortForward(t, w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer kubelet.Close()
	certFile := filepath.Join(t.TempDir(), "kubelet-ca.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: kubelet.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	config := controlPlaneOptions(t, "--kubelet-certificate-authority="+certFile)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	core, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	port := kubelet.Listener.Addr().(*net.TCPAddr).Port
	node, err := core.CoreV1().Nodes().Create(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "session-node"}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	node.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "127.0.0.1"}}
	node.Status.DaemonEndpoints.KubeletEndpoint.Port = int32(port)
	if _, err = core.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	pod, err := core.CoreV1().Pods("default").Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "session-pod"}, Spec: corev1.PodSpec{NodeName: node.Name, Containers: []corev1.Container{{Name: "app", Image: "test.invalid/image"}}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod.Status.Phase = corev1.PodRunning
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", Image: "test.invalid/image", ImageID: "test", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()}}}}
	if _, err = core.CoreV1().Pods("default").UpdateStatus(ctx, pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	connection, err := (kube.Factory{}).New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := connection.(session.Client)
	target := resource.Ref{ClusterID: "test", Version: "v1", Resource: "pods", Namespace: "default", Name: pod.Name}
	request := session.Request{Target: target, UID: string(pod.UID), Kind: "logs", Container: "app", TailLines: 100, Previous: true}
	var output bytes.Buffer
	if _, err = client.RunSession(ctx, request, session.Streams{Output: &output, Ready: func(uint16) {}}); err != nil {
		t.Fatal(err)
	}
	if output.String() != "previous container log\n" {
		t.Fatal(output.String())
	}
	request.Kind = "exec"
	request.Command = []string{"/bin/sh"}
	output.Reset()
	sizes := make(chan session.Size, 1)
	sizes <- session.Size{Columns: 100, Rows: 30}
	outcome, err := client.RunSession(ctx, request, session.Streams{Input: strings.NewReader("hello\n"), Output: &output, Sizes: sizes, Ready: func(uint16) {}})
	if err != nil || outcome.ExitCode != 0 || output.String() != "proxied hello\n" {
		t.Fatalf("exec result %v %v %q", outcome, err, output.String())
	}
	fileClient := connection.(filetransfer.Client)
	binary := []byte{0, 13, 10, 255, 128, 42}
	output.Reset()
	if err = fileClient.FileExec(ctx, request, []string{"creals-file-test"}, bytes.NewReader(binary), &output); err != nil || !bytes.Equal(output.Bytes(), binary) {
		t.Fatalf("binary file exec: %v %v", output.Bytes(), err)
	}
	checkProxiedForward(t, ctx, client, request)
	checkSessionRBAC(t, ctx, config, core, request)
}

func checkSessionRBAC(t *testing.T, ctx context.Context, config *rest.Config, core kubernetes.Interface, request session.Request) {
	t.Helper()
	_, err := core.RbacV1().Roles("default").Create(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "session-reader"}, Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = core.RbacV1().RoleBindings("default").Create(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "session-reader"}, Subjects: []rbacv1.Subject{{Kind: "User", Name: "session-reader"}}, RoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "session-reader"}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	restricted := rest.CopyConfig(config)
	restricted.Impersonate = rest.ImpersonationConfig{UserName: "session-reader"}
	connection, err := (kube.Factory{}).New(ctx, restricted)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := connection.(session.Client)
	if _, err = client.Pod(ctx, request.Target); err != nil {
		t.Fatal(err)
	}
	fileClient := connection.(filetransfer.Client)
	if err := fileClient.FileExec(ctx, request, []string{"creals-file-test"}, nil, io.Discard); err == nil || !strings.Contains(err.Error(), "权限不足") {
		t.Fatalf("file exec RBAC: %v", err)
	}
	for _, kind := range []string{"logs", "exec", "forward"} {
		request.Kind = kind
		request.RemotePort = 8080
		_, err = client.RunSession(ctx, request, session.Streams{Input: strings.NewReader(""), Output: io.Discard, Ready: func(uint16) { t.Error("forbidden stream became ready") }})
		if err == nil || !strings.Contains(err.Error(), "权限不足") {
			t.Fatalf("%s RBAC: %v", kind, err)
		}
	}
}

type acceptedStream struct {
	stream httpstream.Stream
	reply  <-chan struct{}
}

func serveExec(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if _, err := httpstream.Handshake(r, w, []string{"v4.channel.k8s.io"}); err != nil {
		return
	}
	received := make(chan acceptedStream, 4)
	connection := streamspdy.NewResponseUpgrader().UpgradeResponse(w, r, func(stream httpstream.Stream, reply <-chan struct{}) error {
		received <- acceptedStream{stream, reply}
		return nil
	})
	if connection == nil {
		return
	}
	defer connection.Close()
	streams := map[string]httpstream.Stream{}
	for len(streams) < 4 {
		select {
		case item := <-received:
			<-item.reply
			streams[item.stream.Headers().Get(corev1.StreamType)] = item.stream
		case <-time.After(5 * time.Second):
			t.Error("exec streams missing")
			return
		}
	}
	input := make([]byte, 6)
	if _, err := io.ReadFull(streams[corev1.StreamTypeStdin], input); err != nil {
		t.Error(err)
		return
	}
	var size struct{ Width, Height uint16 }
	if err := json.NewDecoder(streams[corev1.StreamTypeResize]).Decode(&size); err != nil {
		t.Error(err)
		return
	}
	if size.Width != 100 || size.Height != 30 {
		t.Error("resize", strconv.Itoa(int(size.Width)), size.Height)
	}
	io.WriteString(streams[corev1.StreamTypeStdout], "proxied "+string(input))
	streams[corev1.StreamTypeStdout].Close()
	streams[corev1.StreamTypeStdin].Close()
	io.WriteString(streams[corev1.StreamTypeError], `{"status":"Success"}`)
	streams[corev1.StreamTypeError].Close()
	<-connection.CloseChan()
}

func servePortForward(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if _, err := httpstream.Handshake(r, w, []string{"portforward.k8s.io"}); err != nil {
		return
	}
	received := make(chan httpstream.Stream, 4)
	connection := streamspdy.NewResponseUpgrader().UpgradeResponse(w, r, func(stream httpstream.Stream, reply <-chan struct{}) error { received <- stream; return nil })
	if connection == nil {
		return
	}
	defer connection.Close()
	for {
		select {
		case <-connection.CloseChan():
			return
		case stream := <-received:
			if stream.Headers().Get(corev1.StreamType) == corev1.StreamTypeError {
				stream.Close()
				continue
			}
			// This test opens one connection with one short request.
			data := make([]byte, 4)
			if _, err := io.ReadFull(stream, data); err != nil {
				t.Error(err)
				return
			}
			stream.Write(data)
			stream.Close()
		}
	}
}
func checkProxiedForward(t *testing.T, ctx context.Context, client session.Client, request session.Request) {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	request.Kind = "forward"
	request.RemotePort = 8080
	ready := make(chan uint16, 1)
	done := make(chan error, 1)
	go func() {
		_, err := client.RunSession(ctx, request, session.Streams{Ready: func(port uint16) { ready <- port }})
		done <- err
	}()
	var port uint16
	select {
	case port = <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("proxied forward did not start")
	}
	socket, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	socket.SetDeadline(time.Now().Add(3 * time.Second))
	socket.Write([]byte("ping"))
	result := make([]byte, 4)
	if _, err := io.ReadFull(socket, result); err != nil || string(result) != "ping" {
		t.Fatalf("forward response %q %v", result, err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("forward did not close")
	}
}

// A real API server proxies binary non-TTY Exec streams to this test-owned kubelet endpoint.
func serveFileExec(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if _, err := httpstream.Handshake(r, w, []string{"v4.channel.k8s.io"}); err != nil {
		return
	}
	received := make(chan acceptedStream, 4)
	connection := streamspdy.NewResponseUpgrader().UpgradeResponse(w, r, func(stream httpstream.Stream, reply <-chan struct{}) error {
		received <- acceptedStream{stream, reply}
		return nil
	})
	if connection == nil {
		return
	}
	defer connection.Close()
	streams := map[string]httpstream.Stream{}
	for len(streams) < 4 {
		select {
		case item := <-received:
			<-item.reply
			streams[item.stream.Headers().Get(corev1.StreamType)] = item.stream
		case <-time.After(5 * time.Second):
			t.Error("file streams missing")
			return
		}
	}
	if streams[corev1.StreamTypeResize] != nil || streams[corev1.StreamTypeStderr] == nil {
		t.Error("file transfer used TTY")
		return
	}
	data := make([]byte, 6)
	if _, err := io.ReadFull(streams[corev1.StreamTypeStdin], data); err != nil {
		t.Error(err)
		return
	}
	streams[corev1.StreamTypeStdout].Write(data)
	streams[corev1.StreamTypeStdout].Close()
	io.WriteString(streams[corev1.StreamTypeStderr], "diagnostic")
	streams[corev1.StreamTypeStderr].Close()
	streams[corev1.StreamTypeStdin].Close()
	io.WriteString(streams[corev1.StreamTypeError], `{"status":"Success"}`)
	streams[corev1.StreamTypeError].Close()
	<-connection.CloseChan()
}
