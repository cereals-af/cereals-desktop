package filetransfer

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/session"
)

type testOwner struct{ ctx context.Context }

func (s testOwner) FileTarget(string) (session.FileTarget, error) {
	return session.FileTarget{Context: s.ctx, ConnectionID: "connection", Request: session.Request{Target: resource.Ref{ClusterID: "cluster", Version: "v1", Resource: "pods", Namespace: "ns", Name: "pod"}, UID: "uid", Kind: "exec", Container: "app"}}, nil
}

type testDialogs struct{ upload, save string }

func (d *testDialogs) PickUpload() (string, error)     { return d.upload, nil }
func (d *testDialogs) SaveFile(string) (string, error) { return d.save, nil }

type testClient struct {
	ctx context.Context
	uid string
	run func(context.Context, []string, io.Reader, io.Writer) error
}

func (c *testClient) ConnectionID() string         { return "connection" }
func (c *testClient) SessionDone() <-chan struct{} { return c.ctx.Done() }
func (c *testClient) Pod(ctx context.Context, _ resource.Ref) (session.Pod, error) {
	if err := ctx.Err(); err != nil {
		return session.Pod{}, err
	}
	return session.Pod{UID: c.uid, Phase: "Running", Containers: []session.Container{{Name: "app", State: "running"}}}, nil
}
func (c *testClient) FileExec(ctx context.Context, _ session.Request, command []string, input io.Reader, output io.Writer) error {
	return c.run(ctx, command, input, output)
}

type testResolver struct{ client *testClient }

func (r testResolver) FileClient(string) (Client, error) { return r.client, nil }

func localExec(ctx context.Context, command []string, input io.Reader, output io.Writer) error {
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = append(os.Environ(), "COPYFILE_DISABLE=1")
	cmd.Stdin = input
	cmd.Stdout = output
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 17 {
		return ErrExists
	}
	return err
}
func fixture(t *testing.T) (*Service, *testDialogs, *testClient, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	d := &testDialogs{save: filepath.Join(t.TempDir(), "saved.bin")}
	c := &testClient{ctx: t.Context(), uid: "uid", run: localExec}
	s := NewService(t.Context(), testOwner{ctx}, testResolver{c}, d)
	t.Cleanup(func() { s.ServiceShutdown() })
	return s, d, c, cancel
}
func await(t *testing.T, s *Service, id string, condition func(Progress) bool) Progress {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		value, err := s.Poll(id)
		if err != nil {
			t.Fatal(err)
		}
		if condition(value) {
			return value
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("transfer did not reach expected state")
	return Progress{}
}
func waitDone(t *testing.T, s *Service, id string) Progress {
	return await(t, s, id, func(p Progress) bool { return p.Done })
}

func TestFileRoundTripAndExplicitOverwrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX container script exercised on Unix hosts")
	}
	s, d, _, _ := fixture(t)
	d.upload = filepath.Join(t.TempDir(), "report $(no-command).bin")
	data := append([]byte{0, 1, 255, 13, 10}, []byte("中文内容")...)
	if err := os.WriteFile(d.upload, data, 0600); err != nil {
		t.Fatal(err)
	}
	remote := filepath.Join(t.TempDir(), "directory with ' quotes")
	if err := os.Mkdir(remote, 0700); err != nil {
		t.Fatal(err)
	}
	selection, err := s.SelectUpload(t.Context(), "terminal-1")
	if err != nil {
		t.Fatal(err)
	}
	r := Request{SessionID: "terminal-1", Direction: "upload", SelectionID: selection.ID, RemotePath: remote}
	for index, overwrite := range []bool{false, false, true} {
		r.Overwrite = overwrite
		id, err := s.Start(t.Context(), r)
		if err != nil {
			t.Fatal(err)
		}
		p := waitDone(t, s, id)
		expected := "completed"
		if index == 1 {
			expected = "conflict"
		}
		if p.State != expected {
			t.Fatalf("%+v", p)
		}
		if overwrite && p.State != "completed" {
			t.Fatal("confirmed overwrite failed")
		}
		got, _ := os.ReadFile(filepath.Join(remote, selection.Name))
		if !bytes.Equal(got, data) {
			t.Fatal("upload changed bytes")
		}
		s.Close(id)
	}
	id, err := s.Start(t.Context(), Request{SessionID: "terminal-2", Direction: "download", RemotePath: filepath.Join(remote, selection.Name)})
	if err != nil {
		t.Fatal(err)
	}
	if p := waitDone(t, s, id); p.State != "completed" || p.Bytes != int64(len(data)) {
		t.Fatalf("%+v", p)
	}
	got, _ := os.ReadFile(d.save)
	if !bytes.Equal(got, data) {
		t.Fatal("binary download corrupted")
	}
	entries, _ := os.ReadDir(remote)
	if len(entries) != 1 {
		t.Fatal("remote staging file leaked", entries)
	}
}

