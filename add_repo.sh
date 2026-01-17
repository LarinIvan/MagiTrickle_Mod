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
# URLs
REPO_URL="https://github.com/LarinIvan/MagiTrickle_Mod/releases/latest/download"

# Version Check
echo "Checking versions..."
INSTALLED_VERSION=$($OPKG_BIN list-installed magitrickle_mod | awk '{print $3}')
LATEST_VERSION=$(wget -qO- -U "MagiTrickle" https://api.github.com/repos/LarinIvan/MagiTrickle_Mod/releases/latest | awk -F'"' '{for(i=1;i<=NF;i++)if($i=="tag_name"){print $(i+2);exit}}')

NEED_UPDATE=0

if [ -z "$INSTALLED_VERSION" ]; then
    echo "Package not installed."
    echo "Latest version available: $LATEST_VERSION"
    echo "Installing..."
    NEED_UPDATE=1
else
    CLEAN_INSTALLED_VERSION=$(echo "$INSTALLED_VERSION" | sed 's/-[0-9]*$//')

    echo "Latest version: $LATEST_VERSION"
    echo "Installed version: $CLEAN_INSTALLED_VERSION"

    if [ -n "$LATEST_VERSION" ] && [ "$LATEST_VERSION" != "$CLEAN_INSTALLED_VERSION" ]; then
        echo "New version available ($LATEST_VERSION). Updating..."
        NEED_UPDATE=1
    else
        echo "Latest version already installed. No action needed."
        NEED_UPDATE=0
    fi
fi


# Platform Specific Configuration
if [ "$IS_ENTWARE" -eq 1 ]; then
    CONF_DIR="/opt/etc/opkg"
    CONF_FILE="$CONF_DIR/magitrickle_mod.conf"
    INIT_SCRIPT="/opt/etc/init.d/S99magitrickle"
    PLATFORM_NAME="entware"
    
    # Entware config content
    REPO_CONF_CONTENT="src/gz magitrickle_mod $REPO_URL"
    
elif [ "$IS_OPENWRT" -eq 1 ]; then
    CONF_DIR="/etc/opkg"
    CONF_FILE="$CONF_DIR/magitrickle_mod.conf"
    INIT_SCRIPT="/etc/init.d/magitrickle"
    PLATFORM_NAME="openwrt"
    
    # OpenWRT config content
    REPO_CONF_CONTENT="src/gz magitrickle_mod $REPO_URL"
fi

# --- 1. Setup Repository for Entware / Clean up for OpenWRT ---
if [ "$IS_OPENWRT" -eq 1 ]; then
    # OpenWRT: Remove old repository config if exists
    if [ -f "$CONF_FILE" ]; then
        rm "$CONF_FILE"
        echo "Removed old repository config: $CONF_FILE"
    fi
elif [ "$IS_ENTWARE" -eq 1 ]; then
    echo "Setting up repository..."
    mkdir -p "$CONF_DIR"

    echo "$REPO_CONF_CONTENT" > "$CONF_FILE"
    echo "Repository configured in $CONF_FILE"
    echo ""
fi

# --- 2. Installation / Update Logic ---

if [ "$NEED_UPDATE" -eq 1 ]; then
    # Update package lists only for Entware (OpenWRT uses direct download)
    if [ "$IS_ENTWARE" -eq 1 ]; then
        # echo "Clearing opkg cache..."
        # rm -f /opt/var/opkg-lists/magitrickle_mod
        echo "Updating package lists..."
        $OPKG_BIN update 2>&1 | awk '!/has no valid architecture/'
    fi
    
    INSTALL_SUCCESS=0
    
    # DIRECT URL for Fallback
    DIRECT_URL="$REPO_URL/magitrickle_mod_${PLATFORM_NAME}-${ARCH}.ipk"
    
    if [ -z "$INSTALLED_VERSION" ]; then
        # --- FRESH INSTALL ---
        echo "Installing magitrickle_mod (Fresh Install)..."
        
        if [ "$IS_OPENWRT" -eq 1 ]; then
            # OpenWRT: Direct download only
            echo "Using direct package download for OpenWRT..."
            if $OPKG_BIN install "$DIRECT_URL" --force-checksum --no-check-certificate; then
                INSTALL_SUCCESS=1
                echo "✓ Direct installation successful!"
            else
                echo "❌ Installation failed."
                INSTALL_SUCCESS=0
            fi
        else
            # Entware: Try repository first, then fallback to direct URL
            echo "Attempt 1: Installing from Repository..."
            if $OPKG_BIN install magitrickle_mod 2>&1 | awk '!/has no valid architecture/'; then
                INSTALL_SUCCESS=1
                echo "✓ Repository installation successful!"
            else
                echo "⚠ Repository installation failed."
                echo "Attempt 2: Fallback to Direct Download..."
                echo "Source: $DIRECT_URL"
                
                if $OPKG_BIN install "$DIRECT_URL" --force-checksum; then
                    INSTALL_SUCCESS=1
                    echo "✓ Direct installation successful!"
                else
                    echo "❌ Installation failed."
                    INSTALL_SUCCESS=0
                fi
            fi
        fi
    else
        # --- UPGRADE ---
        echo "Upgrading magitrickle_mod (Update)..."
        
        if [ "$IS_OPENWRT" -eq 1 ]; then
            # OpenWRT: Direct download with reinstall
            echo "Using direct package download for OpenWRT..."
            if $OPKG_BIN install "$DIRECT_URL" --force-checksum --no-check-certificate --force-reinstall; then
                INSTALL_SUCCESS=1
                echo "✓ Direct update successful!"
            else
                echo "❌ Update failed."
                INSTALL_SUCCESS=0
            fi
        else
            # Entware: Try upgrade first, then fallback to direct URL  
            echo "Attempt 1: Upgrading from Repository..."
            if $OPKG_BIN upgrade magitrickle_mod 2>&1 | awk '!/has no valid architecture/'; then
                INSTALL_SUCCESS=1
                echo "✓ Repository upgrade successful!"
            else
                echo "⚠ Repository upgrade failed."
                echo "Attempt 2: Fallback to Direct Download..."
                echo "Source: $DIRECT_URL"
                
                if $OPKG_BIN install "$DIRECT_URL" --force-checksum --force-reinstall; then
                    INSTALL_SUCCESS=1
                    echo "✓ Direct update successful!"
                else
                    echo "❌ Update failed."
                    INSTALL_SUCCESS=0
                fi
            fi
        fi
    fi

    # --- 3. Post-Install Actions ---
    if [ "$INSTALL_SUCCESS" -eq 1 ]; then
        echo ""
        echo "✓ Operation successful!"
        echo "Starting service..."
        
        # Enable service (autostart on boot)
        if [ "$IS_OPENWRT" -eq 1 ]; then
             $INIT_SCRIPT enable
        fi
        
        # Restart the service
        $INIT_SCRIPT stop >/dev/null 2>&1
        $INIT_SCRIPT start
        
        echo "✓ Service started!"
        
        ROUTER_IP=$(echo $SSH_CONNECTION | awk '{print $3}')
        if [ -z "$ROUTER_IP" ]; then
            ROUTER_IP="your_router_ip"
        fi
        
        echo "Access web interface at: http://${ROUTER_IP}:8080"
    else
        echo ""
        echo "Error: Package operation failed."
        echo "Architecture '$ARCH' ($PLATFORM_NAME) should be supported."
        echo "Check releases at: https://github.com/LarinIvan/MagiTrickle_Mod/releases"
        exit 1
    fi

else
    # Already installed and up to date
    echo ""
    echo "✓ System is up to date."
fi