//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

// controlPlane starts only test-owned loopback processes. It never loads kubeconfig.
func controlPlane(t *testing.T) *rest.Config { return controlPlaneOptions(t) }

func controlPlaneOptions(t *testing.T, extra ...string) *rest.Config {
	t.Helper()
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" {
		t.Skip("set KUBEBUILDER_ASSETS to the official envtest binary directory")
	}
	dir := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(dir, "service-account.key")
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		t.Fatal(err)
	}
	token := uuid.NewString()
	tokenFile := filepath.Join(dir, "tokens.csv")
	if err := os.WriteFile(tokenFile, []byte(token+",creals-test,test-admin,system:masters\n"), 0600); err != nil {
		t.Fatal(err)
	}
	etcdPort, peerPort, apiPort := freePort(t), freePort(t), freePort(t)
	etcdURL := fmt.Sprintf("http://127.0.0.1:%d", etcdPort)
	peerURL := fmt.Sprintf("http://127.0.0.1:%d", peerPort)
	startProcess(t, filepath.Join(assets, "etcd"), dir,
		"--data-dir="+filepath.Join(dir, "data"), "--listen-client-urls="+etcdURL, "--advertise-client-urls="+etcdURL,
		"--listen-peer-urls="+peerURL, "--initial-advertise-peer-urls="+peerURL, "--initial-cluster=default="+peerURL)
	args := []string{
		"--etcd-servers=" + etcdURL, "--bind-address=127.0.0.1", "--advertise-address=127.0.0.1", fmt.Sprintf("--secure-port=%d", apiPort),
		"--cert-dir=" + dir, "--service-cluster-ip-range=10.240.0.0/24", "--service-account-issuer=https://creals.test",
		"--service-account-signing-key-file=" + keyFile, "--service-account-key-file=" + keyFile,
		"--token-auth-file=" + tokenFile, "--authorization-mode=RBAC", "--anonymous-auth=false", "--disable-admission-plugins=ServiceAccount"}
	startProcess(t, filepath.Join(assets, "kube-apiserver"), dir, append(args, extra...)...)
	config := &rest.Config{Host: fmt.Sprintf("https://127.0.0.1:%d", apiPort), BearerToken: token, TLSClientConfig: rest.TLSClientConfig{Insecure: true}, Timeout: 2 * time.Second, QPS: 100, Burst: 100}
	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	for ctx.Err() == nil {
		if err := client.RESTClient().Get().AbsPath("/readyz").Do(ctx).Error(); err == nil {
			config.Timeout = 0
			return config
		}
		select {
		case <-ctx.Done():
		case <-time.After(200 * time.Millisecond):
		}
	}
	log, _ := os.ReadFile(filepath.Join(dir, "kube-apiserver.log"))
	t.Fatalf("test API server did not become ready: %s", log)
	return nil
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func startProcess(t *testing.T, path, dir string, args ...string) {
	t.Helper()
	log, err := os.Create(filepath.Join(dir, filepath.Base(path)+".log"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(path, args...)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = command.Process.Kill()
			<-done
		}
		_ = log.Close()
	})
}
