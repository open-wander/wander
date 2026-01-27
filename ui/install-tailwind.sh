#!/bin/bash

# Install Tailwind CSS standalone CLI
# This script downloads the appropriate Tailwind CLI binary for your platform

set -e

TAILWIND_VERSION="v3.4.17"
INSTALL_DIR="$(dirname "$0")/bin"
TAILWIND_BIN="${INSTALL_DIR}/tailwindcss"

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case "${ARCH}" in
    x86_64)
        ARCH="x64"
        ;;
    aarch64)
        ARCH="arm64"
        ;;
    arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: ${ARCH}"
        exit 1
        ;;
esac

# Map OS names
case "${OS}" in
    darwin)
        OS="macos"
        ;;
    linux)
        OS="linux"
        ;;
    *)
        echo "Unsupported OS: ${OS}"
        exit 1
        ;;
esac

# Construct download URL
DOWNLOAD_URL="https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/tailwindcss-${OS}-${ARCH}"

echo "Installing Tailwind CSS ${TAILWIND_VERSION} for ${OS}-${ARCH}..."
echo "Download URL: ${DOWNLOAD_URL}"

# Create bin directory if it doesn't exist
mkdir -p "${INSTALL_DIR}"

# Download Tailwind CLI
echo "Downloading Tailwind CLI..."
curl -sL -o "${TAILWIND_BIN}" "${DOWNLOAD_URL}"

# Make it executable
chmod +x "${TAILWIND_BIN}"

# Verify installation
if [ -f "${TAILWIND_BIN}" ] && [ -x "${TAILWIND_BIN}" ]; then
    echo "✓ Tailwind CLI installed successfully!"
    echo "  Binary location: ${TAILWIND_BIN}"

    # Test the binary
    VERSION_OUTPUT=$("${TAILWIND_BIN}" --help | head -n 1)
    echo "  Version: ${VERSION_OUTPUT}"

    echo ""
    echo "Next steps:"
    echo "  1. Build CSS: ./bin/tailwindcss -i static/css/input.css -o static/css/output.css"
    echo "  2. Watch mode: ./bin/tailwindcss -i static/css/input.css -o static/css/output.css --watch"
    echo "  3. Production: ./bin/tailwindcss -i static/css/input.css -o static/css/output.css --minify"
else
    echo "✗ Installation failed!"
    exit 1
fi
