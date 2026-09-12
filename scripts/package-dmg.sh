#!/usr/bin/env bash
# scripts/package-dmg.sh — wraps an already-built xchats.app (wails build)
# into xchats-desktop-macos-universal.dmg: the .app plus a symlink to
# /Applications, so opening the mounted image and dragging one onto the
# other is the whole install — the conventional macOS flow (see
# docs/desktop.md). hdiutil is a macOS system tool; nothing else to install.
#
# Usage: package-dmg.sh <app-bundle-path> <output-dmg-path>
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <app-bundle-path> <output-dmg-path>" >&2
  exit 1
fi

APP="$1"
OUT="$2"

[ -d "$APP" ] || { echo "package-dmg.sh: app bundle not found: $APP" >&2; exit 1; }
case "$APP" in
  *.app) ;;
  *) echo "package-dmg.sh: expected a .app bundle, got: $APP" >&2; exit 1 ;;
esac

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

# ditto (not cp -R) preserves the bundle's symlinks, resource forks and
# extended attributes exactly — the same tool desktop-build.yml's own zip
# packaging step already uses for this bundle.
ditto "$APP" "$STAGE/$(basename "$APP")"
ln -s /Applications "$STAGE/Applications"

mkdir -p "$(dirname "$OUT")"
rm -f "$OUT"
STAGE_MB="$(du -sm "$STAGE" | cut -f1)"
STAGE_SIZE_MB="$((STAGE_MB + 128))"
hdiutil create -volname "xchats" -srcfolder "$STAGE" -ov -format UDZO -size "${STAGE_SIZE_MB}m" "$OUT"
hdiutil imageinfo "$OUT" | head -5
