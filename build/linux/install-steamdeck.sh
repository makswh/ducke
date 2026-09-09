#!/usr/bin/env bash
set -e

# Ducke - SteamOS / Steam Deck Installation Script
# Installs binary to ~/.local/bin, desktop entry to ~/.local/share/applications, and icon to ~/.local/share/icons

APP_NAME="Ducke"
INSTALL_DIR="$HOME/.local/bin"
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_PATH="$SCRIPT_DIR/../bin/Ducke"

if [ ! -f "$BIN_PATH" ]; then
    # If run from release root
    BIN_PATH="$SCRIPT_DIR/Ducke"
fi

if [ ! -f "$BIN_PATH" ]; then
    echo "Error: Ducke binary not found! Please build it first with 'wails build -platform linux/amd64'."
    exit 1
fi

echo "==> Installing Ducke for SteamOS / Steam Deck..."

# 1. Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$DESKTOP_DIR"
mkdir -p "$ICON_DIR"

# 2. Copy binary
echo "--> Installing binary to $INSTALL_DIR/Ducke"
cp -f "$BIN_PATH" "$INSTALL_DIR/Ducke"
chmod +x "$INSTALL_DIR/Ducke"

# 3. Copy app icon
if [ -f "$SCRIPT_DIR/../appicon.png" ]; then
    cp -f "$SCRIPT_DIR/../appicon.png" "$ICON_DIR/ducke.png"
elif [ -f "$SCRIPT_DIR/ducke.png" ]; then
    cp -f "$SCRIPT_DIR/ducke.png" "$ICON_DIR/ducke.png"
fi

# 4. Install Desktop Entry
cat <<EOF > "$DESKTOP_DIR/Ducke.desktop"
[Desktop Entry]
Name=Ducke
Comment=Remote Game Repository Downloader & Steam Metadata Hub
Exec=$INSTALL_DIR/Ducke
Icon=ducke
Terminal=false
Type=Application
Categories=Game;Network;FileTransfer;
StartupWMClass=Ducke
PrefersNonDefaultGPU=true
X-KDE-SubstituteUID=false
EOF
chmod +x "$DESKTOP_DIR/Ducke.desktop"

# 5. Update desktop database
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$DESKTOP_DIR" || true
fi

echo ""
echo "============================================================"
echo " [SUCCESS] Ducke installed successfully!"
echo "============================================================"
echo " - Executable: $INSTALL_DIR/Ducke"
echo " - Desktop Entry: $DESKTOP_DIR/Ducke.desktop"
echo ""
echo " To add to Steam Game Mode on Steam Deck:"
echo " 1. Open Steam in Desktop Mode"
echo " 2. Click 'Games' menu -> 'Add a Non-Steam Game to My Library...'"
echo " 3. Select 'Ducke' from the list and click 'Add Selected Programs'"
echo " 4. Switch back to Game Mode and enjoy!"
echo "============================================================"
