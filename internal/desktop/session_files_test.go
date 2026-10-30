package desktop

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/session"
)

type noLogSource struct{}

func (noLogSource) OpenLog(string) (*session.LogSnapshot, error) { return nil, os.ErrNotExist }

type saveStub struct{ path string }

func (s saveStub) SaveFile(string) (string, error) { return s.path, nil }
func TestLogExportMissingSessionDoesNotTouchDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.log")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewSessionFilesService(noLogSource{}, saveStub{path})
	if _, err := s.ExportLogs(t.Context(), "missing"); err == nil {
		t.Fatal("missing session exported")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal("existing destination changed")
	}
}

type snapshotSource struct{ snapshot *session.LogSnapshot }

func (s snapshotSource) OpenLog(string) (*session.LogSnapshot, error) { return s.snapshot, nil }

type saveFunc func(string) (string, error)

func (s saveFunc) SaveFile(name string) (string, error) { return s(name) }

type failedLogReader struct{}

func (failedLogReader) Read([]byte) (int, error) { return 0, errors.New("archive read failed") }

func TestLogExportWritesSnapshotAndPreservesDestinationOnFailure(t *testing.T) {
	for _, scenario := range []string{"saved", "dialog cancelled", "owner closed", "read failed"} {
		t.Run(scenario, func(t *testing.T) {
			directory := t.TempDir()
			destination := filepath.Join(directory, "session.log")
			if err := os.WriteFile(destination, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			owner, cancel := context.WithCancel(t.Context())
			defer cancel()
			const logs = "first\npaused interval\nlast\n"
			var reader io.Reader = strings.NewReader(logs)
			if scenario == "read failed" {
				reader = io.MultiReader(strings.NewReader("partial"), failedLogReader{})
			}
			source := snapshotSource{&session.LogSnapshot{Reader: reader, Bytes: int64(len(logs)), Filename: "suggested.log", Context: owner}}
			picker := saveFunc(func(name string) (string, error) {
				if name != "suggested.log" {
					t.Fatalf("unexpected suggested name %q", name)
				}
				if scenario == "dialog cancelled" {
					return "", nil
				}
				if scenario == "owner closed" {
					cancel()
				}
				return destination, nil
			})
			result, err := NewSessionFilesService(source, picker).ExportLogs(t.Context(), "session")
			want := "original"
			switch scenario {
			case "saved":
				want = logs
				if err != nil || result.Name != "session.log" || result.Bytes != int64(len(logs)) || result.Cancelled {
					t.Fatalf("result=%+v error=%v", result, err)
				}
			case "dialog cancelled":
				if err != nil || !result.Cancelled {
					t.Fatalf("result=%+v error=%v", result, err)
				}
			default:
				if err == nil {
					t.Fatal("failed export reported success")
				}
			}
			contents, err := os.ReadFile(destination)
			if err != nil || string(contents) != want {
				t.Fatalf("contents=%q error=%v", contents, err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary file leaked: %v, %v", entries, err)
			}
		})
	}
}
