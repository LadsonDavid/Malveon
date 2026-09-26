#!/bin/sh
# Installs the malveon binary for macOS/Linux: detects OS/arch, downloads
# the matching release asset, makes it executable, places it on PATH,
# and (macOS) clears the Gatekeeper quarantine flag so the first run
# doesn't get blocked. Run via:
#   curl -fsSL https://raw.githubusercontent.com/LadsonDavid/Malveon/main/install.sh | sh
set -e

REPO="LadsonDavid/Malveon"
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
GITHUB="${MALVEON_RELEASE_URL:-https://github.com/${REPO}/releases/latest/download}"   # overridable only for testing
COUNTER="${MALVEON_COUNTER_URL:-https://get.malveon.com/latest}"   # counts the download, then forwards to GitHub

# malveon's release key (public half). Every release's SHA256SUMS is signed
# with its private half, which never leaves the maintainer's machine.
PUBKEY_PEM="-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAQys7kKGp7MivUG53NwXgYScQSmHHIcpNU/xBh/yUVzs=
-----END PUBLIC KEY-----"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
tmp="$work/$asset"

echo "Downloading ${asset}..."
if ! curl -fsSL "${COUNTER}/${asset}?src=install.sh" -o "$tmp" 2>/dev/null &&
   ! curl -fsSL "${GITHUB}/${asset}" -o "$tmp"; then
  echo "malveon: download failed — check https://github.com/${REPO}/releases for available builds" >&2
  exit 1
fi

# Verify before installing anything: the checksum list comes straight from
# GitHub (never through the counter), and the binary must match it.
if ! curl -fsSL "${GITHUB}/SHA256SUMS" -o "$work/SHA256SUMS" ||
   ! curl -fsSL "${GITHUB}/SHA256SUMS.sig" -o "$work/SHA256SUMS.sig"; then
  echo "malveon: this release has no signed checksum list — not installing it" >&2
  exit 1
fi
want=$(grep " ${asset}\$" "$work/SHA256SUMS" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp" | cut -d' ' -f1)
elif command -v shasum >/dev/null 2>&1; then
  got=$(shasum -a 256 "$tmp" | cut -d' ' -f1)
else
  echo "malveon: no sha256sum or shasum on this machine to verify the download — not installing it" >&2
  exit 1
fi
if [ -z "$want" ] || [ "$got" != "$want" ]; then
  echo "malveon: the download doesn't match the release's checksum — not installing it" >&2
  exit 1
fi
# The signature proves the checksum list itself came from malveon's
# maintainer. It needs OpenSSL 3; without it, say plainly what was checked.
if openssl version 2>/dev/null | grep -q "^OpenSSL 3"; then
  printf '%s\n' "$PUBKEY_PEM" > "$work/key.pem"
  openssl base64 -d -A -in "$work/SHA256SUMS.sig" -out "$work/sig.bin"
  if ! openssl pkeyutl -verify -pubin -inkey "$work/key.pem" -rawin -in "$work/SHA256SUMS" -sigfile "$work/sig.bin" >/dev/null 2>&1; then
    echo "malveon: the release's checksum list isn't signed with malveon's release key — not installing it" >&2
    exit 1
  fi
  echo "Verified: checksum and signature match."
else
  echo "Verified: checksum matches (signature not checked — that needs OpenSSL 3; 'malveon update' checks it from now on)."
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

echo ""
echo "Want a custom check rule for your stack, or found something broken? Run 'malveon register' —"
echo "opens a real GitHub Discussion, no email or signup required."
