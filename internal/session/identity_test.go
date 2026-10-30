package session

import (
	"context"
	"errors"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"testing"
	"time"
)

type identityClient struct {
	Client
	lookup func() (Pod, error)
	done   <-chan struct{}
}

func (c identityClient) Pod(context.Context, resource.Ref) (Pod, error) { return c.lookup() }
func (c identityClient) SessionDone() <-chan struct{}                   { return c.done }
func TestIdentityMonitorStopsReplacementAndDeletion(t *testing.T) {
	for _, test := range []struct {
		name  string
		pod   Pod
		err   error
		stops bool
	}{{"replacement", Pod{UID: "new"}, nil, true}, {"deletion", Pod{}, ErrPodGone, true}, {"transient", Pod{}, errors.New("network"), false}, {"same", Pod{UID: "pod-1"}, nil, false}} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			op := &operation{ctx: ctx, cancel: cancel, request: request()}
			done := make(chan struct{})
			exited := make(chan struct{})
			client := identityClient{lookup: func() (Pod, error) { return test.pod, test.err }, done: make(chan struct{})}
			go func() { defer close(exited); monitorPod(op, client, done, time.Millisecond) }()
			select {
			case <-ctx.Done():
				if !test.stops {
					t.Fatal("transient lookup interrupted session")
				}
			case <-time.After(30 * time.Millisecond):
				if test.stops {
					t.Fatal("old session remained alive")
				}
			}
			close(done)
			select {
			case <-exited:
			case <-time.After(time.Second):
				t.Fatal("monitor leaked")
			}
		})
	}
}
