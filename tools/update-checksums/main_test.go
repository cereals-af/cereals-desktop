package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChecksumsAndMixedVersionRejection(t *testing.T) {
	dir := t.TempDir()
	name := "cereals-1.0.0-update-windows-amd64.exe"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(dir, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad  "+name+"\n" {
		t.Fatalf("checksum=%s", data)
	}
	if err := os.WriteFile(filepath.Join(dir, strings.Replace(name, "1.0.0", "0.9.0", 1)), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(dir, "1.0.0"); err == nil {
		t.Fatal("accepted mixed versions")
	}
	unchanged, _ := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if string(unchanged) != string(data) {
		t.Fatal("modified checksum file after failed validation")
	}
}

func TestRejectsEmptyAndInvalidVersion(t *testing.T) {
	for _, version := range []string{"", "../1.0.0", "1.0.0"} {
		if err := run(t.TempDir(), version); err == nil {
			t.Fatalf("accepted empty release %q", version)
		}
	}
}
