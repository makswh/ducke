#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_FILE="$SCRIPT_DIR/Ducke.flatpak"

echo "============================================================"
echo "      Ducke Installer for SteamOS / Steam Deck"
echo "============================================================"
echo ""

# 1. Ensure Flathub remote is configured
echo "[1/3] Checking Flathub repository..."
flatpak remote-add --if-not-exists --user flathub https://dl.flathub.org/repo/flathub.flatpakrepo || true

# Mask problematic Cisco OpenH264 (often geo-blocked or causes download timeouts; not needed by Ducke)
flatpak mask --user org.freedesktop.Platform.openh264 2>/dev/null || true

# 2. Check and install GNOME 46 Platform if missing
echo "[2/3] Checking GNOME 46 Platform..."
if ! flatpak info org.gnome.Platform//46 >/dev/null 2>&1; then
    echo "Installing org.gnome.Platform//46 from Flathub..."
    flatpak install -y --user flathub org.gnome.Platform//46 || flatpak install -y --user --no-related flathub org.gnome.Platform//46
else
    echo "GNOME 46 Platform already installed."
fi

# 3. Install Ducke Flatpak
echo "[3/3] Installing Ducke Flatpak..."
if [ -f "$BUNDLE_FILE" ]; then
    flatpak install -y --user "$BUNDLE_FILE"
elif [ -f "$HOME/Downloads/Ducke.flatpak" ]; then
    flatpak install -y --user "$HOME/Downloads/Ducke.flatpak"
elif [ -f "Ducke.flatpak" ]; then
    flatpak install -y --user Ducke.flatpak
else
    echo "Error: Ducke.flatpak not found in $SCRIPT_DIR!"
    exit 1
fi

echo ""
echo "============================================================"
echo " Ducke installed successfully!"
echo "============================================================"
echo ""
echo "To add Ducke to Game Mode in SteamOS:"
echo "1. In Desktop Mode, open the Steam client."
echo "2. In top menu click: Games -> 'Add a Non-Steam Game to My Library...'"
echo "3. Check 'Ducke' in the list and click 'Add Selected Programs'."
echo "4. Return to Gaming Mode and launch Ducke!"
echo ""
read -p "Press Enter to exit..."
