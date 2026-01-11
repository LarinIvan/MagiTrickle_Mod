#!/bin/sh

echo ""
echo "Detecting platform..."

# Find opkg first
OPKG_BIN=$(which opkg 2>/dev/null || command -v opkg 2>/dev/null)

if [ -z "$OPKG_BIN" ]; then
    echo "Error: opkg not found. Please install Entware or OpenWRT package manager."
    exit 1
fi

echo "Found opkg at: $OPKG_BIN"

# Ensure required dependencies are installed
echo "Checking dependencies..."
if ! $OPKG_BIN list-installed | grep -q "^wget-ssl "; then
    echo "Installing wget-ssl..."
    $OPKG_BIN update
    $OPKG_BIN install wget-ssl
fi

if ! $OPKG_BIN list-installed | grep -q "^ca-certificates "; then
    echo "Installing ca-certificates..."
    $OPKG_BIN install ca-certificates
fi
echo "Dependencies OK"

# Detect platform by specific marker files
IS_ENTWARE=0
IS_OPENWRT=0

if [ -f /etc/openwrt_release ]; then
    # OpenWRT has this file with distribution info
    IS_OPENWRT=1
    echo "Platform: OpenWRT (detected via /etc/openwrt_release)"
elif [ -f /opt/etc/entware_release ]; then
    # Entware has this file
    IS_ENTWARE=1
    echo "Platform: Entware (detected via /opt/etc/entware_release)"
else
    # Fallback: detect by opkg location
    case "$OPKG_BIN" in
        /opt/*)
            IS_ENTWARE=1
            echo "Platform: Entware (detected by opkg location: $OPKG_BIN)"
            ;;
        *)
            IS_OPENWRT=1
            echo "Platform: Assuming OpenWRT-like (opkg location: $OPKG_BIN)"
            ;;
    esac
fi

# Detect Architecture
# Find the architecture with the highest priority that is NOT 'all' or 'noarch'
# Entware output: "arch <name> <priority>" (3 columns)
# OpenWRT output: arch <name> <priority>" (3 columns)
# Any other output with 2 columns: "<name> <priority>" (2 columns)
ARCH=$($OPKG_BIN print-architecture | awk '{if (NF==3) print $2, $3; else print $1, $2}' | grep -v -E "^(all|noarch)" | sort -n -k 2 | tail -n 1 | awk '{print $1}')

if [ -z "$ARCH" ]; then
    echo "Error: Could not detect architecture."
    exit 1
fi

echo "Architecture: $ARCH"

# Add Repository
REPO_URL="https://github.com/LarinIvan/MagiTrickle_Mod/releases/latest/download"

if [ "$IS_ENTWARE" -eq 1 ]; then
    CONF_DIR="/opt/etc/opkg"
    CONF_FILE="$CONF_DIR/magitrickle_mod.conf"
    INIT_SCRIPT="/opt/etc/init.d/S99magitrickle"
    
    mkdir -p "$CONF_DIR"
    echo "src/gz magitrickle_mod $REPO_URL" > "$CONF_FILE"
    echo "Repository successfully added to $CONF_FILE"
    
    echo ""
    echo "Installing magitrickle_mod..."
    
    # Update package list
    $OPKG_BIN update
    
    # Install magitrickle_mod
    if $OPKG_BIN install magitrickle_mod; then
        echo ""
        echo "✓ Installation successful!"
        echo "Starting service..."
        
        # Start the service
        $INIT_SCRIPT start
        
        echo ""
        echo "✓ Service started successfully!"
        
        # Get router IP address from SSH connection
        ROUTER_IP=$(echo $SSH_CONNECTION | awk '{print $3}')
        if [ -z "$ROUTER_IP" ]; then
            ROUTER_IP="your_router_ip"
        fi
        
        echo "Access web interface at: http://${ROUTER_IP}:8080"
    else
        echo ""
        echo "✗ Installation failed!"
        exit 1
    fi

elif [ "$IS_OPENWRT" -eq 1 ]; then
    # For OpenWRT /etc/opkg/customfeeds.conf used by LuCI or standard conf
    CONF_FILE="/etc/opkg/customfeeds.conf"
    OLD_CONF_FILE="/etc/opkg/magitrickle_mod.conf"
    INIT_SCRIPT="/etc/init.d/magitrickle"

    # Remove old separate config if exists to avoid duplicates
    if [ -f "$OLD_CONF_FILE" ]; then
        rm "$OLD_CONF_FILE"
    fi

    # Check if feed already exists in customfeeds
    if grep -q "magitrickle_mod" "$CONF_FILE" 2>/dev/null; then
        # Update existing line
        sed -i "\#src/gz magitrickle_mod#d" "$CONF_FILE"
    fi
    
    echo "src/gz magitrickle_mod $REPO_URL" >> "$CONF_FILE"
    echo "Adding repository to $CONF_FILE..."
    echo "Repository successfully added!"

    echo ""
    echo "Installing package..."
    echo "1. Updating package lists..."
    $OPKG_BIN update

    echo "2. Removing old versions if present..."
    # Always try remove first to avoid conflicts if previously installed
    $OPKG_BIN remove magitrickle >/dev/null 2>&1
    $OPKG_BIN remove magitrickle_mod >/dev/null 2>&1
    
    echo "3. Installing package for architecture: $ARCH"
    
    # Construct direct URL to the package file
    # Package naming format: magitrickle_mod_openwrt-${ARCH}.ipk (note: dash, not underscore!)
    DIRECT_URL="$REPO_URL/magitrickle_mod_openwrt-${ARCH}.ipk"
    echo "Installing from: $DIRECT_URL"
    
    if $OPKG_BIN install "$DIRECT_URL" --force-checksum; then
        echo ""
        echo "Success! Package installed successfully."
    else
        echo ""
        echo "Error: Package installation failed."
        echo "Architecture '$ARCH' is supported."
        echo "Check available packages at: https://github.com/LarinIvan/MagiTrickle_Mod/releases"
        exit 1
    fi

    
    echo ""
    echo "✓ Installation successful!"
    echo "Starting service..."
    
    # Enable service (autostart on boot)
    $INIT_SCRIPT enable
    
    # Start the service
    $INIT_SCRIPT start
    
    echo ""
    echo "✓ Service started successfully!"
    
    # Get router IP address from SSH connection
    ROUTER_IP=$(echo $SSH_CONNECTION | awk '{print $3}')
    if [ -z "$ROUTER_IP" ]; then
        ROUTER_IP="your_router_ip"
    fi
    
    echo "Access web interface at: http://${ROUTER_IP}:8080"
fi