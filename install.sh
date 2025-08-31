#!/bin/bash

# ==========================================================
# xhub-agent install.sh installation and uninstallation script
# Usage: 
#   Interactive: sudo bash install.sh (recommended)
#   Direct install: sudo bash install.sh "token=abc123&api_url=https://api.example.com&grpc_url=https://grpc.example.com"
#   Direct uninstall: sudo bash install.sh --uninstall
# ==========================================================

set -e

# Configuration variables
QUERY="$1"
INSTALL_DIR="/opt/xhub-agent"
SERVICE_NAME="xhub-agent"
LOG_DIR="$INSTALL_DIR/logs"
UNINSTALL_MODE=false

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Log functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root user
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script needs to be run as root user"
        exit 1
    fi
}

# Check existing installation
check_existing_installation() {
    EXISTING_SERVICE=false
    EXISTING_BINARY=false
    EXISTING_CONFIG=false
    
    # Check if service exists and is active
    if systemctl list-unit-files | grep -q "^$SERVICE_NAME.service"; then
        log_info "Found existing xhub-agent service"
        EXISTING_SERVICE=true
        
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            log_info "Stopping existing xhub-agent service..."
            systemctl stop "$SERVICE_NAME"
            log_info "Service stopped"
        fi
        
        if systemctl is-enabled --quiet "$SERVICE_NAME"; then
            log_info "Disabling existing service temporarily..."
            systemctl disable "$SERVICE_NAME"
        fi
    fi
    
    # Check if binary exists
    if [ -f "$INSTALL_DIR/xhub-agent" ]; then
        log_info "Found existing xhub-agent binary"
        EXISTING_BINARY=true
        
        # Get current version if possible
        if [ -x "$INSTALL_DIR/xhub-agent" ]; then
            CURRENT_VERSION=$("$INSTALL_DIR/xhub-agent" --version 2>/dev/null || "$INSTALL_DIR/xhub-agent" -v 2>/dev/null || echo "unknown")
            log_info "Current version: $CURRENT_VERSION"
        fi
    fi
    
    # Check if config exists
    if [ -f "$INSTALL_DIR/config.yml" ]; then
        log_info "Found existing configuration file"
        EXISTING_CONFIG=true
        
        # Backup existing config
        BACKUP_CONFIG="$INSTALL_DIR/config.yml.backup.$(date +%Y%m%d_%H%M%S)"
        cp "$INSTALL_DIR/config.yml" "$BACKUP_CONFIG"
        log_info "Backed up existing config to: $BACKUP_CONFIG"
    fi
    
    if [ "$EXISTING_BINARY" = true ] || [ "$EXISTING_SERVICE" = true ]; then
        log_warn "Existing xhub-agent installation detected"
        log_info "This installation will:"
        [ "$EXISTING_BINARY" = true ] && log_info "  - Update the binary file"
        [ "$EXISTING_SERVICE" = true ] && log_info "  - Update the service configuration"
        [ "$EXISTING_CONFIG" = true ] && log_info "  - Update configuration (backup created)"
        log_info "  - Restart the service with new version"
        echo
    fi
}

# Show interactive menu if no parameters provided
show_menu() {
    if [ -z "$QUERY" ]; then
        echo
        log_info "🚀 XHub Agent Installation & Management Script"
        echo
        echo "Please select an option:"
        echo "  1) Install xhub-agent"
        echo "  2) Uninstall xhub-agent"
        echo "  3) Exit"
        echo
        read -p "Enter your choice (1-3): " -r CHOICE
        echo
        
        case $CHOICE in
            1)
                log_info "📦 Installation mode selected"
                echo "Enter installation parameters:"
                echo "  Recommended: token=xxx&api_url=https://api.example.com&grpc_url=https://grpc.example.com"
                echo "  Basic: token=xxx&api_url=https://api.example.com"
                read -p "Parameters: " -r QUERY
                if [ -z "$QUERY" ]; then
                    log_error "Installation parameters cannot be empty"
                    echo "Example: token=abc123&api_url=https://api.example.com&grpc_url=https://grpc.example.com"
                    exit 1
                fi
                ;;
            2)
                log_info "🗑️  Uninstall mode selected"
                UNINSTALL_MODE=true
                ;;
            3)
                log_info "👋 Goodbye!"
                exit 0
                ;;
            *)
                log_error "Invalid choice. Please select 1, 2, or 3."
                exit 1
                ;;
        esac
        echo
        return
    fi
    
    # Check for command line uninstall flags
    if [ "$QUERY" = "--uninstall" ] || [ "$QUERY" = "-u" ]; then
        UNINSTALL_MODE=true
        log_info "🗑️  Uninstall mode activated"
        return
    fi
}

