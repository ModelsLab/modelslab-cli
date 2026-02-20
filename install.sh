#!/bin/sh
# ModelsLab CLI installer
# Usage: curl -fsSL https://modelslab.sh/install.sh | sh
set -e

REPO="ModelsLab/modelslab-cli"
BINARY="modelslab"

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin) OS="darwin" ;;
        linux) OS="linux" ;;
        mingw*|msys*|cygwin*) OS="windows" ;;
        *) echo "Unsupported OS: $OS"; exit 1 ;;
    esac

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
}

# Get latest release version
get_latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | \
        grep '"tag_name"' | sed 's/.*"v\(.*\)".*/\1/'
}

# Download and install
install() {
    detect_platform

    VERSION=$(get_latest_version)
    if [ -z "$VERSION" ]; then
        echo "Error: Could not determine latest version"
        exit 1
    fi

    echo "Installing ${BINARY} v${VERSION} (${OS}/${ARCH})..."

    EXT="tar.gz"
    if [ "$OS" = "windows" ]; then
        EXT="zip"
    fi

    URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY}_${VERSION}_${OS}_${ARCH}.${EXT}"

    TMPDIR=$(mktemp -d)
    trap "rm -rf $TMPDIR" EXIT

    echo "Downloading from ${URL}..."
    curl -fsSL "$URL" -o "${TMPDIR}/archive.${EXT}"

    echo "Extracting..."
    if [ "$EXT" = "tar.gz" ]; then
        tar -xzf "${TMPDIR}/archive.${EXT}" -C "$TMPDIR"
    else
        unzip -q "${TMPDIR}/archive.${EXT}" -d "$TMPDIR"
    fi

    # Determine install location
    INSTALL_DIR="/usr/local/bin"
    if [ ! -w "$INSTALL_DIR" ]; then
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi

    mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    chmod +x "${INSTALL_DIR}/${BINARY}"

    echo ""
    echo "✓ ${BINARY} v${VERSION} installed to ${INSTALL_DIR}/${BINARY}"
    echo ""

    # Check if in PATH
    if ! command -v "$BINARY" >/dev/null 2>&1; then
        echo "Note: ${INSTALL_DIR} is not in your PATH."
        echo "Add it with:"
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
    fi

    echo "Get started:"
    echo "  ${BINARY} auth login"
    echo "  ${BINARY} models search --search flux"
    echo "  ${BINARY} generate image --prompt \"sunset over mountains\""
}

install
