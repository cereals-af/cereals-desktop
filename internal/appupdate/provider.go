// Package appupdate integrates the Wails updater with cereals releases.
package appupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const Repository = "cereals-af/cereals-desktop"

// NewProvider uses only dedicated update payloads, never a DMG or NSIS installer.
func NewProvider() (updater.Provider, error) {
	p, err := github.New(github.Config{
		Repository: Repository, ChecksumAsset: "SHA256SUMS",
		AssetMatcher: matchAsset, HTTPClient: &http.Client{Timeout: 15 * time.Minute},
	})
	if err != nil {
		return nil, err
	}
	return verifiedProvider{p}, nil
}

var updateAsset = regexp.MustCompile(`^cereals-[0-9]+\.[0-9]+\.[0-9]+-update-(darwin|windows)-(arm64|amd64)\.(zip|exe)$`)

func matchAsset(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	for i, asset := range assets {
		parts := updateAsset.FindStringSubmatch(asset.Name)
		if len(parts) != 4 || parts[1] != req.Platform || parts[2] != req.Arch {
			continue
		}
		if (req.Platform == "darwin" && parts[3] == "zip") || (req.Platform == "windows" && parts[3] == "exe") {
			return i
		}
	}
	return -1
}

// Wails' GitHub provider treats a missing checksum sidecar/entry as optional.
// cereals requires it, so an incomplete release cannot install unchecked bytes.
type verifiedProvider struct{ updater.Provider }

func (p verifiedProvider) Check(ctx context.Context, req updater.CheckRequest) (*updater.Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	release, err := p.Provider.Check(ctx, req)
	if err != nil || release == nil {
		return release, err
	}
	v := release.Verification
	if v == nil || v.DigestAlgo != "sha256" || len(v.Digest) != sha256.Size {
		return nil, fmt.Errorf("更新附件 %s 缺少有效的 SHA256SUMS 校验值", release.Artifact.Filename)
	}
	return release, nil
}
