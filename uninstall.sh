#!/bin/sh
# Uninstalls cl for macOS/Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/spltek/cl/main/uninstall.sh | sh
#   ./uninstall.sh

set -e

INSTALL_DIR="${CL_INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR=""

case "$(uname)" in
    Darwin) CONFIG_DIR="$HOME/Library/Application Support/cl" ;;
    *)      CONFIG_DIR="$HOME/.config/cl" ;;
esac

echo "Removing binary from $INSTALL_DIR..."
rm -f "$INSTALL_DIR/cl"

echo "Removing config from $CONFIG_DIR..."
rm -rf "$CONFIG_DIR"

echo "cl uninstalled successfully."
