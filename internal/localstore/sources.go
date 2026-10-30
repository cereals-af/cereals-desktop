// Package localstore stores non-secret desktop preferences.
package localstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

type Sources struct {
	mu   sync.Mutex
	path string
}

func NewSources(path string) *Sources { return &Sources{path: path} }

func (s *Sources) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

func (s *Sources) read() ([]string, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	if err := json.Unmarshal(data, &paths); err != nil {
		return nil, err
	}
	return paths, nil
}

func (s *Sources) Add(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, err := s.read()
	if err != nil {
		return err
	}
	if slices.Contains(paths, path) {
		return nil
	}
	data, err := json.MarshalIndent(append(paths, path), "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".sources-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.path)
}
