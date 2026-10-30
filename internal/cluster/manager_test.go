package cluster

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/client-go/rest"
)

const configFixture = `apiVersion: v1
kind: Config
clusters:
- name: production
  cluster:
    server: https://192.0.2.10:6443
    certificate-authority: certificates/ca.crt
contexts:
- name: production
  context:
    cluster: production
    user: operator
    namespace: platform
- name: staging
  context:
    cluster: production
    user: operator
users:
- name: operator
  user:
    token: credential-do-not-expose
`

type memorySources struct{ paths []string }

func (s *memorySources) List() ([]string, error) { return s.paths, nil }
func (s *memorySources) Add(path string) error   { s.paths = append(s.paths, path); return nil }

type testFactory struct{ calls atomic.Int32 }

func (f *testFactory) New(ctx context.Context, config *rest.Config) (resource.Client, error) {
	f.calls.Add(1)
	return &testClient{ctx: ctx, host: config.Host}, nil
}

type testClient struct {
	ctx  context.Context
	host string
}

func (c *testClient) Version(context.Context) (string, error) { return "v1.36.2", c.ctx.Err() }
func (c *testClient) Discover(context.Context) (resource.Catalog, error) {
	return resource.Catalog{}, nil
}
func (c *testClient) List(context.Context, resource.Query) (resource.Page, error) {
	return resource.Page{}, nil
}
func (c *testClient) YAML(context.Context, resource.Ref) (string, error) { return "", nil }
func (c *testClient) Close()                                             {}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	// The manager resolves and checks paths; these tests inject a fake transport.
	if err := os.MkdirAll(filepath.Join(filepath.Dir(path), "certificates"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "certificates", "ca.crt"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportPreservesContextsResolvesPathsAndDoesNotExposeCredentials(t *testing.T) {
	path := writeConfig(t, configFixture)
	manager := NewManager(t.Context(), &testFactory{}, &memorySources{}, &memoryPreferences{})
	defer manager.Close()
	state, err := manager.Import(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Profiles) != 2 {
		t.Fatalf("profiles = %d", len(state.Profiles))
	}
	if state.Profiles[1].Namespace != "default" {
		t.Fatal("missing default namespace")
	}
	item := manager.entries[state.Profiles[0].ID]
	if want := filepath.Join(filepath.Dir(path), "certificates", "ca.crt"); item.config.Clusters["production"].CertificateAuthority != want {
		t.Fatal("relative CA path was not resolved")
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "credential-do-not-expose") {
		t.Fatal("credential exposed to frontend")
	}
}

func TestUntrustedExecNeverReachesClientFactory(t *testing.T) {
	config := strings.Replace(configFixture, "token: credential-do-not-expose", "exec:\n      apiVersion: client.authentication.k8s.io/v1\n      command: test-auth-plugin\n      interactiveMode: Never", 1)
	factory := &testFactory{}
	manager := NewManager(t.Context(), factory, &memorySources{}, &memoryPreferences{})
	defer manager.Close()
	state, err := manager.Import(writeConfig(t, config))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Connect(t.Context(), ConnectRequest{ClusterID: state.Profiles[0].ID})
	if err == nil || factory.calls.Load() != 0 {
		t.Fatal("untrusted exec config reached connection path")
	}
}

func TestConnectionsAreIsolatedAndDisconnectCancelsOnlyItsSession(t *testing.T) {
	manager := NewManager(t.Context(), &testFactory{}, &memorySources{}, &memoryPreferences{})
	defer manager.Close()
	state, err := manager.Import(writeConfig(t, configFixture))
	if err != nil {
		t.Fatal(err)
	}
	a, b := state.Profiles[0].ID, state.Profiles[1].ID
	for _, id := range []string{a, b} {
		if _, err := manager.Connect(t.Context(), ConnectRequest{ClusterID: id}); err != nil {
			t.Fatal(err)
		}
	}
	clientA, _ := manager.Client(a)
	clientB, _ := manager.Client(b)
	manager.Disconnect(a)
	if _, err := manager.Client(a); err == nil {
		t.Fatal("disconnected client is still available")
	}
	if clientA.(*testClient).ctx.Err() == nil {
		t.Fatal("disconnected session was not canceled")
	}
	if clientB.(*testClient).ctx.Err() != nil {
		t.Fatal("unrelated session was canceled")
	}
}

func TestImportInvalidatesOldSessionAndChangedCredentialIdentity(t *testing.T) {
	path := writeConfig(t, configFixture)
	manager := NewManager(t.Context(), &testFactory{}, &memorySources{}, &memoryPreferences{})
	defer manager.Close()
	state, err := manager.Import(path)
	if err != nil {
		t.Fatal(err)
	}
	oldID := state.Profiles[0].ID
	if _, err := manager.Connect(t.Context(), ConnectRequest{ClusterID: oldID}); err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(configFixture, "credential-do-not-expose", "changed-credential", 1)
	if err := os.WriteFile(path, []byte(updated), 0600); err != nil {
		t.Fatal(err)
	}
	state, err = manager.Import(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Profiles[0].ID == oldID {
		t.Fatal("credential changes reused old identity")
	}
	if _, err := manager.Client(oldID); err == nil {
		t.Fatal("old session survived reimport")
	}
}

func TestSuccessfulSessionOutlivesConnectRequest(t *testing.T) {
	manager := NewManager(t.Context(), &testFactory{}, &memorySources{}, &memoryPreferences{})
	defer manager.Close()
	state, err := manager.Import(writeConfig(t, configFixture))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	id := state.Profiles[0].ID
	if _, err := manager.Connect(ctx, ConnectRequest{ClusterID: id}); err != nil {
		t.Fatal(err)
	}
	cancel()
	client, err := manager.Client(id)
	if err != nil {
		t.Fatal(err)
	}
	if client.(*testClient).ctx.Err() != nil {
		t.Fatal("request cancellation killed the established session")
	}
}

func TestImportRejectsServerCredentialsAndRedactsParseErrors(t *testing.T) {
	for _, config := range []string{
		strings.Replace(configFixture, "https://192.0.2.10:6443", "https://secret-user:secret-token@192.0.2.10:6443", 1),
		"apiVersion: [secret-credential\n",
	} {
		_, err := loadEntries(writeConfig(t, config))
		if err == nil {
			t.Fatal("invalid config was accepted")
		}
		if strings.Contains(err.Error(), "secret-") {
			t.Fatal("credential-bearing input leaked in error")
		}
	}
}

type memoryPreferences struct {
	value Preference
	fail  bool
}

func (p *memoryPreferences) Load() (Preference, error) { return p.value, nil }
func (p *memoryPreferences) Save(value Preference) error {
	if p.fail {
		return os.ErrPermission
	}
	p.value = value
	return nil
}
