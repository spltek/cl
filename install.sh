#!/usr/bin/env bash
# Installs the latest (or a specific) cl release for macOS/Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/silviopola/cl/main/install.sh | sh
#   curl -fsSL .../install.sh | sh -s v0.1.0        # install a specific tag
#
# Env vars:
#   CL_INSTALL_DIR   Where to put the binary (default: $HOME/.local/bin)
set -euo pipefail

REPO="silviopola/cl"
INSTALL_DIR="${CL_INSTALL_DIR:-$HOME/.local/bin}"

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Darwin) goos="darwin" ;;
  Linux) goos="linux" ;;
  *)
    echo "cl: unsupported OS '$os'. See https://github.com/$REPO for manual install." >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *)
    echo "cl: unsupported architecture '$arch'." >&2
    exit 1
    ;;
esac

tag="${1:-}"
if [ -z "$tag" ]; then
  echo "Resolving latest release..."
  # Try jq first (robust JSON parsing), fallback to grep+sed if unavailable.
  if command -v jq >/dev/null 2>&1; then
    tag="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
      | jq -r '.tag_name')"
  else
    tag="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
      | grep '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  fi
fi

if [ -z "$tag" ]; then
  echo "cl: could not determine the release to install." >&2
  exit 1
fi

version="${tag#v}"
archive="cl_${version}_${goos}_${goarch}.tar.gz"
url="https://github.com/$REPO/releases/download/$tag/$archive"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "Downloading $url ..."
curl -fsSL "$url" -o "$tmp_dir/$archive"
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"

mkdir -p "$INSTALL_DIR"
mv "$tmp_dir/cl" "$INSTALL_DIR/cl"
chmod +x "$INSTALL_DIR/cl"

if [ "$os" = "Darwin" ]; then
  # Downloads made via curl are not quarantined by macOS Gatekeeper
  # (only downloads attributed to a browser/Mail are), but clear the
  # attribute defensively in case it is ever present. No special
  # privileges needed: this only touches an xattr on a file the
  # current user owns.
  if command -v xattr >/dev/null 2>&1; then
    xattr -d com.apple.quarantine "$INSTALL_DIR/cl" 2>/dev/null || true
  fi

  # On Apple Silicon, the kernel validates a binary's code signature
  # on every launch. Extracting the binary from the release archive
  # can leave it with an invalid/stale signature, which makes the OS
  # SIGKILL it on launch ("zsh: killed", exit code 137) even though
  # the file itself is otherwise fine. Re-signing ad-hoc fixes this;
  # again, no special privileges needed for signing your own file.
  if command -v codesign >/dev/null 2>&1; then
    codesign --sign - --force "$INSTALL_DIR/cl" 2>/dev/null || true
  fi
fi

echo "Installed cl $tag to $INSTALL_DIR/cl"
echo ""
echo "Done. Make sure $INSTALL_DIR is on your PATH, then cl is ready to use."
