package session

import (
	"context"
	"testing"
	"time"
)

func TestHiddenWindowRetainsSessionButResumeRestoresExpiry(t *testing.T) {
	service, _ := setup(t, func(ctx context.Context, _ Request, _ Streams) (Outcome, error) {
		<-ctx.Done()
		return Outcome{}, ctx.Err()
	})
	id, err := service.Start(t.Context(), request())
	if err != nil {
		t.Fatal(err)
	}
	service.SetBackground(true)
	service.expire(time.Now().Add(time.Hour))
	if _, err := service.get(id); err != nil {
		t.Fatal("hidden window lost its session", err)
	}
	service.SetBackground(false)
	service.expire(time.Now().Add(30 * time.Second))
	if _, err := service.get(id); err != nil {
		t.Fatal("resume did not grant a fresh lease", err)
	}
	service.expire(time.Now().Add(time.Minute))
	if _, err := service.get(id); err == nil {
		t.Fatal("abandoned foreground session was not reclaimed")
	}
}

func TestHiddenSessionStillStopsOnDisconnect(t *testing.T) {
	stopped := make(chan struct{})
	service, disconnect := setup(t, func(ctx context.Context, _ Request, _ Streams) (Outcome, error) {
		<-ctx.Done()
		close(stopped)
		return Outcome{}, ctx.Err()
	})
	if _, err := service.Start(t.Context(), request()); err != nil {
		t.Fatal(err)
	}
	service.SetBackground(true)
	disconnect()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("background mode kept a disconnected session alive")
	}
}

func TestHideDoesNotRescueAlreadyAbandonedSession(t *testing.T) {
	service, _ := setup(t, func(ctx context.Context, _ Request, _ Streams) (Outcome, error) {
		<-ctx.Done()
		return Outcome{}, ctx.Err()
	})
	id, err := service.Start(t.Context(), request())
	if err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	service.sessions[id].touched = time.Now().Add(-time.Minute)
	service.mu.Unlock()
	service.SetBackground(true)
	if _, err := service.get(id); err == nil {
		t.Fatal("hide retained an already abandoned session")
	}
}
