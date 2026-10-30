package session

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestLogArchiveSnapshotBoundaryAndClose(t *testing.T) {
	more := make(chan struct{})
	service, _ := setup(t, func(ctx context.Context, _ Request, streams Streams) (Outcome, error) {
		streams.Ready(0)
		if _, err := io.WriteString(streams.Output, "first\n"); err != nil {
			return Outcome{}, err
		}
		select {
		case <-more:
		case <-ctx.Done():
			return Outcome{}, ctx.Err()
		}
		_, err := io.WriteString(streams.Output, "later\n")
		return Outcome{}, err
	})
	r := request()
	r.Kind = "logs"
	r.TailLines = 500
	r.Follow = true
	id, err := service.Start(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	update, err := service.Poll(t.Context(), id, 0)
	if err != nil || update.Cursor != 6 {
		t.Fatalf("%+v %v", update, err)
	}
	snapshot, err := service.OpenLog(id)
	if err != nil {
		t.Fatal(err)
	}
	close(more)
	for !update.Done {
		update, err = service.Poll(t.Context(), id, update.Cursor)
		if err != nil {
			t.Fatal(err)
		}
	}
	full, err := service.OpenLog(id)
	if err != nil {
		t.Fatal(err)
	}
	op, _ := service.get(id)
	service.Close(id)
	if op.owner.Err() == nil {
		t.Fatal("closing did not cancel transfer/export owner")
	}
	data, err := io.ReadAll(snapshot.Reader)
	if err != nil || string(data) != "first\n" {
		t.Fatalf("snapshot boundary changed %q %v", data, err)
	}
	data, err = io.ReadAll(full.Reader)
	if err != nil || string(data) != "first\nlater\n" {
		t.Fatalf("completed archive missing bytes %q %v", data, err)
	}
	snapshot.Close()
	full.Close()
	full.Close()
	if op.archive.file != nil {
		t.Fatal("snapshot references leaked archive")
	}
}

func TestLogArchiveLimitRetainsAcceptedBytes(t *testing.T) {
	a, err := newLogArchive()
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	a.limit = 5
	if _, err = a.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if n, err := a.Write([]byte("later")); n != 0 || err != errArchiveFull {
		t.Fatalf("limit n=%d err=%v", n, err)
	}
	data := make([]byte, 5)
	if _, err = a.file.ReadAt(data, 0); err != nil || string(data) != "hello" {
		t.Fatalf("archive changed %q %v", data, err)
	}
}

func TestCompletedLogsRetainLeaseUntilViewCloses(t *testing.T) {
	service, _ := setup(t, func(_ context.Context, _ Request, streams Streams) (Outcome, error) {
		_, err := io.WriteString(streams.Output, "finished")
		return Outcome{}, err
	})
	r := request()
	r.Kind = "logs"
	r.TailLines = 100
	id, err := service.Start(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	var update Update
	for !update.Done {
		update, err = service.Poll(t.Context(), id, update.Cursor)
		if err != nil {
			t.Fatal(err)
		}
	}
	service.mu.Lock()
	service.sessions[id].touched = time.Now().Add(-time.Minute)
	service.mu.Unlock()
	if err = service.Touch(id); err != nil {
		t.Fatal(err)
	}
	service.SetBackground(true)
	service.expire(time.Now().Add(time.Hour))
	snapshot, err := service.OpenLog(id)
	if err != nil {
		t.Fatal("hidden completed logs were reclaimed", err)
	}
	snapshot.Close()
	service.SetBackground(false)
	service.expire(time.Now().Add(time.Minute))
	if _, err = service.OpenLog(id); err == nil {
		t.Fatal("abandoned archive was not reclaimed")
	}
}

func TestArchiveFailureEndsStreamWithActionableMessage(t *testing.T) {
	service, _ := setup(t, func(ctx context.Context, _ Request, streams Streams) (Outcome, error) {
		streams.Ready(0)
		_, err := io.Copy(streams.Output, strings.NewReader(strings.Repeat("x", 100)))
		return Outcome{}, err
	})
	// Start behind a gate so the bound can be lowered without racing the producer.
	client := service.resolver.(testResolver).client
	run := client.run
	gate := make(chan struct{})
	client.run = func(ctx context.Context, r Request, streams Streams) (Outcome, error) {
		<-gate
		return run(ctx, r, streams)
	}
	r := request()
	r.Kind = "logs"
	r.TailLines = 100
	id, err := service.Start(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	op, _ := service.get(id)
	op.archive.limit = 10
	close(gate)
	var update Update
	for !update.Done {
		update, err = service.Poll(t.Context(), id, update.Cursor)
		if err != nil {
			t.Fatal(err)
		}
	}
	if update.State != "stopped" || !strings.Contains(update.Message, "归档已达到") {
		t.Fatalf("%+v", update)
	}
}