# Parse URL parameters
parse_params() {
    if [ "$UNINSTALL_MODE" = true ]; then
        return # Skip parameter parsing for uninstall mode
    fi
    
    if [ -z "$QUERY" ]; then
        log_error "Missing parameters"
        echo "Usage:"
        echo "  Install:   bash install.sh 'token=abc123&xhub=https://xhub.example.com'"
        echo "  Uninstall: bash install.sh --uninstall"
        exit 1
    fi

    log_info "Parsing installation parameters..."
    log_info "Raw parameters: $QUERY"

    # Parse token, API URL, and optional gRPC URL
    TOKEN=$(echo "$QUERY" | sed -n 's/.*token=\([^&]*\).*/\1/p')
    XHUB_API_BASE=$(echo "$QUERY" | sed -n 's/.*api_url=\([^&]*\).*/\1/p')
    GRPC_SERVER_OVERRIDE=$(echo "$QUERY" | sed -n 's/.*grpc_url=\([^&]*\).*/\1/p')

    # Backward compatibility: still support old 'xhub' parameter
    if [ -z "$XHUB_API_BASE" ]; then
        XHUB_API_BASE=$(echo "$QUERY" | sed -n 's/.*xhub=\([^&]*\).*/\1/p')
    fi

    if [ -z "$TOKEN" ] || [ -z "$XHUB_API_BASE" ]; then
        log_error "Unable to parse token or API URL"
        log_error "Raw parameters: $QUERY"
        echo "Usage:"
        echo "  New format: bash install.sh 'token=abc123&api_url=https://api.example.com&grpc_url=https://grpc.example.com'"
        echo "  Basic: bash install.sh 'token=abc123&api_url=https://api.example.com'"
        echo "  Old format (still supported): bash install.sh 'token=abc123&xhub=https://api.example.com'"
        exit 1
    fi

    log_info "Parameters parsed successfully:"
    log_info "├─ Token: ${TOKEN:0:8}...${TOKEN: -4}"
    log_info "├─ API URL: $XHUB_API_BASE"
    if [ -n "$GRPC_SERVER_OVERRIDE" ]; then
        log_info "└─ gRPC URL: $GRPC_SERVER_OVERRIDE"
    else
        log_info "└─ gRPC Server: will be extracted from API URL"
    fi
    echo
}

