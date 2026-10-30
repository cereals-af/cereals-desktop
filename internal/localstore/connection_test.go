package localstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/cluster"
)

func TestConnectionPreferencesRoundTripAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings", "connection.json")
	store := NewConnectionPreferences(path)
	if got, err := store.Load(); err != nil || got.Source != "" {
		t.Fatalf("missing setting: %+v %v", got, err)
	}
	want := cluster.Preference{Source: "/config", Context: "production", ApprovedID: "fingerprint"}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(); err != nil || got != want {
		t.Fatalf("restore: %+v %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("preferences must be private")
	}
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("corruption ignored")
	}
}
