package appupdate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

type fakeEngine struct {
	release          *updater.Release
	state            updater.State
	checks, installs int
	entered          chan struct{}
}

func (e *fakeEngine) Check(context.Context) (*updater.Release, error) {
	e.checks++
	return e.release, nil
}
func (e *fakeEngine) State() updater.State { return e.state }
func (e *fakeEngine) CheckAndInstall(ctx context.Context) error {
	e.installs++
	if e.entered != nil {
		close(e.entered)
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func TestAutomaticChecksRemainQuietUnlessNewRelease(t *testing.T) {
	e := &fakeEngine{}
	s := NewService(context.Background(), e, true, func(error) {})
	defer s.cancel()
	if err := s.check(false); err != nil || e.installs != 0 {
		t.Fatal("opened window without an update")
	}
	e.release = &updater.Release{Version: "1.1.0"}
	if err := s.check(false); err != nil || e.installs != 1 {
		t.Fatal("did not stage available update")
	}
	e.state = updater.StateReady
	if err := s.check(false); err != nil || e.installs != 1 || e.checks != 2 {
		t.Fatal("rechecked staged update")
	}
}

func TestManualCheckCancellationAndConcurrentCalls(t *testing.T) {
	e := &fakeEngine{entered: make(chan struct{})}
	s := NewService(context.Background(), e, true, func(error) {})
	if err := s.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- s.CheckForUpdates() }()
	<-e.entered
	if err := s.CheckForUpdates(); err == nil {
		t.Fatal("allowed overlapping update")
	}
	done := make(chan struct{})
	go func() { _ = s.ServiceShutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel update")
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if e.installs != 1 {
		t.Fatalf("installs=%d", e.installs)
	}
}

func TestDevelopmentBuildDoesNotUpdate(t *testing.T) {
	e := &fakeEngine{}
	s := NewService(context.Background(), e, false, func(error) {})
	defer s.cancel()
	if err := s.CheckForUpdates(); err == nil || e.installs != 0 || e.checks != 0 {
		t.Fatal("development executable can self-update")
	}
}