# Check system compatibility
check_compatibility() {
    OS=$(uname -s)
    
    if [ "$OS" = "Linux" ]; then
        log_info "Checking Linux system compatibility..."
        
        # Check GLIBC version for Linux
        if command -v ldd &> /dev/null; then
            GLIBC_VERSION=$(ldd --version 2>/dev/null | head -n1 | grep -o '[0-9]\+\.[0-9]\+' | head -n1)
            if [ -n "$GLIBC_VERSION" ]; then
                log_info "Detected GLIBC version: $GLIBC_VERSION"
                
                # Compare version (basic check for major compatibility)
                MAJOR_VERSION=$(echo "$GLIBC_VERSION" | cut -d. -f1)
                MINOR_VERSION=$(echo "$GLIBC_VERSION" | cut -d. -f2)
                
                if [ "$MAJOR_VERSION" -gt 2 ] || ([ "$MAJOR_VERSION" -eq 2 ] && [ "$MINOR_VERSION" -ge 31 ]); then
                    log_info "✅ System is compatible (GLIBC $GLIBC_VERSION >= 2.31)"
                else
                    log_warn "⚠️  System may have compatibility issues (GLIBC $GLIBC_VERSION < 2.31)"
                    log_warn "    The agent uses static linking to maximize compatibility"
                    log_warn "    If installation fails, please upgrade your system or contact support"
                fi
            else
                log_info "GLIBC version detection failed, continuing with installation..."
            fi
        else
            log_info "ldd not available, skipping GLIBC check"
        fi
        
        # Check for musl (Alpine Linux)
        if [ -f /etc/alpine-release ]; then
            log_info "Detected Alpine Linux (musl libc)"
            log_info "✅ Static binary should work on Alpine Linux"
        fi
        
        # Check common distributions
        if [ -f /etc/os-release ]; then
            DISTRO=$(grep "^ID=" /etc/os-release | cut -d= -f2 | tr -d '"')
            VERSION_ID=$(grep "^VERSION_ID=" /etc/os-release | cut -d= -f2 | tr -d '"' 2>/dev/null || echo "unknown")
            log_info "Distribution: $DISTRO $VERSION_ID"
            
            case $DISTRO in
                debian)
                    if [ "$VERSION_ID" -ge 11 ] 2>/dev/null; then
                        case $VERSION_ID in
                            11) log_info "✅ Debian 11 (Bullseye) is supported - GLIBC 2.31" ;;
                            12) log_info "✅ Debian 12 (Bookworm) is supported - GLIBC 2.36" ;;
                            *) log_info "✅ Debian $VERSION_ID is supported" ;;
                        esac
                    else
                        log_warn "⚠️  Debian $VERSION_ID may have compatibility issues (requires Debian 11+)"
                    fi
                    ;;
                ubuntu)
                    MAJOR_VER=$(echo "$VERSION_ID" | cut -d. -f1)
                    if [ "$MAJOR_VER" -ge 20 ] 2>/dev/null; then
                        log_info "✅ Ubuntu $VERSION_ID is supported"  
                    else
                        log_warn "⚠️  Ubuntu $VERSION_ID may have compatibility issues"
                    fi
                    ;;
                centos|rhel)
                    MAJOR_VER=$(echo "$VERSION_ID" | cut -d. -f1)
                    if [ "$MAJOR_VER" -ge 8 ] 2>/dev/null; then
                        log_info "✅ $DISTRO $VERSION_ID is supported"
                    else
                        log_warn "⚠️  $DISTRO $VERSION_ID may have compatibility issues"
                    fi
                    ;;
                alpine)
                    log_info "✅ Alpine Linux is supported (static binary)"
                    ;;
                *)
                    log_info "Unknown distribution, but static binary should work on most systems"
                    ;;
            esac
        fi
    fi
    echo
}

# Detect system architecture  
detect_arch() {
    OS=$(uname -s)
    ARCH=$(uname -m)
    
    case $OS in
        Linux)
            case $ARCH in
                x86_64) 
                    ARCHIVE_FILE="xhub-agent_linux_amd64.tar.gz"
                    BIN_FILE="xhub-agent_linux_amd64"
                    ;;
                aarch64|arm64) 
                    ARCHIVE_FILE="xhub-agent_linux_arm64.tar.gz"
                    BIN_FILE="xhub-agent_linux_arm64"
                    ;;
                *) log_error "Unsupported architecture: $ARCH"; exit 1 ;;
            esac
            ;;
        Darwin)
            case $ARCH in
                x86_64)
                    ARCHIVE_FILE="xhub-agent_darwin_amd64.tar.gz"
                    BIN_FILE="xhub-agent_darwin_amd64"
                    ;;
                arm64)
                    ARCHIVE_FILE="xhub-agent_darwin_arm64.tar.gz"
                    BIN_FILE="xhub-agent_darwin_arm64"
                    ;;
                *) log_error "Unsupported architecture: $ARCH"; exit 1 ;;
            esac
            ;;
        MINGW*|MSYS*|CYGWIN*)
            ARCHIVE_FILE="xhub-agent_windows_amd64.exe.zip"
            BIN_FILE="xhub-agent_windows_amd64.exe"
            ;;
        *)
            log_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac
    
    log_info "Detected system: $OS/$ARCH"
    log_info "Will download: $ARCHIVE_FILE"
}

