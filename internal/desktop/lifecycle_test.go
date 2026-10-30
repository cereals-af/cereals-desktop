package desktop

import (
	"errors"
	"testing"
)

type testWindow struct{ shown, hidden int }

func (w *testWindow) Show() { w.shown++ }
func (w *testWindow) Hide() { w.hidden++ }

type testPreference struct {
	value bool
	err   error
}

func (p *testPreference) Load() (bool, error) { return p.value, p.err }
func (p *testPreference) Save(value bool) error {
	if p.err != nil {
		return p.err
	}
	p.value = value
	return nil
}

func TestCloseHidesUntilUserExplicitlyQuits(t *testing.T) {
	window := &testWindow{}
	quit := 0
	background := false
	shell := &Shell{window: window, background: func(value bool) { background = value }, quit: func() { quit++ }}
	shell.closeToTray.Store(true)
	shell.closeRequested()
	if window.hidden != 1 || !background || quit != 0 {
		t.Fatal("close should retain the window and background sessions")
	}
	shell.Show()
	if window.shown != 1 || background {
		t.Fatal("show should resume foreground leases")
	}
	shell.Quit()
	shell.Quit()
	shell.Show()
	shell.Hide()
	if quit != 1 || window.shown != 1 || window.hidden != 1 {
		t.Fatal("shutdown must be final and idempotent")
	}
}

func TestPreferenceFailureDoesNotChangeCloseBehavior(t *testing.T) {
	preference := &testPreference{err: errors.New("read only")}
	quit := false
	shell := &Shell{preferences: preference, quit: func() { quit = true }}
	if err := shell.setCloseToTray(true); err == nil {
		t.Fatal("expected persistence failure")
	}
	shell.closeRequested()
	if !quit {
		t.Fatal("failed save must keep close-to-exit behavior")
	}
}

func TestInstanceIdentityUsesConfigurationDirectory(t *testing.T) {
	if InstanceID("/tmp/creals/a/../test") != InstanceID("/tmp/creals/test") {
		t.Fatal("equivalent directories must share an instance")
	}
	if InstanceID("/tmp/creals/test") == InstanceID("/tmp/creals/user") {
		t.Fatal("test instance must not control the user's app")
	}
}
