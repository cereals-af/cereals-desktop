// Command update-checksums creates the sidecar consumed by the Wails GitHub provider.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	dir := flag.String("dir", "bin/updates", "directory containing this release's update payloads")
	version := flag.String("version", "", "release version (required)")
	flag.Parse()
	if err := run(*dir, *version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(dir, version string) error {
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) {
		return fmt.Errorf("invalid release version %q", version)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	// Reject leftovers instead of producing a checksum list for mixed versions.
	pattern := regexp.MustCompile(`^cereals-` + regexp.QuoteMeta(version) + `-update-(darwin-(arm64|amd64)\.zip|windows-(amd64|arm64)\.exe)$`)
	var sums strings.Builder
	for _, entry := range entries {
		if entry.Name() == "SHA256SUMS" {
			continue
		}
		if !entry.Type().IsRegular() || !pattern.MatchString(entry.Name()) {
			return fmt.Errorf("unexpected update payload %s; use a clean directory for version %s", entry.Name(), version)
		}
		digest, err := checksum(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		fmt.Fprintf(&sums, "%x  %s\n", digest, entry.Name())
	}
	if sums.Len() == 0 {
		return fmt.Errorf("no update payloads for %s", version)
	}
	return os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums.String()), 0644)
}

func checksum(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
