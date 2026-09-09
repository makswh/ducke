#!/usr/bin/env bash
set -e

export PATH="/usr/local/go/bin:/usr/local/node/bin:/root/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="${1:-$SCRIPT_DIR}"

echo "===================================================="
echo "=== Step 1: Toolchain Verification               ==="
echo "===================================================="
echo "Go:          $(go version)"
echo "Wails:       $(wails version)"
echo "GCC:         $(gcc --version | head -n 1)"
echo "WebKit:      $(pkg-config --modversion webkit2gtk-4.1)"
echo "Flatpak:     $(flatpak --version)"
echo "Project Dir: ${PROJECT_DIR}"

echo ""
echo "===================================================="
echo "=== Step 2: Compiling Linux Binary (AMD64)       ==="
echo "===================================================="
cd "${PROJECT_DIR}"
wails build -platform linux/amd64 -tags webkit2_41 -s -clean=false

echo ""
echo "===================================================="
echo "=== Step 3: Verifying Linux Binary               ==="
echo "===================================================="
ls -lh "${PROJECT_DIR}/build/bin/Ducke"
file "${PROJECT_DIR}/build/bin/Ducke" || true
ldd "${PROJECT_DIR}/build/bin/Ducke" | grep -i webkit || true

echo ""
echo "===================================================="
echo "=== Step 4: Building Flatpak Bundle              ==="
echo "===================================================="
FLATPAK_WORK_DIR="/var/tmp/ducke-flatpak"
MANIFEST_PATH="${PROJECT_DIR}/build/linux/flatpak/com.ducke.Ducke.yml"
BUNDLE_OUTPUT="${PROJECT_DIR}/build/bin/Ducke.flatpak"

rm -rf "${FLATPAK_WORK_DIR}"
mkdir -p "${FLATPAK_WORK_DIR}/build" "${FLATPAK_WORK_DIR}/repo" "${FLATPAK_WORK_DIR}/state"

echo "Running flatpak-builder..."
flatpak-builder --force-clean \
  --state-dir="${FLATPAK_WORK_DIR}/state" \
  --repo="${FLATPAK_WORK_DIR}/repo" \
  "${FLATPAK_WORK_DIR}/build" \
  "${MANIFEST_PATH}"

echo "Creating Flatpak bundle (${BUNDLE_OUTPUT})..."
flatpak build-bundle --runtime-repo="https://dl.flathub.org/repo/flathub.flatpakrepo" "${FLATPAK_WORK_DIR}/repo" "${BUNDLE_OUTPUT}" com.ducke.Ducke

# Ensure Steam Deck installer script is copied and executable
cp -f "${PROJECT_DIR}/build/linux/install-flatpak.sh" "${PROJECT_DIR}/build/bin/install_steamdeck.sh" 2>/dev/null || true
chmod +x "${PROJECT_DIR}/build/bin/install_steamdeck.sh" 2>/dev/null || true

# Clean up temp working files in WSL
rm -rf "${FLATPAK_WORK_DIR}"

echo ""
echo "===================================================="
echo "=== Build Complete!                              ==="
echo "===================================================="
echo "Native Binary:     ${PROJECT_DIR}/build/bin/Ducke"
echo "Flatpak Bundle:    ${BUNDLE_OUTPUT}"
echo "SteamDeck Helper:  ${PROJECT_DIR}/build/bin/install_steamdeck.sh"
ls -lh "${PROJECT_DIR}/build/bin/Ducke" "${BUNDLE_OUTPUT}" "${PROJECT_DIR}/build/bin/install_steamdeck.sh"