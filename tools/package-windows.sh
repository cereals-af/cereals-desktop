#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
root="$PWD"
version=$(node -p "require('./frontend/package.json').version")
output="$root/bin/installers"
updates="$root/bin/updates"
mkdir -p "$output" "$updates"
resource="$root/installer_windows_amd64.syso"
if [ -e "$resource" ]; then
  printf 'Refusing to replace existing resource: %s\n' "$resource" >&2
  exit 1
fi
staging=$(mktemp -d "$root/bin/.windows-installer.XXXXXX")
trap 'rm -f "$resource"; rm -rf "$staging"' EXIT

node --input-type=module - "$staging/info.json" <<'JS'
import { readFileSync, writeFileSync } from 'node:fs';
const { version } = JSON.parse(readFileSync('frontend/package.json', 'utf8'));
writeFileSync(process.argv[2], JSON.stringify({
  fixed: { file_version: version, product_version: version },
  info: { '0000': {
    ProductVersion: version,
    CompanyName: 'cereals',
    FileDescription: 'cereals Kubernetes workspace',
    ProductName: 'cereals',
  } },
}, null, 2));
JS

wails3 generate syso -arch amd64 -icon build/windows/icon.ico \
  -manifest build/windows/wails.exe.manifest -info "$staging/info.json" -out "$resource"
env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -tags production -trimpath -buildvcs=false \
  -ldflags="-s -w -H windowsgui -X github.com/cereals-af/cereals-desktop/internal/buildinfo.Version=$version" \
  -o "$staging/cereals.exe" .
wails3 generate webview2bootstrapper -dir "$staging"

installer="$output/cereals-$version-windows-amd64-setup.exe"
makensis -V3 -DPRODUCT_VERSION="$version" -DAPP_BINARY="$staging/cereals.exe" \
  -DWEBVIEW_BOOTSTRAPPER="$staging/MicrosoftEdgeWebview2Setup.exe" \
  -DINSTALLER_OUTPUT="$installer" build/windows/nsis/project.nsi
cp "$staging/cereals.exe" "$root/bin/cereals-windows-amd64.exe"
cp "$staging/cereals.exe" "$updates/cereals-$version-update-windows-amd64.exe"
cd "$output"
shasum -a 256 "${installer##*/}" > "${installer##*/}.sha256"
printf 'Created %s\n' "$installer"
