#!/bin/sh
# Conductor installation script
# Usage: curl -sSL https://raw.githubusercontent.com/tombee/conductor/main/scripts/install.sh | sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
REPO="tombee/conductor"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TEMP_DIR="$(mktemp -d)"

# Cleanup on exit
cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

# Print colored message
print_info() {
    printf "${GREEN}==>${NC} %s\n" "$1"
}

print_error() {
    printf "${RED}Error:${NC} %s\n" "$1" >&2
}

print_warning() {
    printf "${YELLOW}Warning:${NC} %s\n" "$1"
}

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Darwin)
            OS="darwin"
            ;;
        Linux)
            OS="linux"
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            print_error "Conductor currently supports macOS and Linux only."
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            print_error "Conductor supports amd64 and arm64 only."
            exit 1
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    print_info "Detected platform: $PLATFORM"
}

# Get latest release version from GitHub
get_latest_version() {
    print_info "Fetching latest release version..."

    # Try using curl with GitHub API
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | \
                  grep '"tag_name":' | \
                  sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | \
                  grep '"tag_name":' | \
                  sed -E 's/.*"([^"]+)".*/\1/')
    else
        print_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi

    if [ -z "$VERSION" ]; then
        print_error "Could not determine latest version"
        exit 1
    fi

    print_info "Latest version: $VERSION"
}

# Download binary and checksum
download_release() {
    BINARY_NAME="conductor_${VERSION}_${PLATFORM}.tar.gz"
    CHECKSUM_NAME="conductor_${VERSION}_checksums.txt"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/${CHECKSUM_NAME}"

    print_info "Downloading conductor..."

    cd "$TEMP_DIR"

    if command -v curl >/dev/null 2>&1; then
        curl -sLO "$DOWNLOAD_URL" || {
            print_error "Failed to download conductor from $DOWNLOAD_URL"
            exit 1
        }
        curl -sLO "$CHECKSUM_URL" || {
            print_warning "Could not download checksums file, skipping verification"
            SKIP_CHECKSUM=1
        }
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$DOWNLOAD_URL" || {
            print_error "Failed to download conductor from $DOWNLOAD_URL"
            exit 1
        }
        wget -q "$CHECKSUM_URL" || {
            print_warning "Could not download checksums file, skipping verification"
            SKIP_CHECKSUM=1
        }
    fi

    print_info "Download complete"
}

# Verify checksum
verify_checksum() {
    if [ -n "$SKIP_CHECKSUM" ]; then
        return 0
    fi

    print_info "Verifying checksum..."

    # Check if shasum or sha256sum is available
    if command -v shasum >/dev/null 2>&1; then
        ACTUAL_CHECKSUM=$(shasum -a 256 "$BINARY_NAME" | awk '{print $1}')
    elif command -v sha256sum >/dev/null 2>&1; then
        ACTUAL_CHECKSUM=$(sha256sum "$BINARY_NAME" | awk '{print $1}')
    else
        print_warning "Neither shasum nor sha256sum found, skipping checksum verification"
        return 0
    fi

    EXPECTED_CHECKSUM=$(grep "$BINARY_NAME" "$CHECKSUM_NAME" | awk '{print $1}')

    if [ -z "$EXPECTED_CHECKSUM" ]; then
        print_warning "Could not find checksum for $BINARY_NAME, skipping verification"
        return 0
    fi

    if [ "$ACTUAL_CHECKSUM" != "$EXPECTED_CHECKSUM" ]; then
        print_error "Checksum verification failed!"
        print_error "Expected: $EXPECTED_CHECKSUM"
        print_error "Got:      $ACTUAL_CHECKSUM"
        exit 1
    fi

    print_info "Checksum verified successfully"
}

# Extract and install binary
install_binary() {
    print_info "Extracting archive..."
    tar -xzf "$BINARY_NAME"

    # Check if install directory is writable
    if [ ! -w "$INSTALL_DIR" ]; then
        print_info "Installing to $INSTALL_DIR (requires sudo)..."
        sudo install -m 755 conductor "$INSTALL_DIR/conductor"
    else
        print_info "Installing to $INSTALL_DIR..."
        install -m 755 conductor "$INSTALL_DIR/conductor"
    fi

    print_info "Installation complete!"
}

# Verify installation
verify_installation() {
    if command -v conductor >/dev/null 2>&1; then
        INSTALLED_VERSION=$(conductor version --json 2>/dev/null | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
        print_info "Conductor installed successfully: $INSTALLED_VERSION"
    else
        print_warning "conductor command not found in PATH"
        print_warning "You may need to add $INSTALL_DIR to your PATH"
        print_warning "Add this to your shell profile:"
        print_warning "  export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
}

# Print next steps
print_next_steps() {
    echo ""
    echo "Next steps:"
    echo "  1. Run 'conductor init' to set up your configuration"
    echo "  2. Try 'conductor quickstart' to run your first workflow"
    echo "  3. See 'conductor --help' for more commands"
    echo ""
    echo "Documentation: https://github.com/${REPO}"
}

# Main installation flow
main() {
    echo "Conductor Installer"
    echo "==================="
    echo ""

    detect_platform
    get_latest_version
    download_release
    verify_checksum
    install_binary
    verify_installation
    print_next_steps
}

main
