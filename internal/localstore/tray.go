package localstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// TrayPreferences stores the window-close choice separately from kubeconfig sources.
type TrayPreferences struct {
	path         string
	defaultValue bool
}

func NewTrayPreferences(path string, defaultValue bool) *TrayPreferences {
	return &TrayPreferences{path: path, defaultValue: defaultValue}
}

func (p *TrayPreferences) Load() (bool, error) {
	data, err := os.ReadFile(p.path)
	if errors.Is(err, os.ErrNotExist) {
		return p.defaultValue, nil
	}
	if err != nil {
		return p.defaultValue, fmt.Errorf("读取托盘设置: %w", err)
	}
	var settings struct {
		CloseToTray *bool `json:"closeToTray"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return p.defaultValue, fmt.Errorf("解析托盘设置: %w", err)
	}
	if settings.CloseToTray == nil {
		return p.defaultValue, nil
	}
	return *settings.CloseToTray, nil
}

func (p *TrayPreferences) Save(closeToTray bool) error {
	data, err := json.MarshalIndent(struct {
		CloseToTray bool `json:"closeToTray"`
	}{closeToTray}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0700); err != nil {
		return fmt.Errorf("创建托盘设置目录: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(p.path), ".tray-*")
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
