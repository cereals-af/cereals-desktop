#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
root="$PWD"
version=$(node -p "require('./frontend/package.json').version")
arch=$(go env GOHOSTARCH)
output="$root/bin/installers"
updates="$root/bin/updates"
mkdir -p "$output" "$updates"
staging=$(mktemp -d "$root/bin/.macos-installer.XXXXXX")
trap 'rm -rf "$staging"' EXIT

env GOOS=darwin GOARCH="$arch" CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=15.0 \
  go build -tags production -trimpath -buildvcs=false \
  -ldflags="-s -w -X github.com/cereals-af/cereals-desktop/internal/buildinfo.Version=$version -extldflags=-mmacosx-version-min=15.0" \
  -o "$staging/cereals" .
go run ./tools/package-macos -binary "$staging/cereals" \
  -out "$staging/volume/cereals.app" -version "$version"

# Ad-hoc signing is required for arm64 integrity; it is not Developer ID signing.
codesign --force --sign - --timestamp=none "$staging/volume/cereals.app"
codesign --verify --deep --strict "$staging/volume/cereals.app"
# Wails replaces the entire bundle, including Info.plist and its code signature.
ditto -c -k --sequesterRsrc --keepParent "$staging/volume/cereals.app" \
  "$updates/cereals-$version-update-darwin-$arch.zip"
ln -s /Applications "$staging/volume/Applications"
printf '%s\n' \
  '将 cereals 拖到 Applications 即可安装。' \
  '更新前请从托盘选择“退出 cereals”，关闭窗口仅会隐藏应用。' \
  '适用于 macOS 15.0 及以上。此构建使用本地 ad-hoc 签名，未进行 Developer ID 签名或 Apple 公证。' \
  > "$staging/volume/安装说明.txt"

dmg="$output/cereals-$version-macos-$arch.dmg"
hdiutil create -volname "cereals $version" -srcfolder "$staging/volume" \
  -format UDZO -ov "$dmg"
hdiutil verify "$dmg"
cd "$output"
shasum -a 256 "${dmg##*/}" > "${dmg##*/}.sha256"
printf 'Created %s\n' "$dmg"
