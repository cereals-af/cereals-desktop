package appupdate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func TestMatchOnlyUpdatePayloads(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "cereals-1.1.0-windows-amd64-setup.exe"},
		{Name: "cereals-1.1.0-macos-arm64.dmg"},
		{Name: "cereals-1.1.0-update-darwin-arm64.zip.sha256"},
		{Name: "cereals-1.1.0-update-darwin-arm64.zip"},
		{Name: "cereals-1.1.0-update-windows-amd64.exe"},
		{Name: "cereals-1.1.0-update-darwin-amd64.exe"},
	}
	for _, tc := range []struct {
		platform, arch string
		want           int
	}{
		{"darwin", "arm64", 3}, {"windows", "amd64", 4},
		{"darwin", "amd64", -1}, {"linux", "amd64", -1}, {"windows", "arm64", -1},
	} {
		if got := matchAsset(updater.CheckRequest{Platform: tc.platform, Arch: tc.arch}, assets); got != tc.want {
			t.Errorf("%s/%s matched %d, want %d", tc.platform, tc.arch, got, tc.want)
		}
	}
}

type headlessHost struct{}

func (headlessHost) Emit(string, ...any) bool                              { return true }
func (headlessHost) OnEvent(string, func(any)) func()                      { return func() {} }
func (headlessHost) OpenWindow(updater.WindowOptions) updater.WindowHandle { return nil }
func (headlessHost) Quit()                                                 {}

// Exercise the actual Wails download and verification path without replacing
// the test executable. Restart/helper swaps require native platform acceptance.
func TestWailsRejectsCorruptDownload(t *testing.T) {
	const name = "cereals-1.1.0-update-windows-amd64.exe"
	digest := sha256.Sum256([]byte("expected release bytes"))
	for _, payload := range []string{"expected release bytes", "corrupt release bytes"} {
		t.Run(payload, func(t *testing.T) {
			var base string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/sums":
					fmt.Fprintf(w, "%x  %s\n", digest, name)
				case "/payload":
					fmt.Fprint(w, payload)
				default:
					if err := json.NewEncoder(w).Encode(map[string]any{
						"tag_name": "v1.1.0", "assets": []map[string]any{
							{"name": name, "browser_download_url": base + "/payload"},
							{"name": "SHA256SUMS", "browser_download_url": base + "/sums"},
						},
					}); err != nil {
						t.Error(err)
					}
				}
			}))
			defer server.Close()
			base = server.URL
			p, err := github.New(github.Config{Repository: Repository, BaseURL: base, HTTPClient: server.Client(), ChecksumAsset: "SHA256SUMS", AssetMatcher: matchAsset})
			if err != nil {
				t.Fatal(err)
			}
			u := updater.New(headlessHost{})
			if err := u.Init(updater.Config{CurrentVersion: "1.0.0", Platform: "windows", Arch: "amd64", Providers: []updater.Provider{verifiedProvider{p}}, Window: updater.WindowNone}); err != nil {
				t.Fatal(err)
			}
			err = u.CheckAndInstall(context.Background())
			if payload == "corrupt release bytes" {
				if err == nil || u.State() != updater.StateError || u.DownloadedPath() != "" {
					t.Fatalf("corrupt download accepted: state=%s error=%v", u.State(), err)
				}
				return
			}
			if err != nil || u.State() != updater.StateReady {
				t.Fatalf("state=%s error=%v", u.State(), err)
			}
			path := u.DownloadedPath()
			defer os.RemoveAll(filepath.Dir(path))
			data, err := os.ReadFile(path)
			if err != nil || string(data) != payload {
				t.Fatalf("staged bytes=%q error=%v", data, err)
			}
		})
	}
}

func TestGitHubReleaseContract(t *testing.T) {
	const name = "cereals-1.1.0-update-windows-amd64.exe"
	digest := sha256.Sum256([]byte("payload"))
	for _, tc := range []struct {
		name, sums, current    string
		asset                  bool
		wantRelease, wantError bool
	}{
		{"valid", fmt.Sprintf("%x  %s\n", digest, name), "1.0.0", true, true, false},
		{"missing sidecar", "", "1.0.0", false, false, true},
		{"missing entry", fmt.Sprintf("%x  other.exe\n", digest), "1.0.0", true, false, true},
		{"malformed digest", "bad  " + name, "1.0.0", true, false, true},
		{"same version", "", "1.1.0", false, false, false},
		{"older version", "", "2.0.0", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var base string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/sums" {
					fmt.Fprint(w, tc.sums)
					return
				}
				assets := []map[string]any{{"name": name, "browser_download_url": base + "/payload"}}
				if tc.asset {
					assets = append(assets, map[string]any{"name": "SHA256SUMS", "browser_download_url": base + "/sums"})
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.1.0", "assets": assets}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			base = server.URL
			p, err := github.New(github.Config{Repository: Repository, BaseURL: base, HTTPClient: server.Client(), ChecksumAsset: "SHA256SUMS", AssetMatcher: matchAsset})
			if err != nil {
				t.Fatal(err)
			}
			release, err := (verifiedProvider{p}).Check(context.Background(), updater.CheckRequest{CurrentVersion: tc.current, Platform: "windows", Arch: "amd64"})
			if (err != nil) != tc.wantError || (release != nil) != tc.wantRelease {
				t.Fatalf("release=%v error=%v", release, err)
			}
		})
	}
}
