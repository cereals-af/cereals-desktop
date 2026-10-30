// Command package-macos assembles a local, unsigned app bundle without requiring Xcode.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

const plist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>cereals</string>
<key>CFBundleIdentifier</key><string>cn.creals.desktop</string>
<key>CFBundleName</key><string>cereals</string>
<key>CFBundleDisplayName</key><string>cereals</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleIconFile</key><string>icon.icns</string>
<key>CFBundleShortVersionString</key><string>%s</string>
<key>CFBundleVersion</key><string>%s</string>
<key>LSMinimumSystemVersion</key><string>15.0</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
`

func main() {
	binary := flag.String("binary", "bin/cereals", "compiled macOS executable")
	output := flag.String("out", "bin/cereals.app", "destination app bundle")
	version := flag.String("version", "1.0.0", "three-part application version")
	flag.Parse()
	if err := run(*binary, *output, *version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(binary, output, version string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("macOS packaging must run on macOS")
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) {
		return fmt.Errorf("invalid application version %q", version)
	}
	contents := filepath.Join(output, "Contents")
	if err := os.MkdirAll(filepath.Join(contents, "MacOS"), 0755); err != nil {
		return err
	}
	if err := copyFile(binary, filepath.Join(contents, "MacOS", "cereals"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(fmt.Sprintf(plist, version, version)), 0644); err != nil {
		return err
	}
	resources := filepath.Join(contents, "Resources")
	if err := os.MkdirAll(resources, 0755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join("build", "darwin", "icon.icns"), filepath.Join(resources, "icon.icns"), 0644); err != nil {
		return err
	}
	fmt.Printf("Created %s (local, unsigned build)\n", output)
	return nil
}

// Replace files atomically so packaging never modifies a running executable in place.
func copyFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.CreateTemp(filepath.Dir(destination), ".package-*")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	if err := output.Chmod(mode); err != nil {
		output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Rename(output.Name(), destination)
}
