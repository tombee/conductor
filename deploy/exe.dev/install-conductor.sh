#!/bin/bash
# install-conductor.sh - Install Conductor on an exe.dev VM
#
# Usage:
#   Option 1: SSH in and run directly
#     ssh exe.dev ssh <vmname>
#     curl -fsSL https://raw.githubusercontent.com/conductor-dev/conductor/main/deploy/exe.dev/install-conductor.sh | bash
#
#   Option 2: Pipe via SSH
#     ssh exe.dev ssh <vmname> < install-conductor.sh
#
# This script:
#   1. Downloads the Conductor binary
#   2. Generates an API key for authentication
#   3. Configures the daemon for remote access
#   4. Starts the daemon and verifies health
#
# Requirements:
#   - curl
#   - openssl (for API key generation)
#   - tar

set -e

# Configuration
CONDUCTOR_VERSION="${CONDUCTOR_VERSION:-latest}"
CONDUCTOR_PORT="${CONDUCTOR_PORT:-9000}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${CONFIG_DIR:-$HOME/.config/conductor}"
DATA_DIR="${DATA_DIR:-$HOME/.local/share/conductor}"

# Colors for output (if terminal supports it)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    NC=''
fi

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v curl &> /dev/null; then
        log_error "curl is not installed. Please install curl and retry."
        exit 1
    fi

    if ! command -v openssl &> /dev/null; then
        log_error "openssl is not installed. Please install openssl and retry."
        exit 1
    fi

    if ! command -v tar &> /dev/null; then
        log_error "tar is not installed. Please install tar and retry."
        exit 1
    fi

    # Check if port is available
    if command -v ss &> /dev/null; then
        if ss -tuln 2>/dev/null | grep -q ":${CONDUCTOR_PORT}"; then
            log_error "Port ${CONDUCTOR_PORT} is already in use."
            log_error "Either stop the existing service or set CONDUCTOR_PORT to a different value."
            exit 1
        fi
    elif command -v netstat &> /dev/null; then
        if netstat -tuln 2>/dev/null | grep -q ":${CONDUCTOR_PORT}"; then
            log_error "Port ${CONDUCTOR_PORT} is already in use."
            exit 1
        fi
    fi

    log_info "Prerequisites OK"
}

# Download and install Conductor binary
install_binary() {
    log_info "Installing Conductor ${CONDUCTOR_VERSION}..."

    mkdir -p "$INSTALL_DIR"

    # Determine architecture
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        arm64)   ARCH="arm64" ;;
        *)
            log_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    OS=$(uname -s | tr '[:upper:]' '[:lower:]')

    # Download URL (adjust based on actual release structure)
    if [ "$CONDUCTOR_VERSION" = "latest" ]; then
        DOWNLOAD_URL="https://github.com/tombee/conductor/releases/latest/download/conductor-${OS}-${ARCH}.tar.gz"
    else
        DOWNLOAD_URL="https://github.com/tombee/conductor/releases/download/${CONDUCTOR_VERSION}/conductor-${OS}-${ARCH}.tar.gz"
    fi

    log_info "Downloading from: $DOWNLOAD_URL"

    # Download and extract
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    if ! curl -fsSL --max-time 120 "$DOWNLOAD_URL" -o "$TEMP_DIR/conductor.tar.gz"; then
        log_error "Failed to download Conductor. Check your internet connection and try again."
        log_error "URL: $DOWNLOAD_URL"
        exit 1
    fi

    if ! tar xzf "$TEMP_DIR/conductor.tar.gz" -C "$TEMP_DIR"; then
        log_error "Failed to extract Conductor archive."
        exit 1
    fi

    # Install binary
    if [ -f "$TEMP_DIR/conductor" ]; then
        mv "$TEMP_DIR/conductor" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/conductor"
    else
        log_error "Conductor binary not found in archive."
        exit 1
    fi

    # Add to PATH if not already there
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$HOME/.bashrc"
        export PATH="$PATH:$INSTALL_DIR"
        log_info "Added $INSTALL_DIR to PATH in ~/.bashrc"
    fi

    log_info "Conductor installed to $INSTALL_DIR"
}