# Fetch configuration information
fetch_config() {
    log_info "Fetching configuration from xhub..."
    
    CONFIG_URL="$XHUB_API_BASE/agent/config?token=$TOKEN"
    log_info "Config API URL: $CONFIG_URL"
    
    CONFIG_JSON=$(curl -s "$CONFIG_URL" 2>/dev/null || echo "")
    
    if [ -z "$CONFIG_JSON" ]; then
        log_error "Unable to fetch configuration from xhub"
        log_error "Please check if token and xhub address are correct"
        log_error "API URL: $CONFIG_URL"
        exit 1
    fi


    # Check if jq is installed
    if ! command -v jq &> /dev/null; then
        log_warn "jq not installed, trying to install..."
        if command -v apt-get &> /dev/null; then
            apt-get update && apt-get install -y jq
        elif command -v yum &> /dev/null; then
            yum install -y jq
        else
            log_error "Unable to install jq, please install manually and retry"
            exit 1
        fi
    fi

    log_info "Parsing configuration data..."
    
    # Parse configuration
    SERVER_ID=$(echo "$CONFIG_JSON" | jq -r '.id' 2>/dev/null || echo "")
    XUI_USER=$(echo "$CONFIG_JSON" | jq -r '.user' 2>/dev/null || echo "")
    XUI_PASS=$(echo "$CONFIG_JSON" | jq -r '.pwd' 2>/dev/null || echo "")
    XHUB_API_KEY=$(echo "$CONFIG_JSON" | jq -r '.apiKey' 2>/dev/null || echo "")
    GRPC_PORT=$(echo "$CONFIG_JSON" | jq -r '.grpcPort' 2>/dev/null || echo "")
    ROOT_PATH=$(echo "$CONFIG_JSON" | jq -r '.rootPath' 2>/dev/null || echo "")
    PORT=$(echo "$CONFIG_JSON" | jq -r '.port' 2>/dev/null || echo "")
    RESOLVED_DOMAIN=$(echo "$CONFIG_JSON" | jq -r '.resolvedDomain' 2>/dev/null || echo "")
    
    # Extract gRPC server address
    if [ -n "$GRPC_SERVER_OVERRIDE" ]; then
        # Use provided gRPC server address
        GRPC_SERVER=$(echo "$GRPC_SERVER_OVERRIDE" | sed 's|^https\?://||' | sed 's|/.*||' | sed 's|:.*||')
        log_info "Using custom gRPC server: $GRPC_SERVER"
    else
        # Extract from XHUB_API_BASE (remove http/https and path)
        GRPC_SERVER=$(echo "$XHUB_API_BASE" | sed 's|^https\?://||' | sed 's|/.*||' | sed 's|:.*||')
        log_info "Extracted gRPC server from API base: $GRPC_SERVER"
    fi
    
    if [ -z "$GRPC_SERVER" ]; then
        log_error "Unable to determine gRPC server address"
        log_error "API Base: $XHUB_API_BASE"
        [ -n "$GRPC_SERVER_OVERRIDE" ] && log_error "gRPC Override: $GRPC_SERVER_OVERRIDE"
        [ -n "$RESOLVED_DOMAIN" ] && log_error "Resolved Domain: $RESOLVED_DOMAIN"
        exit 1
    fi
    
    # Apply smart gRPC port logic based on server type
    if [ "$GRPC_SERVER" = "localhost" ] || [ "$GRPC_SERVER" = "127.0.0.1" ] || [ "$GRPC_SERVER" = "::1" ]; then
        # Local development: use backend provided port or default to 9090
        if [ -z "$GRPC_PORT" ] || [ "$GRPC_PORT" = "null" ] || [ "$GRPC_PORT" = "0" ]; then
            GRPC_PORT="9090"
            log_info "Setting default gRPC port for local development: $GRPC_PORT"
        else
            log_info "Using configured gRPC port for local development: $GRPC_PORT"
        fi
    else
        # Production/remote server: ALWAYS override to 443 (standard TLS port)
        if [ -n "$GRPC_PORT" ] && [ "$GRPC_PORT" != "null" ] && [ "$GRPC_PORT" != "0" ] && [ "$GRPC_PORT" != "443" ]; then
            log_info "Overriding backend gRPC port $GRPC_PORT to 443 for production TLS"
        fi
        GRPC_PORT="443"
        log_info "Using production gRPC port: $GRPC_PORT (TLS required)"
    fi

    if [ -z "$SERVER_ID" ] || [ "$SERVER_ID" = "null" ]; then
        log_error "Configuration information incomplete or format error"
        log_error "Raw response: $CONFIG_JSON"
        exit 1
    fi

    log_info "Configuration parsed successfully:"
    log_info "├─ Server ID: ${SERVER_ID}..."
    log_info "├─ XUI User: $XUI_USER"
    log_info "├─ XUI Password: ${XUI_PASS}***"
    log_info "├─ API Key: ${XHUB_API_KEY}..."
    log_info "├─ gRPC Server: $GRPC_SERVER"
    log_info "├─ gRPC Port: $GRPC_PORT"
    log_info "├─ Resolved Domain: $RESOLVED_DOMAIN"
    log_info "├─ Root Path: $ROOT_PATH"
    log_info "└─ Port: $PORT"
    echo
}

