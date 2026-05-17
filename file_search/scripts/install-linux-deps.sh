#!/usr/bin/env bash
# Install Tauri v2 system dependencies on Ubuntu/Debian.
# Run once: bash scripts/install-linux-deps.sh
set -euo pipefail
echo "[deps] Installing Tauri system libraries..."
sudo apt-get update -qq
sudo apt-get install -y --no-install-recommends \
    libwebkit2gtk-4.1-dev \
    libssl-dev \
    libgtk-3-dev \
    libayatana-appindicator3-dev \
    librsvg2-dev \
    libxdo-dev \
    patchelf \
    pkg-config \
    build-essential
echo "[deps] Done! You can now run: cargo check"
