package localstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDirectoryCompatibility(t *testing.T) {
	root := t.TempDir()
	check := func(override, legacy, want string) {
		t.Helper()
		got, err := ConfigDirectory(root, override, legacy)
		if err != nil || got != want {
			t.Fatalf("ConfigDirectory = %q, %v; want %q", got, err, want)
		}
	}
	check("", "", filepath.Join(root, "cereals"))
	old := filepath.Join(root, "creals")
	if err := os.Mkdir(old, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "sources.json"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	check("", "", old)
	current := filepath.Join(root, "cereals")
	if err := os.Mkdir(current, 0700); err != nil {
		t.Fatal(err)
	}
	check("", "", current)
	check("", "/legacy-override", "/legacy-override")
	check("/new-override", "/legacy-override", "/new-override")
	data, err := os.ReadFile(filepath.Join(old, "sources.json"))
	if err != nil || string(data) != "preserve" {
		t.Fatal("legacy configuration changed")
	}
}

func TestConfigDirectoryRejectsFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cereals"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfigDirectory(root, "", ""); err == nil {
		t.Fatal("accepted a file as the configuration directory")
	}
}
