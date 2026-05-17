#!/bin/bash
# FileSearch — script de lancement (WSL)
# Usage: ./start.sh [DIR] [PORT]
# - Utilise le binaire compilé ./filesearch-server si présent
# - Sinon, fallback sur go run

DIR="${1:-}"
PORT="${2:-8080}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$SCRIPT_DIR/filesearch-server"
CFG="$SCRIPT_DIR/data/config.json"

echo "══════════════════════════════"
echo "  FileSearch — Lancement"
echo "══════════════════════════════"

# Arrêter l'ancienne instance si elle tourne
if pgrep -f filesearch-server > /dev/null 2>&1; then
    echo "Arrêt de l'ancienne instance..."
    pkill -f filesearch-server
    sleep 1
fi

mkdir -p "$SCRIPT_DIR/data"

# Choisir le mode de lancement
if [ -f "$BIN" ]; then
    echo "Binaire compilé trouvé ✓"
    LAUNCH_CMD="$BIN -config $CFG"
else
    echo "Binaire absent — utilisation de go run"
    LAUNCH_CMD="go run $SCRIPT_DIR/cmd/server/main.go -config $CFG"
fi

# Ajouter le dossier si passé en argument
if [ -n "$DIR" ]; then
    LAUNCH_CMD="$LAUNCH_CMD -dir $DIR"
fi

echo "Port: $PORT"
echo "Config: $CFG"
echo ""

exec $LAUNCH_CMD -port $PORT
