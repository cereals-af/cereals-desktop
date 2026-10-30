package desktop

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sync/atomic"
)

type shellWindow interface {
	Show()
	Hide()
}

type closePreference interface {
	Load() (bool, error)
	Save(bool) error
}

// Shell owns window visibility without destroying the webview or its drafts.
// Window mutations are dispatched on the native UI thread by its adapter.
type Shell struct {
	window      shellWindow
	preferences closePreference
	background  func(bool)
	quit        func()
	closeToTray atomic.Bool
	stopping    atomic.Bool
}

func (s *Shell) Show() {
	if s.stopping.Load() {
		return
	}
	s.background(false)
	s.window.Show()
}

func (s *Shell) Hide() {
	if s.stopping.Load() {
		return
	}
	s.background(true)
	s.window.Hide()
}

func (s *Shell) closeRequested() {
	if s.closeToTray.Load() {
		s.Hide()
	} else {
		s.Quit()
	}
}

func (s *Shell) setCloseToTray(value bool) error {
	if err := s.preferences.Save(value); err != nil {
		return err
	}
	s.closeToTray.Store(value)
	return nil
}

func (s *Shell) Quit() {
	if s.stopping.CompareAndSwap(false, true) {
		s.quit()
	}
}

// InstanceID isolates development/test config directories from the user's app.
func InstanceID(configDir string) string {
	abs, err := filepath.Abs(configDir)
	if err == nil {
		configDir = abs
	}
	sum := sha256.Sum256([]byte(filepath.Clean(configDir)))
	return fmt.Sprintf("cn.creals.desktop.%x", sum[:12])
}
