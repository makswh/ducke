#!/usr/bin/env bash
set -e

# Ducke - SteamOS / Linux Direct Build Script
# Run this directly on Steam Deck (in Desktop Mode with dev tools) or on an Arch/Ubuntu Linux workstation.

echo "==> Building Ducke for SteamOS (linux/amd64)..."

# Ensure frontend dependencies
if [ ! -d "frontend/node_modules" ]; then
    echo "--> Installing frontend dependencies..."
    cd frontend && npm install && cd ..
fi

# Build native Linux binary using Wails CLI
wails build -platform linux/amd64 -v 2

echo "==> Build complete! Output located at: build/bin/Ducke"