func TestDownloadRejectsUnsafeArchiveAndPreservesExistingFile(t *testing.T) {
	for _, kind := range []string{"traversal", "symlink", "oversize", "extra", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			s, d, c, _ := fixture(t)
			if err := os.WriteFile(d.save, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			c.run = func(_ context.Context, _ []string, _ io.Reader, out io.Writer) error {
				a := tar.NewWriter(out)
				h := &tar.Header{Name: "./support.log", Mode: 0600, Size: 5, Typeflag: tar.TypeReg}
				switch kind {
				case "traversal":
					h.Name = "../outside"
				case "symlink":
					h.Typeflag = tar.TypeSymlink
					h.Size = 0
					h.Linkname = "/etc/passwd"
				case "oversize":
					h.Size = MaxFileSize + 1
				}
				if err := a.WriteHeader(h); err != nil {
					return err
				}
				if kind == "truncated" {
					_, err := out.Write([]byte("x"))
					return err
				}
				if _, err := a.Write([]byte("hello")); err != nil {
					return err
				}
				if kind == "extra" {
					if err := a.WriteHeader(&tar.Header{Name: "other", Typeflag: tar.TypeReg}); err != nil {
						return err
					}
				}
				return a.Close()
			}
			id, err := s.Start(t.Context(), Request{SessionID: "terminal", Direction: "download", RemotePath: "/tmp/support.log"})
			if err != nil {
				t.Fatal(err)
			}
			if p := waitDone(t, s, id); p.State != "error" {
				t.Fatalf("unsafe archive accepted %+v", p)
			}
			got, _ := os.ReadFile(d.save)
			if string(got) != "original" {
				t.Fatal("existing file overwritten on failure")
			}
			entries, _ := os.ReadDir(filepath.Dir(d.save))
			if len(entries) != 1 {
				t.Fatal("partial files leaked", entries)
			}
		})
	}
}

func TestClosingOwnerCancelsDownloadAndRetainsOriginal(t *testing.T) {
	s, d, c, closeOwner := fixture(t)
	if err := os.WriteFile(d.save, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	c.run = func(ctx context.Context, _ []string, _ io.Reader, out io.Writer) error {
		a := tar.NewWriter(out)
		if err := a.WriteHeader(&tar.Header{Name: "./support.log", Size: 100, Mode: 0600, Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err := a.Write([]byte("partial")); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	}
	id, err := s.Start(t.Context(), Request{SessionID: "terminal", Direction: "download", RemotePath: "/tmp/support.log"})
	if err != nil {
		t.Fatal(err)
	}
	await(t, s, id, func(p Progress) bool { return p.Bytes > 0 })
	closeOwner()
	if p := waitDone(t, s, id); p.State != "cancelled" {
		t.Fatalf("%+v", p)
	}
	got, _ := os.ReadFile(d.save)
	if string(got) != "original" {
		t.Fatal("cancel overwrote original")
	}
}

func TestSelectionAndPodIdentityAreValidatedBeforeTransfer(t *testing.T) {
	s, d, c, _ := fixture(t)
	d.upload = filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(d.upload, []byte("original"), 0600)
	selection, err := s.SelectUpload(t.Context(), "terminal")
	if err != nil {
		t.Fatal(err)
	}
	r := Request{SessionID: "other-terminal", Direction: "upload", SelectionID: selection.ID, RemotePath: "/tmp"}
	if _, err = s.Start(t.Context(), r); err == nil {
		t.Fatal("accepted another session's selection")
	}
	r.SessionID = "terminal"
	os.WriteFile(d.upload, []byte("changed-size"), 0600)
	id, err := s.Start(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if p := waitDone(t, s, id); p.State != "error" || !strings.Contains(p.Message, "本地文件已变化") {
		t.Fatalf("%+v", p)
	}
	s.Close(id)
	c.uid = "replacement"
	c.run = func(context.Context, []string, io.Reader, io.Writer) error {
		t.Error("opened stream to replacement")
		return nil
	}
	id, err = s.Start(t.Context(), Request{SessionID: "terminal", Direction: "download", RemotePath: "/tmp/support.log"})
	if err != nil {
		t.Fatal(err)
	}
	if p := waitDone(t, s, id); p.State != "error" || !strings.Contains(p.Message, "重建") {
		t.Fatalf("%+v", p)
	}
}
