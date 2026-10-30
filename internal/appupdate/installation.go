package appupdate

import (
	"path/filepath"
	"strings"
)

// SupportsInstallation prevents a macOS .app archive from replacing a standalone
// binary. Wails swaps the enclosing bundle only when the executable is inside it.
func SupportsInstallation(platform, executable string) bool {
	if platform == "windows" {
		return true
	}
	if platform != "darwin" {
		return false
	}
	macos := filepath.Dir(executable)
	contents := filepath.Dir(macos)
	bundle := filepath.Dir(contents)
	return filepath.Base(macos) == "MacOS" && filepath.Base(contents) == "Contents" && strings.HasSuffix(bundle, ".app")
}
