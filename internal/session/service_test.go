package session

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type testClient struct {
	lifetime context.Context
	run      func(context.Context, Request, Streams) (Outcome, error)
}

func (c *testClient) ConnectionID() string         { return "connection-1" }
func (c *testClient) SessionDone() <-chan struct{} { return c.lifetime.Done() }
func (c *testClient) Pod(context.Context, resource.Ref) (Pod, error) {
	return Pod{UID: "pod-1", Phase: "Running", Containers: []Container{{Name: "app", State: "running"}}}, nil
}
func (c *testClient) RunSession(ctx context.Context, r Request, s Streams) (Outcome, error) {
	return c.run(ctx, r, s)
}

type testResolver struct{ client *testClient }

func (r testResolver) SessionClient(string) (Client, error) { return r.client, nil }
func request() Request {
	return Request{Target: resource.Ref{ClusterID: "cluster", Version: "v1", Resource: "pods", Namespace: "default", Name: "demo"}, UID: "pod-1", Kind: "exec", Container: "app", Command: []string{"/bin/sh"}, Columns: 80, Rows: 24}
}
func setup(t *testing.T, run func(context.Context, Request, Streams) (Outcome, error)) (*Service, context.CancelFunc) {
	t.Helper()
	lifetime, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	service := NewService(t.Context(), testResolver{&testClient{lifetime, run}})
	t.Cleanup(func() { service.ServiceShutdown() })
	return service, cancel
}
func TestOutputAcknowledgementBackpressureAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	buffer := newOutput(ctx)
	payload := bytes.Repeat([]byte("a"), outputLimit)
	if _, err := buffer.Write(payload); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { _, err := buffer.Write([]byte("b")); finished <- err }()
	first, _, err := buffer.read(0)
	if err != nil {
		t.Fatal(err)
	}
	replay, _, _ := buffer.read(0)
	if replay.Data != first.Data || replay.Cursor != first.Cursor {
		t.Fatal("poll must replay unacknowledged bytes")
	}
	select {
	case <-finished:
		t.Fatal("producer exceeded bound without acknowledgement")
	case <-time.After(20 * time.Millisecond):
	}
	if _, _, err = buffer.read(first.Cursor); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("ack did not unblock producer")
	}
	if _, _, err = buffer.read(0); err == nil {
		t.Fatal("accepted stale cursor")
	}
	if _, _, err = buffer.read(uint64(outputLimit * 2)); err == nil {
		t.Fatal("accepted cursor beyond delivered bytes")
	}
	cancel()
	if _, err = buffer.Write([]byte("c")); err == nil {
		t.Fatal("write ignored cancellation")
	}
}
func TestSessionIOAndConnectionDisconnect(t *testing.T) {
	observed := make(chan string, 1)
	service, disconnect := setup(t, func(ctx context.Context, _ Request, streams Streams) (Outcome, error) {
		streams.Ready(0)
		input := make([]byte, 32)
		n, err := streams.Input.Read(input)
		if err != nil {
			return Outcome{-1}, err
		}
		observed <- string(input[:n])
		_, err = streams.Output.Write([]byte("你好\r\n"))
		if err != nil {
			return Outcome{-1}, err
		}
		<-ctx.Done()
		return Outcome{-1}, ctx.Err()
	})
	id, err := service.Start(t.Context(), request())
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Input(id, base64.StdEncoding.EncodeToString([]byte("echo test\n"))); err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-observed:
		if value != "echo test\n" {
			t.Fatal(value)
		}
	case <-time.After(time.Second):
		t.Fatal("input not delivered")
	}
	update, err := service.Poll(t.Context(), id, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := base64.StdEncoding.DecodeString(update.Data)
	if string(data) != "你好\r\n" {
		t.Fatalf("output %q", data)
	}
	disconnect()
	deadline, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	for {
		update, err = service.Poll(deadline, id, update.Cursor)
		if err != nil {
			t.Fatal(err)
		}
		if update.Done {
			break
		}
	}
	if update.State != "stopped" {
		t.Fatal(update)
	}
	if err = service.Input(id, "YQ=="); err == nil {
		t.Fatal("input accepted after disconnect")
	}
	service.Close(id)
	if _, err = service.Poll(t.Context(), id, update.Cursor); err == nil {
		t.Fatal("closed session still found")
	}
}
func TestValidationAndBoundedSessions(t *testing.T) {
	service, _ := setup(t, func(ctx context.Context, _ Request, _ Streams) (Outcome, error) {
		<-ctx.Done()
		return Outcome{-1}, ctx.Err()
	})
	cases := []Request{request(), request(), request(), request(), request()}
	cases[0].UID = "recreated"
	cases[1].Container = "missing"
	cases[2].Kind = "unknown"
	cases[3].Columns = 0
	cases[4].Command = []string{strings.Repeat("x", 4097)}
	for _, r := range cases {
		if _, err := service.Start(t.Context(), r); err == nil {
			t.Fatal("accepted invalid request", r)
		}
	}
	for range 16 {
		if _, err := service.Start(t.Context(), request()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Start(t.Context(), request()); err == nil {
		t.Fatal("accepted excess session")
	}
}
func TestShutdownUnblocksFullOutputAndInput(t *testing.T) {
	service, _ := setup(t, func(_ context.Context, _ Request, streams Streams) (Outcome, error) {
		_, err := io.Copy(streams.Output, bytes.NewReader(bytes.Repeat([]byte("x"), 2*outputLimit)))
		return Outcome{-1}, err
	})
	if _, err := service.Start(t.Context(), request()); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { service.ServiceShutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked on full buffer")
	}
}
func TestInputLimitAndResizeCoalescing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	buffer := newInput(ctx)
	if err := buffer.send(make([]byte, inputLimit)); err != nil {
		t.Fatal(err)
	}
	if err := buffer.send([]byte("x")); err == nil {
		t.Fatal("input limit ignored")
	}
	cancel()
	if _, err := buffer.Read(make([]byte, 1)); err == nil {
		t.Fatal("input read ignored cancellation")
	}
	service, _ := setup(t, func(ctx context.Context, _ Request, _ Streams) (Outcome, error) {
		<-ctx.Done()
		return Outcome{-1}, ctx.Err()
	})
	id, err := service.Start(t.Context(), request())
	if err != nil {
		t.Fatal(err)
	}
	for i := uint16(80); i < 150; i++ {
		if err := service.Resize(id, Size{i, 30}); err != nil {
			t.Fatal(err)
		}
	}
	op, _ := service.get(id)
	if size := <-op.sizes; size.Columns != 149 {
		t.Fatal(size)
	}
}
