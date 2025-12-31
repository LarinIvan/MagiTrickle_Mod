#!/bin/sh

# Check for OPKG (Entware)
if [ ! -f /opt/bin/opkg ]; then
    echo "Error: OPKG (Entware) not found. Please install Entware first."
    exit 1
fi

# Detect Architecture
# filters for the highest priority architecture
ARCH=$(opkg print-architecture | awk '{print $3, $2}' | sort -n | tail -n1 | awk '{print $2}')

if [ -z "$ARCH" ]; then
    echo "Error: Could not detect architecture."
    exit 1
fi

echo "Detected architecture: $ARCH"

# Create config directory if missing
mkdir -p /opt/etc/opkg

# Add Repository
REPO_URL="https://github.com/LarinIvan/MagiTrickle_Mod/releases/latest/download"
CONF_FILE="/opt/etc/opkg/magitrickle_mod.conf"

echo "src/gz magitrickle_mod $REPO_URL" > "$CONF_FILE"

echo "Repository successfully added to $CONF_FILE"
echo ""
echo "Now run:"
echo "opkg update"
echo "opkg remove magitrickle  # Remove original if exists"
echo "opkg install magitrickle_mod"
