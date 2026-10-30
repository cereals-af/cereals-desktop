package observe

import (
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

func TestHiddenWindowRetainsWatchAndResumeRestoresLease(t *testing.T) {
	client := &testWatcher{started: make(chan func(Change), 1), stopped: make(chan struct{})}
	s := NewService(t.Context(), client)
	t.Cleanup(func() { _ = s.ServiceShutdown() })
	id, err := s.Subscribe(resource.Query{ClusterID: "a", Version: "v1", Resource: "pods"}, "10")
	if err != nil {
		t.Fatal(err)
	}
	s.SetBackground(true)
	s.expire(time.Now().Add(time.Hour))
	s.mu.Lock()
	_, retained := s.subs[id]
	s.mu.Unlock()
	if !retained {
		t.Fatal("hidden window lost its watch")
	}
	s.SetBackground(false)
	s.expire(time.Now().Add(30 * time.Second))
	s.mu.Lock()
	_, retained = s.subs[id]
	s.mu.Unlock()
	if !retained {
		t.Fatal("resume did not grant a new lease")
	}
	s.expire(time.Now().Add(time.Minute))
	select {
	case <-client.stopped:
	case <-time.After(time.Second):
		t.Fatal("abandoned foreground watch was not stopped")
	}
}
