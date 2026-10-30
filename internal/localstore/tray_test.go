package localstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrayPreferenceDefaultAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings", "tray.json")
	p := NewTrayPreferences(path, true)
	if value, err := p.Load(); err != nil || !value {
		t.Fatalf("missing settings: %v, %v", value, err)
	}
	if err := p.Save(false); err != nil {
		t.Fatal(err)
	}
	if value, err := NewTrayPreferences(path, true).Load(); err != nil || value {
		t.Fatalf("saved false lost on restart: %v, %v", value, err)
	}
	if err := p.Save(true); err != nil {
		t.Fatal(err)
	}
	if value, err := NewTrayPreferences(path, false).Load(); err != nil || !value {
		t.Fatalf("saved true lost on restart: %v, %v", value, err)
	}
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if value, err := p.Load(); err == nil || !value {
		t.Fatal("corrupt settings must report an error and retain platform default")
	}
}