# Generate API key
generate_api_key() {
    log_info "Generating API key..."

    CONDUCTOR_API_KEY=$(openssl rand -hex 32)
    export CONDUCTOR_API_KEY

    # Save to bashrc for persistence
    # Remove any existing CONDUCTOR_API_KEY export first
    if [ -f "$HOME/.bashrc" ]; then
        sed -i '/^export CONDUCTOR_API_KEY=/d' "$HOME/.bashrc" 2>/dev/null || true
    fi
    echo "export CONDUCTOR_API_KEY=$CONDUCTOR_API_KEY" >> "$HOME/.bashrc"

    echo ""
    echo "=============================================="
    echo -e "${GREEN}YOUR API KEY (save this securely!):${NC}"
    echo ""
    echo "  $CONDUCTOR_API_KEY"
    echo ""
    echo "=============================================="
    echo ""
    log_warn "This key is required to connect from your local CLI."
    log_warn "Store it in a password manager or secure location."
    echo ""
}

# Create configuration
create_config() {
    log_info "Creating configuration..."

    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"

    cat > "$CONFIG_DIR/config.yaml" << EOF
# Conductor daemon configuration for exe.dev deployment
# Generated by install-conductor.sh

daemon:
  listen:
    # TCP port for remote access (exe.dev will proxy this)
    tcp_addr: ":${CONDUCTOR_PORT}"
    # Required for non-localhost connections
    allow_remote: true

  # Authentication - required for security
  auth:
    enabled: true
    api_keys:
      - ${CONDUCTOR_API_KEY}

# LLM Provider configuration
# Uncomment and configure as needed:
#
# providers:
#   anthropic:
#     api_key: \${ANTHROPIC_API_KEY}
#   openai:
#     api_key: \${OPENAI_API_KEY}
EOF

    log_info "Configuration written to $CONFIG_DIR/config.yaml"
}

# Start daemon
start_daemon() {
    log_info "Starting Conductor daemon..."

    # Ensure PATH includes install dir
    export PATH="$PATH:$INSTALL_DIR"

    # Start the daemon using conductor daemon start
    if ! conductor daemon start; then
        log_error "Failed to start daemon. Check logs with: conductor daemon logs"
        exit 1
    fi

    # Wait for health check
    log_info "Waiting for daemon to become healthy..."
    MAX_ATTEMPTS=30
    ATTEMPT=0
    while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
        if conductor daemon status > /dev/null 2>&1; then
            log_info "Conductor daemon is healthy!"
            return 0
        fi
        ATTEMPT=$((ATTEMPT + 1))
        sleep 1
    done

    log_error "Daemon failed to start within ${MAX_ATTEMPTS} seconds."
    log_error "Check logs with: conductor daemon logs"
    exit 1
}

# Print next steps
print_next_steps() {
    echo ""
    echo "=============================================="
    echo -e "${GREEN}Installation complete!${NC}"
    echo "=============================================="
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Share the port with exe.dev (run from your local machine):"
    echo "   ssh exe.dev share port \$(hostname) ${CONDUCTOR_PORT}"
    echo ""
    echo "2. Configure your local CLI:"
    echo "   export CONDUCTOR_HOST=https://<url-from-step-1>"
    echo "   export CONDUCTOR_API_KEY=${CONDUCTOR_API_KEY}"
    echo ""
    echo "3. Test the connection:"
    echo "   conductor runs list"
    echo ""
    echo "Useful commands on this VM:"
    echo "  conductor daemon start   - Start the daemon"
    echo "  conductor daemon stop    - Stop the daemon"
    echo "  conductor daemon status  - Check daemon status"
    echo "  conductor daemon logs    - View daemon logs"
    echo ""
}

# Main
main() {
    echo ""
    echo "=============================================="
    echo "Conductor Installation for exe.dev"
    echo "=============================================="
    echo ""

    check_prerequisites
    install_binary
    generate_api_key
    create_config
    start_daemon
    print_next_steps
}

main "$@"
