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
# Any other strange output with 2 columns: "<name> <priority>" (2 columns)
ARCH=$($OPKG_BIN print-architecture | awk '{if (NF==3) print $2, $3; else print $1, $2}' | grep -v -E "^(all|noarch)" | sort -n -k 2 | tail -n 1 | awk '{print $1}')

if [ -z "$ARCH" ]; then
    echo "Error: Could not detect architecture."
    exit 1
fi

echo "Architecture: $ARCH"
REPO_URL="https://github.com/LarinIvan/MagiTrickle_Mod/releases/latest/download"

# Version Check
echo "Checking versions..."
INSTALLED_VERSION=$($OPKG_BIN list-installed magitrickle_mod | awk '{print $3}')

NEED_UPDATE=0

if [ -z "$INSTALLED_VERSION" ]; then
    echo "Package not installed. Installing..."
    NEED_UPDATE=1
else
    LATEST_VERSION=$(wget -qO- https://api.github.com/repos/LarinIvan/MagiTrickle_Mod/releases/latest | grep '"tag_name"' | head -n1 | cut -d'"' -f4)
    
    CLEAN_INSTALLED_VERSION=$(echo "$INSTALLED_VERSION" | sed 's/-[0-9]*$//')

    echo "Latest version: $LATEST_VERSION"
    echo "Installed version: $INSTALLED_VERSION"

    if [ -n "$LATEST_VERSION" ] && [ "$LATEST_VERSION" != "$CLEAN_INSTALLED_VERSION" ]; then
        echo "New version available ($LATEST_VERSION). Updating..."
        NEED_UPDATE=1
    else
        echo "Latest version already installed. No action needed."
        NEED_UPDATE=0
    fi
fi


if [ "$IS_ENTWARE" -eq 1 ]; then
    CONF_DIR="/opt/etc/opkg"
    CONF_FILE="$CONF_DIR/magitrickle_mod.conf"
    INIT_SCRIPT="/opt/etc/init.d/S99magitrickle"
    
    mkdir -p "$CONF_DIR"
    echo "src/gz magitrickle_mod $REPO_URL" > "$CONF_FILE"
    echo "Repository successfully added to $CONF_FILE"
    
    echo ""
    
    if [ "$NEED_UPDATE" -eq 1 ]; then
        if [ -z "$INSTALLED_VERSION" ]; then
            echo "Installing magitrickle_mod..."
            $OPKG_BIN update
            
            if $OPKG_BIN install magitrickle_mod; then
                SUCCESS=1
                echo ""
                echo "✓ Installation successful!"
            else
                SUCCESS=0
            fi
        else
            echo "Upgrading magitrickle_mod..."
            $OPKG_BIN update
            
            if $OPKG_BIN upgrade magitrickle_mod; then
                SUCCESS=1
                echo ""
                echo "✓ Update successful!"
            else
                SUCCESS=0
            fi
        fi

        if [ "$SUCCESS" -eq 1 ]; then
            echo "Starting service..."
            
            # Start the service
            $INIT_SCRIPT start
            
            echo ""
            echo "✓ Service started successfully!"
            
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
    else
        # Already installed and up to date
        echo ""
        echo "✓ System is up to date."
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
    
    if [ "$NEED_UPDATE" -eq 1 ]; then
        echo "Updating package..."
        echo "1. Updating package lists..."
        $OPKG_BIN update

        # Construct direct URL to the package file
        DIRECT_URL="$REPO_URL/magitrickle_mod_openwrt-${ARCH}.ipk"
        
        if [ -z "$INSTALLED_VERSION" ]; then
            echo "2. Installing package for architecture: $ARCH"
            echo "Installing from: $DIRECT_URL"
            
            if $OPKG_BIN install "$DIRECT_URL" --force-checksum; then
                echo ""
                echo "✓ Installation successful!"
            else
                echo ""
                echo "Error: Package installation failed."
                echo "Architecture '$ARCH' is supported."
                echo "Check available packages at: https://github.com/LarinIvan/MagiTrickle_Mod/releases"
                exit 1
            fi
        else
            echo "2. Updating package for architecture: $ARCH"
            echo "Installing from: $DIRECT_URL"
            
            if $OPKG_BIN install "$DIRECT_URL" --force-checksum --force-reinstall; then
                echo ""
                echo "✓ Update successful!"
            else
                echo ""
                echo "Error: Package update failed."
                echo "Architecture '$ARCH' is supported."
                echo "Check available packages at: https://github.com/LarinIvan/MagiTrickle_Mod/releases"
                exit 1
            fi
        fi
        
        echo "Starting service..."
        
        # Enable service (autostart on boot)
        $INIT_SCRIPT enable
        
        # Start the service
        $INIT_SCRIPT start
        
        echo ""
        echo "✓ Service started successfully!"
        
        ROUTER_IP=$(echo $SSH_CONNECTION | awk '{print $3}')
        if [ -z "$ROUTER_IP" ]; then
            ROUTER_IP="your_router_ip"
        fi
        
        echo "Access web interface at: http://${ROUTER_IP}:8080"
    else
        # Already installed and up to date
        echo ""
        echo "✓ System is up to date."
    fi
fi