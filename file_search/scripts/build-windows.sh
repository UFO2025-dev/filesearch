#!/usr/bin/env bash
# build-windows.sh — Cross-compile FileSearch for Windows with embedded manifest
# Run from the file_search/ directory on Linux/WSL.
# Requires: go, windres (sudo apt install mingw-w64)

set -euo pipefail

VERSION=${VERSION:-1.0.1}
OUT=filesearch.exe

echo "=== FileSearch Windows Build ==="
echo "Version: $VERSION"
echo ""

# 1. Compile resource file (manifest + version info) into a .syso object.
#    The .syso file is automatically linked by the Go toolchain.
if command -v x86_64-w64-mingw32-windres &>/dev/null; then
    echo "[1/3] Compiling Windows resources..."
    x86_64-w64-mingw32-windres         -i cmd/server/filesearch.rc         -o cmd/server/filesearch_windows.syso         --target=pe-x86-64
    echo "      Resources compiled -> cmd/server/filesearch_windows.syso"
else
    echo "[1/3] windres not found — skipping resource embedding."
    echo "      Install with: sudo apt install mingw-w64"
    echo "      Binary will work but won't have version info / manifest embedded."
fi

# 2. Build the Windows binary.
echo "[2/3] Building Windows executable..."
GOOS=windows GOARCH=amd64 \
    go build \
        -ldflags="-s -w -H windowsgui -X main.Version=$VERSION" \
        -o "$OUT" \
        ./cmd/server

echo "      Built: $OUT ($(du -sh $OUT | cut -f1))"

# 3. Quick sanity check.
echo "[3/3] Sanity check..."
file "$OUT"

echo ""
echo "=== Build complete: $OUT ==="
echo ""
echo "Next steps:"
echo "  1. Copy $OUT to Windows"
echo "  2. Open installer/ in Inno Setup 6 and compile installer.iss"
echo "  3. Distribute FileSearch-Setup-v$VERSION.exe"
