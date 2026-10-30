package localstore

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigDirectory uses the new product name for new installations, while reusing
// an existing legacy directory without copying credentials or losing preferences.
func ConfigDirectory(root, override, legacyOverride string) (string, error) {
	if override != "" {
		return override, nil
	}
	if legacyOverride != "" {
		return legacyOverride, nil
	}
	for _, name := range []string{"cereals", "creals"} {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("读取应用配置目录: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("应用配置路径不是目录: %s", path)
		}
		return path, nil
	}
	return filepath.Join(root, "cereals"), nil
}
