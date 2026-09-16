#!/bin/sh
# Installs the malveon binary for macOS/Linux: detects OS/arch, downloads
# the matching release asset, makes it executable, places it on PATH,
# and (macOS) clears the Gatekeeper quarantine flag so the first run
# doesn't get blocked. Run via:
#   curl -fsSL https://raw.githubusercontent.com/LadsonDavid/beta-test/main/install.sh | sh
set -e

REPO="LadsonDavid/beta-test"
INSTALL_DIR="${MALVEON_INSTALL_DIR:-/usr/local/bin}"

os=$(uname -s)
arch=$(uname -m)

case "$os" in
  Darwin) platform="darwin" ;;
  Linux) platform="linux" ;;
  *)
    echo "malveon: unsupported OS: $os — download a binary manually from" >&2
    echo "  https://github.com/${REPO}/releases" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *)
    echo "malveon: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if [ "$platform" = "linux" ] && [ "$goarch" = "arm64" ]; then
  echo "malveon: linux/arm64 isn't built yet — only linux/amd64 is available" >&2
  echo "  see https://github.com/${REPO}/releases for what's actually shipped" >&2
  exit 1
fi

asset="malveon-${platform}-${goarch}"
url="https://github.com/${REPO}/releases/latest/download/${asset}"

echo "Downloading ${asset}..."
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
if ! curl -fsSL "$url" -o "$tmp"; then
  echo "malveon: download failed — check https://github.com/${REPO}/releases for available builds" >&2
  exit 1
fi
chmod +x "$tmp"

dest="$INSTALL_DIR/malveon"
if [ -w "$INSTALL_DIR" ] || [ ! -e "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || true
  mv "$tmp" "$dest" 2>/dev/null || { echo "Need sudo to write to $INSTALL_DIR"; sudo mv "$tmp" "$dest"; }
else
  echo "Need sudo to write to $INSTALL_DIR"
  sudo mv "$tmp" "$dest"
fi

if [ "$platform" = "darwin" ]; then
  xattr -d com.apple.quarantine "$dest" 2>/dev/null || true
fi

echo "Installed to $dest"
if ! command -v malveon >/dev/null 2>&1; then
  echo "$INSTALL_DIR isn't on your PATH yet — add it, or run malveon directly: $dest"
else
  echo "Run 'malveon' to verify."
fi
