package appupdate

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

type engine interface {
	Check(context.Context) (*updater.Release, error)
	CheckAndInstall(context.Context) error
	State() updater.State
}

// Service serializes manual and automatic flows. Background checks stay silent
// unless a new release is found; Wails owns download, verification and restart UI.
type Service struct {
	ctx     context.Context
	cancel  context.CancelFunc
	engine  engine
	enabled bool
	mu      sync.Mutex
	done    chan struct{}
	report  func(error)
}

func NewService(ctx context.Context, engine engine, enabled bool, report func(error)) *Service {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{ctx: ctx, cancel: cancel, engine: engine, enabled: enabled, done: make(chan struct{}), report: report}
}

func (s *Service) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	go s.poll()
	return nil
}

func (s *Service) poll() {
	defer close(s.done)
	if !s.enabled {
		return
	}
	// Let the main window settle before the first check. Later checks are bounded
	// to one every six hours and are cancelled together with application shutdown.
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
			if err := s.check(false); err != nil && !errors.Is(err, context.Canceled) {
				s.report(err)
			}
			timer.Reset(6 * time.Hour)
		}
	}
}

// CheckForUpdates opens the Wails update window. Installation is staged only;
// replacing the running app requires the user's Restart & Apply action.
func (s *Service) CheckForUpdates() error { return s.check(true) }

func (s *Service) check(manual bool) error {
	if !s.enabled {
		return fmt.Errorf("自动更新仅支持 macOS/Windows 发行构建；macOS 请从 cereals.app 启动")
	}
	if !s.mu.TryLock() {
		if manual {
			return fmt.Errorf("正在检查或下载更新，请稍候")
		}
		return nil
	}
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if !manual {
		if s.engine.State() == updater.StateReady {
			return nil
		}
		release, err := s.engine.Check(s.ctx)
		if err != nil || release == nil {
			return err
		}
	}
	// The public Wails API opens its window through CheckAndInstall. A background
	// hit is rechecked there before downloading, including skip/version handling.
	return s.engine.CheckAndInstall(s.ctx)
}

func (s *Service) ServiceShutdown() error {
	s.cancel()
	<-s.done
	// A manual binding/menu invocation may still be unwinding its network I/O.
	s.mu.Lock()
	s.mu.Unlock()
	return nil
}
