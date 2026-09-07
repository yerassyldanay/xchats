#!/usr/bin/env bash
# scripts/package-deb.sh — builds xchats-desktop-linux-amd64.deb from an
# already-built `wails build` binary. Conventional system paths on purpose
# (see docs/desktop.md): /usr/bin/xchats, a .desktop entry, and a hicolor
# icon, so the app appears in the Ubuntu application menu and launches
# without a terminal — the portable tar.gz remains the arbitrary-install-
# location option for anyone who wants that instead.
#
# Runtime deps (libgtk-3-0, libwebkit2gtk-4.1-0) are declared, not vendored —
# same runtime libraries the tar.gz build has always needed from the distro
# (see docs/desktop.md's prerequisites table), just now enforced by apt
# instead of discovered at first launch.
#
# Usage: package-deb.sh <version> <binary-path> <icon-path> <output-deb-path>
#   version:    Debian policy version string (a bare "1.2.3", no leading "v")
#   binary-path: the built xchats executable (wails build's output)
#   icon-path:  a square PNG (1024x1024 in this repo's own build/appicon.png)
#   output-deb-path: where to write the .deb
set -euo pipefail

if [ "$#" -ne 4 ]; then
  echo "usage: $0 <version> <binary-path> <icon-path> <output-deb-path>" >&2
  exit 1
fi

VERSION="$1"
BINARY="$2"
ICON="$3"
OUT="$4"

if [[ ! "$VERSION" =~ ^[0-9] ]]; then
  echo "package-deb.sh: version must start with a digit per Debian policy, got: $VERSION" >&2
  exit 1
fi
[ -f "$BINARY" ] || { echo "package-deb.sh: binary not found: $BINARY" >&2; exit 1; }
[ -f "$ICON" ] || { echo "package-deb.sh: icon not found: $ICON" >&2; exit 1; }

PKGROOT="$(mktemp -d)"
trap 'rm -rf "$PKGROOT"' EXIT

install -d -m 0755 "$PKGROOT/DEBIAN"
install -d -m 0755 "$PKGROOT/usr/bin"
install -d -m 0755 "$PKGROOT/usr/share/applications"
install -d -m 0755 "$PKGROOT/usr/share/icons/hicolor/1024x1024/apps"
install -d -m 0755 "$PKGROOT/usr/share/doc/xchats"

install -m 0755 "$BINARY" "$PKGROOT/usr/bin/xchats"
install -m 0644 "$ICON" "$PKGROOT/usr/share/icons/hicolor/1024x1024/apps/xchats.png"

cat > "$PKGROOT/usr/share/applications/xchats.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=xchats
Comment=AI chat assistant for WhatsApp, Telegram, Instagram and Messenger
Exec=/usr/bin/xchats
Icon=xchats
Categories=Network;Chat;InstantMessaging;
Terminal=false
StartupNotify=true
EOF
chmod 0644 "$PKGROOT/usr/share/applications/xchats.desktop"

INSTALLED_SIZE="$(du -sk "$PKGROOT/usr" | cut -f1)"

cat > "$PKGROOT/DEBIAN/control" <<EOF
Package: xchats
Version: $VERSION
Section: net
Priority: optional
Architecture: amd64
Installed-Size: $INSTALLED_SIZE
Depends: libgtk-3-0, libwebkit2gtk-4.1-0
Maintainer: xchats project <https://github.com/yerassyldanay/xchats>
Homepage: https://github.com/yerassyldanay/xchats
Description: AI chat assistant for WhatsApp, Telegram, Instagram and Messenger
 xchats is a free, open-source AI assistant that connects to multiple social
 media platforms and eliminates hallucinations using a strict template-based
 response pattern.
EOF

cat > "$PKGROOT/usr/share/doc/xchats/copyright" <<'EOF'
Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: xchats
Source: https://github.com/yerassyldanay/xchats

Files: *
License: AGPL-3.0-or-later
 Licensed under the GNU Affero General Public License v3.0 or later. See
 https://github.com/yerassyldanay/xchats/blob/master/LICENSE for the full
 text, and THIRD_PARTY_NOTICES.md for the licenses of every dependency
 compiled or bundled into this binary.
EOF
chmod 0644 "$PKGROOT/usr/share/doc/xchats/copyright"

mkdir -p "$(dirname "$OUT")"
dpkg-deb --build --root-owner-group "$PKGROOT" "$OUT"
dpkg-deb --info "$OUT"
