package localstore

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSourcePathsSurviveRestartWithoutDuplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creals", "sources.json")
	store := NewSources(path)
	for range 2 {
		if err := store.Add("/example/kube/config"); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := NewSources(path).List()
	if err != nil || len(paths) != 1 || paths[0] != "/example/kube/config" {
		t.Fatalf("paths = %v, err = %v", paths, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("unsafe permissions: %v", info.Mode())
	}
}
