#!/bin/sh
set -e

# Owner/Repo information
OWNER="robert-at-pretension-io"
REPO="simple-agent"
GITHUB_URL="https://github.com/${OWNER}/${REPO}"

# Detect OS and Arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Normalize Arch
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Normalize OS
case "$OS" in
    darwin) OS="darwin" ;;
    linux) OS="linux" ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

# Determine version
VERSION="$1"
if [ -z "$VERSION" ]; then
    echo "Fetching latest version..."
    LATEST_URL="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -sL "$LATEST_URL" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "$LATEST_URL" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        echo "Error: curl or wget is required."
        exit 1
    fi
fi

if [ -z "$VERSION" ]; then
    echo "Error: Could not determine version."
    exit 1
fi

echo "Installing ${REPO} ${VERSION} for ${OS}/${ARCH}..."

# Construct download URL
BINARY_NAME="simple-agent-${OS}-${ARCH}"
DOWNLOAD_URL="${GITHUB_URL}/releases/download/${VERSION}/${BINARY_NAME}"

# Download
TMP_DIR=$(mktemp -d)
DEST="${TMP_DIR}/${BINARY_NAME}"

echo "Downloading from $DOWNLOAD_URL..."
if command -v curl >/dev/null 2>&1; then
    CODE=$(curl -sL -w "%{http_code}" -o "$DEST" "$DOWNLOAD_URL")
    if [ "$CODE" != "200" ]; then
        echo "Error: Download failed with status $CODE"
        exit 1
    fi
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$DEST" "$DOWNLOAD_URL"
else
    echo "Error: curl or wget is required."
    exit 1
fi

# Install
chmod +x "$DEST"

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
    echo "Warning: /usr/local/bin not writable. Installing to $INSTALL_DIR"
    echo "Please ensure $INSTALL_DIR is in your PATH."
else
    # Need sudo if not writable, but we are just checking -w
    # If we are root or have permissions, good. If not, fallback or ask for sudo?
    # Simplicity: Fallback to local if system is not writable, or assume user handles sudo before running script.
    # For this script, I'll try to move, and if it fails, suggest sudo.
    :
fi

# Try to move
if mv "$DEST" "$INSTALL_DIR/simple-agent"; then
    echo "Successfully installed to $INSTALL_DIR/simple-agent"
    echo "Run 'simple-agent --help' to get started."
else
    echo "Error: Could not move binary to $INSTALL_DIR"
    echo "Try running with sudo or check permissions."
    rm -rf "$TMP_DIR"
    exit 1
fi

rm -rf "$TMP_DIR"