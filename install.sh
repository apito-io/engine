#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Apito Engine installer
echo -e "${BLUE}"
echo "🚀 Apito Engine Installer"
echo "=========================="
echo -e "${NC}"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case ${ARCH} in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}❌ Unsupported architecture: ${ARCH}${NC}"
        echo "Supported architectures: x86_64 (amd64), aarch64/arm64"
        exit 1
        ;;
esac

# Map OS names
case ${OS} in
    linux)
        OS="linux"
        ;;
    darwin)
        OS="darwin"
        ;;
    *)
        echo -e "${RED}❌ Unsupported operating system: ${OS}${NC}"
        echo "Supported operating systems: Linux, macOS (Darwin)"
        exit 1
        ;;
esac

# Show detected platform
echo -e "${GREEN}✅ Detected platform: ${OS}-${ARCH}${NC}"

# Get latest release version
echo -e "${YELLOW}📡 Fetching latest release information...${NC}"
LATEST_VERSION=$(curl -s https://api.github.com/repos/apito-io/engine/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo -e "${RED}❌ Failed to fetch latest version${NC}"
    exit 1
fi

echo -e "${GREEN}📦 Latest version: ${LATEST_VERSION}${NC}"

# Download URL
BINARY_NAME="engine-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/apito-io/engine/releases/download/${LATEST_VERSION}/${BINARY_NAME}.zip"

echo -e "${YELLOW}⬇️  Downloading ${BINARY_NAME}.zip...${NC}"

# Download the binary
if command -v wget &> /dev/null; then
    wget -q --show-progress "$DOWNLOAD_URL" -O "${BINARY_NAME}.zip"
elif command -v curl &> /dev/null; then
    curl -L --progress-bar "$DOWNLOAD_URL" -o "${BINARY_NAME}.zip"
else
    echo -e "${RED}❌ Neither wget nor curl is available. Please install one of them.${NC}"
    exit 1
fi

# Extract the binary
echo -e "${YELLOW}📦 Extracting binary...${NC}"
unzip -q "${BINARY_NAME}.zip"

# Make executable
chmod +x engine

# Clean up
rm "${BINARY_NAME}.zip"

# Check if installation was successful
if [ -f "engine" ]; then
    echo -e "${GREEN}✅ Apito Engine installed successfully!${NC}"
    echo ""
    echo -e "${BLUE}🎯 Next steps:${NC}"
    echo "1. Start Apito Engine:"
    echo -e "   ${YELLOW}./engine${NC}"
    echo ""
    echo "2. Open your browser and visit:"
    echo -e "   ${YELLOW}http://localhost:8080${NC}"
    echo ""
    echo -e "${BLUE}📚 Documentation:${NC}"
    echo "   https://docs.apito.io"
    echo ""
    echo -e "${BLUE}💬 Support:${NC}"
    echo "   https://discord.com/invite/fwHgF8pUpt"
else
    echo -e "${RED}❌ Installation failed. Binary not found.${NC}"
    exit 1
fi 