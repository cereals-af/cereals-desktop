package localstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/cereals-af/cereals-desktop/internal/cluster"
)

// ConnectionPreferences stores only a context reference and an approval fingerprint.
type ConnectionPreferences struct{ path string }

func NewConnectionPreferences(path string) *ConnectionPreferences {
	return &ConnectionPreferences{path: path}
}

func (p *ConnectionPreferences) Load() (cluster.Preference, error) {
	data, err := os.ReadFile(p.path)
	if errors.Is(err, os.ErrNotExist) {
		return cluster.Preference{}, nil
	}
	if err != nil {
		return cluster.Preference{}, err
	}
	var preference cluster.Preference
	if err := json.Unmarshal(data, &preference); err != nil {
		return cluster.Preference{}, err
	}
	return preference, nil
}

func (p *ConnectionPreferences) Save(preference cluster.Preference) error {
	data, err := json.MarshalIndent(preference, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(p.path), ".connection-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), p.path)
}
