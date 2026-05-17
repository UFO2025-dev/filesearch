#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/src-tauri/binaries"
mkdir -p "$OUT"

# Locate go binary
GO_BIN="${GOPATH:-$HOME/go}/bin/go"
if [ ! -x "$GO_BIN" ]; then
    GO_BIN="$(which go 2>/dev/null || true)"
fi
if [ ! -x "$GO_BIN" ]; then
    echo "[build-go] ERROR: go not found. Set GOPATH or add go to PATH." >&2
    exit 1
fi

echo "[build-go] Using Go: $GO_BIN ($($GO_BIN version))"
echo "[build-go] Compiling Go server..."
cd "$ROOT"

GOOS=linux GOARCH=amd64 "$GO_BIN" build -trimpath -ldflags="-s -w" \
    -o "$OUT/filesearch-server-x86_64-unknown-linux-gnu" ./cmd/server/

echo "[build-go] Done -> $OUT/"
ls -lh "$OUT/"