# Create installation directories
create_directories() {
    log_info "Creating installation directories..."
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$LOG_DIR"
}

# Download Agent binary file
download_agent() {
    log_info "Downloading xhub-agent..."
    
    DOWNLOAD_URL="https://github.com/tsubamex/xhub-agent/releases/latest/download/$ARCHIVE_FILE"
    
    # Download archive file
    if curl -L -o "$INSTALL_DIR/$ARCHIVE_FILE" "$DOWNLOAD_URL"; then
        log_info "Archive downloaded successfully"
        
        # Extract the archive
        cd "$INSTALL_DIR"
        if [[ "$ARCHIVE_FILE" == *.tar.gz ]]; then
            tar -xzf "$ARCHIVE_FILE"
            # The binary file should be extracted as the BIN_FILE name
            if [ -f "$BIN_FILE" ]; then
                mv "$BIN_FILE" "xhub-agent"
            else
                log_error "Binary file $BIN_FILE not found in archive"
                exit 1
            fi
        elif [[ "$ARCHIVE_FILE" == *.zip ]]; then
            # Check if unzip is available
            if ! command -v unzip &> /dev/null; then
                log_warn "unzip not installed, trying to install..."
                if command -v apt-get &> /dev/null; then
                    apt-get update && apt-get install -y unzip
                elif command -v yum &> /dev/null; then
                    yum install -y unzip
                else
                    log_error "Unable to install unzip, please install manually and retry"
                    exit 1
                fi
            fi
            unzip -o "$ARCHIVE_FILE"
            if [ -f "$BIN_FILE" ]; then
                mv "$BIN_FILE" "xhub-agent"
            else
                log_error "Binary file $BIN_FILE not found in archive"
                exit 1
            fi
        fi
        
        # Clean up archive file
        rm -f "$ARCHIVE_FILE"
        
        # Make binary executable
        chmod +x "$INSTALL_DIR/xhub-agent"
        log_info "Agent extraction and setup completed"
    else
        log_error "Download failed, please check network connection and download address"
        exit 1
    fi
}

# Generate configuration file
generate_config() {
    log_info "Generating configuration file..."
    
    # Only preserve optional timeout/performance settings from existing config
    if [ "$EXISTING_CONFIG" = true ] && [ -f "$BACKUP_CONFIG" ]; then
        log_info "Preserving optional settings from previous installation..."
        
        # Only preserve these optional performance settings
        EXISTING_XUI_BASE_URL=$(grep "^xui_base_url:" "$BACKUP_CONFIG" 2>/dev/null | cut -d'"' -f2 || echo "127.0.0.1")
        EXISTING_POLL_INTERVAL=$(grep "^poll_interval:" "$BACKUP_CONFIG" 2>/dev/null | awk '{print $2}' || echo "2")
        EXISTING_LOG_LEVEL=$(grep "^log_level:" "$BACKUP_CONFIG" 2>/dev/null | cut -d'"' -f2 || echo "info")
        
        log_info "Preserved: poll_interval, log_level"
        log_info "Updated: all server-provided configuration (uuid, credentials, endpoints)"
    else
        # Use default values for new installation
        EXISTING_XUI_BASE_URL="127.0.0.1"
        EXISTING_POLL_INTERVAL="2"
        EXISTING_LOG_LEVEL="info"
    fi
    
    cat > "$INSTALL_DIR/config.yml" <<EOF
# xhub-agent configuration file
# Server configuration (automatically updated from xhub server)
uuid: $SERVER_ID
xui_user: $XUI_USER
xui_pass: $XUI_PASS
xhub_api_key: $XHUB_API_KEY
resolvedDomain: "$RESOLVED_DOMAIN"

# gRPC connection configuration
grpcServer: "$GRPC_SERVER"
grpcPort: $GRPC_PORT

# 3x-ui connection configuration (from server)
rootPath: $ROOT_PATH
port: $PORT

# Optional configuration (preserved from previous installation)
xui_base_url: "$EXISTING_XUI_BASE_URL"
poll_interval: $EXISTING_POLL_INTERVAL
log_level: "$EXISTING_LOG_LEVEL"
EOF

    log_info "Configuration file generated: $INSTALL_DIR/config.yml"
    
    if [ "$EXISTING_CONFIG" = true ]; then
        log_info "Server configuration updated from xhub"
        log_info "Optional settings (timeouts, intervals) preserved"
        log_info "Original config backed up to: $BACKUP_CONFIG"
    fi
}

