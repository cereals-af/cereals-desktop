package cluster

import (
	"os"
	"strings"
	"testing"
)

func singleConfig() string {
	return strings.Replace(configFixture, "- name: staging\n  context:\n    cluster: production\n    user: operator\n", "", 1)
}
func TestSingleClusterDefaultPersistsWhenAnotherContextIsAdded(t *testing.T) {
	path := writeConfig(t, singleConfig())
	prefs := &memoryPreferences{}
	sources := &memorySources{paths: []string{path}}
	manager := NewManager(t.Context(), &testFactory{}, sources, prefs)
	defer manager.Close()
	first := manager.State()
	if len(first.Profiles) != 1 || first.DefaultClusterID != first.Profiles[0].ID {
		t.Fatal("single context is not default")
	}
	if err := os.WriteFile(path, []byte(configFixture), 0600); err != nil {
		t.Fatal(err)
	}
	next, err := manager.Import(path)
	if err != nil {
		t.Fatal(err)
	}
	if next.DefaultClusterID != next.Profiles[0].ID || next.DefaultClusterID == first.DefaultClusterID {
		t.Fatal("default did not follow context across fingerprint change")
	}
	if _, err := manager.SetDefault(next.Profiles[1].ID); err != nil {
		t.Fatal(err)
	}
	manager.Close()
	restored := NewManager(t.Context(), &testFactory{}, sources, prefs)
	defer restored.Close()
	if restored.State().DefaultClusterID != next.Profiles[1].ID {
		t.Fatal("selected default was not restored")
	}
}
func TestDefaultApprovalIsBoundToConfigIdentity(t *testing.T) {
	config := strings.Replace(singleConfig(), "token: credential-do-not-expose", "exec:\n      apiVersion: client.authentication.k8s.io/v1\n      command: test-auth-plugin\n      interactiveMode: Never", 1)
	path := writeConfig(t, config)
	prefs := &memoryPreferences{}
	factory := &testFactory{}
	sources := &memorySources{paths: []string{path}}
	manager := NewManager(t.Context(), factory, sources, prefs)
	defer manager.Close()
	state := manager.State()
	if !state.DefaultNeedsConfirmation {
		t.Fatal("new plugin did not require confirmation")
	}
	if _, err := manager.Connect(t.Context(), ConnectRequest{ClusterID: state.DefaultClusterID, TrustExec: true}); err != nil {
		t.Fatal(err)
	}
	if manager.State().DefaultNeedsConfirmation {
		t.Fatal("confirmed default approval not remembered")
	}
	restored := NewManager(t.Context(), factory, sources, prefs)
	defer restored.Close()
	if restored.State().DefaultNeedsConfirmation {
		t.Fatal("approval not restored")
	}
	if err := os.WriteFile(path, []byte(strings.Replace(config, "test-auth-plugin", "changed-auth-plugin", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	next, err := restored.Import(path)
	if err != nil {
		t.Fatal(err)
	}
	if !next.DefaultNeedsConfirmation || next.DefaultClusterID == state.DefaultClusterID {
		t.Fatal("changed config reused old approval")
	}
}
func TestInvalidDefaultAndPersistenceFailureKeepSelection(t *testing.T) {
	path := writeConfig(t, configFixture)
	prefs := &memoryPreferences{}
	manager := NewManager(t.Context(), &testFactory{}, &memorySources{paths: []string{path}}, prefs)
	defer manager.Close()
	state := manager.State()
	if state.DefaultClusterID != "" {
		t.Fatal("multiple contexts picked arbitrary default")
	}
	if _, err := manager.SetDefault(state.Profiles[0].ID); err != nil {
		t.Fatal(err)
	}
	prefs.fail = true
	if _, err := manager.SetDefault(state.Profiles[1].ID); err == nil {
		t.Fatal("save failure ignored")
	}
	if _, err := manager.SetDefault("missing"); err == nil {
		t.Fatal("invalid cluster accepted")
	}
	if manager.State().DefaultClusterID != state.Profiles[0].ID {
		t.Fatal("failed write changed default")
	}
}
