#!/bin/sh
set -e

# Configuration
GITHUB_OWNER="bastio-ai"
GITHUB_REPO="bast"
BINARY_NAME="bast"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "${YELLOW}Installing ${BINARY_NAME}...${NC}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin) OS="darwin" ;;
  linux) OS="linux" ;;
  *) echo "${RED}Unsupported OS: $OS${NC}"; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

# Get latest release
LATEST=$(curl -sSL "https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | cut -d'"' -f4)

if [ -z "$LATEST" ]; then
  echo "${RED}Failed to fetch latest release${NC}"
  exit 1
fi

VERSION="${LATEST#v}"
FILENAME="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download/${LATEST}/${FILENAME}"
CHECKSUM_URL="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download/${LATEST}/${BINARY_NAME}_${VERSION}_checksums.txt"

# Create temp directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Download and verify
echo "${YELLOW}Downloading ${BINARY_NAME} ${LATEST}...${NC}"
curl -sSL -o "${TEMP_DIR}/${FILENAME}" "${DOWNLOAD_URL}"
curl -sSL -o "${TEMP_DIR}/checksums.txt" "${CHECKSUM_URL}"

cd "${TEMP_DIR}"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c checksums.txt 2>&1 | grep -q "${FILENAME}" || { echo "${RED}Checksum failed${NC}"; exit 1; }
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 -c checksums.txt 2>&1 | grep -q "${FILENAME}" || { echo "${RED}Checksum failed${NC}"; exit 1; }
fi

# Extract and install
tar -xzf "${FILENAME}"

if [ -w "$INSTALL_DIR" ]; then
  mv "${BINARY_NAME}" "${INSTALL_DIR}/"
else
  echo "${YELLOW}sudo required for ${INSTALL_DIR}${NC}"
  sudo mv "${BINARY_NAME}" "${INSTALL_DIR}/"
fi

echo "${GREEN}Successfully installed ${BINARY_NAME} ${LATEST}${NC}"
echo ""
echo "Next steps:"
echo "  1. Run: ${BINARY_NAME} init"
echo "  2. Add to ~/.zshrc: eval \"\$(${BINARY_NAME} hook zsh)\""
echo "  3. Restart terminal and press Ctrl+A"