# Create systemd service
create_service() {
    log_info "Creating systemd service..."
    
    cat > "/etc/systemd/system/$SERVICE_NAME.service" <<EOF
[Unit]
Description=XHub Agent
After=network.target

[Service]
Type=simple
User=root
ExecStart=$INSTALL_DIR/xhub-agent -c $INSTALL_DIR/config.yml -l $LOG_DIR/agent.log
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    log_info "systemd service created"
}

# Start service
start_service() {
    if [ "$EXISTING_SERVICE" = true ]; then
        log_info "Restarting xhub-agent service..."
    else
        log_info "Starting xhub-agent service..."
    fi
    
    systemctl enable "$SERVICE_NAME"
    systemctl start "$SERVICE_NAME"
    
    # Wait a moment then check status
    sleep 3
    
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        if [ "$EXISTING_SERVICE" = true ]; then
            log_info "xhub-agent service restarted successfully"
            log_info "Service updated and running with new version"
        else
            log_info "xhub-agent service started successfully"
        fi
        log_info "Check status: systemctl status $SERVICE_NAME"
        log_info "View logs: tail -f $LOG_DIR/agent.log"
        
        # Display version info if available
        sleep 1
        NEW_VERSION=$("$INSTALL_DIR/xhub-agent" --version 2>/dev/null || "$INSTALL_DIR/xhub-agent" -v 2>/dev/null || echo "unknown")
        if [ "$NEW_VERSION" != "unknown" ]; then
            log_info "Running version: $NEW_VERSION"
        fi
    else
        log_error "xhub-agent service failed to start"
        log_error "View details: systemctl status $SERVICE_NAME"
        log_error "Check logs: journalctl -u $SERVICE_NAME -f"
        exit 1
    fi
}

# ========================================
# UNINSTALL FUNCTIONS
# ========================================

# Check what components are installed
check_installed_components() {
    log_info "🔍 Checking installed components..."
    
    FOUND_SERVICE=false
    FOUND_BINARY=false
    FOUND_CONFIG=false
    FOUND_INSTALL_DIR=false
    
    # Check service
    if systemctl list-unit-files | grep -q "^$SERVICE_NAME.service"; then
        FOUND_SERVICE=true
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            SERVICE_STATUS="active"
        elif systemctl is-enabled --quiet "$SERVICE_NAME"; then
            SERVICE_STATUS="enabled"
        else
            SERVICE_STATUS="installed"
        fi
        log_info "├─ ✅ Service: $SERVICE_NAME ($SERVICE_STATUS)"
    else
        log_info "├─ ❌ Service: not found"
    fi
    
    # Check binary
    if [ -f "$INSTALL_DIR/xhub-agent" ]; then
        FOUND_BINARY=true
        BINARY_VERSION=$("$INSTALL_DIR/xhub-agent" --version 2>/dev/null || "$INSTALL_DIR/xhub-agent" -v 2>/dev/null || echo "unknown")
        log_info "├─ ✅ Binary: $INSTALL_DIR/xhub-agent ($BINARY_VERSION)"
    else
        log_info "├─ ❌ Binary: not found"
    fi
    
    # Check config
    if [ -f "$INSTALL_DIR/config.yml" ]; then
        FOUND_CONFIG=true
        log_info "├─ ✅ Config: $INSTALL_DIR/config.yml"
    else
        log_info "├─ ❌ Config: not found"
    fi
    
    # Check install directory
    if [ -d "$INSTALL_DIR" ]; then
        FOUND_INSTALL_DIR=true
        DIR_SIZE=$(du -sh "$INSTALL_DIR" 2>/dev/null | cut -f1 || echo "unknown")
        log_info "└─ ✅ Install directory: $INSTALL_DIR ($DIR_SIZE)"
    else
        log_info "└─ ❌ Install directory: not found"
    fi
    
    echo
    
    # Check if anything is installed
    if [ "$FOUND_SERVICE" = false ] && [ "$FOUND_BINARY" = false ] && [ "$FOUND_CONFIG" = false ] && [ "$FOUND_INSTALL_DIR" = false ]; then
        log_warn "❌ No xhub-agent installation found"
        log_info "Nothing to uninstall"
        exit 0
    fi
}

