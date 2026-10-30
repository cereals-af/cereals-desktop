package observe

import (
	"context"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type testWatcher struct {
	started chan func(Change)
	stopped chan struct{}
}

func (*testWatcher) ConnectionID() string { return "connection-a" }
func (c *testWatcher) Observe(ctx context.Context, _ resource.Query, _ string, notify func(Change)) error {
	c.started <- notify
	<-ctx.Done()
	close(c.stopped)
	return ctx.Err()
}
func (c *testWatcher) WatchClient(string) (Client, error) { return c, nil }

func TestSubscriptionsShareStreamCoalesceAndRelease(t *testing.T) {
	c := &testWatcher{started: make(chan func(Change), 2), stopped: make(chan struct{})}
	s := NewService(t.Context(), c)
	t.Cleanup(func() { _ = s.ServiceShutdown() })
	query := resource.Query{ClusterID: "a", Version: "v1", Resource: "pods", Namespace: "team", LabelSelector: "app=one"}
	first, err := s.Subscribe(query, "10")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Subscribe(query, "11")
	if err != nil {
		t.Fatal(err)
	}
	var notify func(Change)
	select {
	case notify = <-c.started:
	case <-time.After(time.Second):
		t.Fatal("watch did not start")
	}
	for range 1000 {
		notify(Change{State: "live", Dirty: true})
	}
	update, err := s.Poll(t.Context(), second, 0)
	if err != nil || update.Changes != 1000 || update.State != "live" {
		t.Fatalf("bad coalesced update: %+v, %v", update, err)
	}
	select {
	case <-c.started:
		t.Fatal("duplicate upstream watch")
	default:
	}
	s.Unsubscribe(first)
	select {
	case <-c.stopped:
		t.Fatal("remaining subscriber lost watch")
	default:
	}
	s.Unsubscribe(second)
	select {
	case <-c.stopped:
	case <-time.After(time.Second):
		t.Fatal("last subscriber leaked watch")
	}
	if _, err := s.Poll(t.Context(), second, 0); err == nil {
		t.Fatal("removed subscription remained usable")
	}
}
