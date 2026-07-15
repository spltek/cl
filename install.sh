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
  tag="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
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

echo "Installed cl $tag to $INSTALL_DIR/cl"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo
    echo "Note: $INSTALL_DIR is not on your PATH. Add something like this to your shell profile:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

echo
echo "Next: add shell integration to your profile so picked commands land on your prompt:"
echo '  zsh:  echo '"'"'eval "$(cl init zsh)"'"'"'  >> ~/.zshrc'
echo '  bash: echo '"'"'eval "$(cl init bash)"'"'"' >> ~/.bashrc'