# Stop and remove service
remove_service() {
    if [ "$FOUND_SERVICE" = true ]; then
        log_info "🛑 Stopping and removing service..."
        
        # Stop service if running
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            log_info "├─ Stopping service..."
            systemctl stop "$SERVICE_NAME" || log_warn "Failed to stop service"
        fi
        
        # Disable service if enabled
        if systemctl is-enabled --quiet "$SERVICE_NAME"; then
            log_info "├─ Disabling service..."
            systemctl disable "$SERVICE_NAME" || log_warn "Failed to disable service"
        fi
        
        # Remove service file
        if [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
            log_info "├─ Removing service file..."
            rm -f "/etc/systemd/system/$SERVICE_NAME.service"
        fi
        
        # Reload systemd
        log_info "└─ Reloading systemd daemon..."
        systemctl daemon-reload
        
        log_info "✅ Service removed successfully"
    else
        log_info "ℹ️  No service to remove"
    fi
}

# Remove binary and installation directory
remove_files() {
    if [ "$FOUND_INSTALL_DIR" = true ]; then
        log_info "🗂️  Backing up configuration..."
        
        # Create backup of config if it exists
        if [ -f "$INSTALL_DIR/config.yml" ]; then
            BACKUP_DIR="$HOME/.xhub-agent-backup-$(date +%Y%m%d_%H%M%S)"
            mkdir -p "$BACKUP_DIR"
            cp "$INSTALL_DIR/config.yml" "$BACKUP_DIR/"
            if [ -d "$LOG_DIR" ]; then
                cp -r "$LOG_DIR" "$BACKUP_DIR/" 2>/dev/null || true
            fi
            log_info "├─ Configuration backed up to: $BACKUP_DIR"
        fi
        
        log_info "🗑️  Removing installation files..."
        
        # Remove installation directory
        log_info "├─ Removing directory: $INSTALL_DIR"
        rm -rf "$INSTALL_DIR"
        
        log_info "✅ Files removed successfully"
    else
        log_info "ℹ️  No files to remove"
    fi
}

# Confirm uninstallation
confirm_uninstall() {
    echo
    log_warn "⚠️  This will completely remove xhub-agent from your system:"
    [ "$FOUND_SERVICE" = true ] && echo "   - Stop and remove systemd service"
    [ "$FOUND_BINARY" = true ] && echo "   - Remove binary file"
    [ "$FOUND_CONFIG" = true ] && echo "   - Remove configuration (with backup)"
    [ "$FOUND_INSTALL_DIR" = true ] && echo "   - Remove installation directory"
    echo
    
    read -p "Are you sure you want to uninstall xhub-agent? (y/N): " -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "❌ Uninstallation cancelled"
        exit 0
    fi
    
    log_info "✅ Uninstallation confirmed, proceeding..."
    echo
}

# Main uninstall function
uninstall() {
    log_info "🗑️  Starting xhub-agent uninstallation..."
    echo
    
    check_installed_components
    confirm_uninstall
    remove_service
    remove_files
    
    echo
    log_info "🎉 xhub-agent has been successfully uninstalled!"
    log_info "┌─ Service: removed"
    log_info "├─ Binary: removed"
    log_info "├─ Configuration: backed up and removed"
    log_info "└─ Installation directory: removed"
    echo
    if [ -n "$BACKUP_DIR" ]; then
        log_info "📦 Configuration backup available at: $BACKUP_DIR"
        log_info "    You can restore it if you reinstall xhub-agent later"
    fi
}

# Main function
main() {
    check_root
    show_menu
    
    if [ "$UNINSTALL_MODE" = true ]; then
        # Uninstallation mode
        uninstall
    else
        # Installation mode
        log_info "Starting xhub-agent installation..."
        
        check_existing_installation
        parse_params
        check_compatibility
        detect_arch
        fetch_config
        create_directories
        download_agent
        generate_config
        create_service
        start_service
        
        if [ "$EXISTING_BINARY" = true ] || [ "$EXISTING_SERVICE" = true ]; then
            log_info "Update completed!"
            log_info "xhub-agent has been updated to the latest version"
            log_info "Server configuration updated from xhub server"
            [ "$EXISTING_CONFIG" = true ] && log_info "Optional settings (timeouts, intervals) preserved"
        else
            log_info "Installation completed!"
            log_info "xhub-agent has been installed and configured"
        fi
    fi
}

# If running this script directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
