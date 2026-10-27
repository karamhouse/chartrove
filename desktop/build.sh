#!/bin/sh
set -eu
desktop_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(dirname "$desktop_root")
cd "$repo_root/frontend"
# Install the locked frontend dependencies separately; builds do not refresh them.
if [ ! -x node_modules/.bin/ng ]; then
  echo 'Install frontend dependencies with corepack yarn install --frozen-lockfile first.' >&2
  exit 1
fi
case "${1:-}" in
  '') node node_modules/@angular/cli/bin/ng.js build --configuration desktop_prod --output-path ../dist/frontend ;;
  --skip-frontend) test -f "$repo_root/dist/frontend/index.html" ;;
  *) echo 'Usage: desktop/build.sh [--skip-frontend]' >&2; exit 2 ;;
esac
mkdir -p "$desktop_root/assets/frontend"
rsync -a --delete --exclude .gitkeep --exclude .DS_Store "$repo_root/dist/frontend/" "$desktop_root/assets/frontend/"
build_root=$(mktemp -d "${TMPDIR:-/tmp}/chartrove-build.XXXXXX")
trap 'rm -rf "$build_root"' EXIT HUP INT TERM
bundle="$build_root/Chartrove.app"
mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"
cp "$desktop_root/Info.plist" "$bundle/Contents/Info.plist"
swift "$desktop_root/build-icon.swift" "$build_root/Chartrove.iconset"
iconutil -c icns "$build_root/Chartrove.iconset" -o "$bundle/Contents/Resources/Chartrove.icns"
cd "$desktop_root"
CGO_ENABLED=1 CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=13.0" CGO_CXXFLAGS="${CGO_CXXFLAGS:-} -mmacosx-version-min=13.0" \
  go build -tags production -ldflags '-extldflags=-mmacosx-version-min=13.0' -o "$bundle/Contents/MacOS/chartrove" ./cmd/fasten-desktop
# Sign outside File Provider managed folders, which can attach Finder metadata
# to .app directories during signing. The retained ZIP preserves the signature.
codesign --force --sign - "$bundle"
codesign --verify --strict "$bundle"
ditto -c -k --norsrc --keepParent "$bundle" "$repo_root/dist/Chartrove.zip"
echo "Built $repo_root/dist/Chartrove.zip"
