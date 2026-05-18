#!/usr/bin/env bash
# run_all.sh — Run all benchmarks and save results.
set -euo pipefail

RESULTS_DIR="$(dirname "$0")/../results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUTFILE="$RESULTS_DIR/${TIMESTAMP}_$(uname -m).txt"
mkdir -p "$RESULTS_DIR"

echo "=== FileSearch Benchmark Suite ==="
echo "Machine: $(uname -a)"
echo "Go: $(go version)"
echo "Time: $(date -u)"
echo ""

# Drop OS cache if running as root (CI)
if [[ $EUID -eq 0 ]]; then
  sync && echo 3 > /proc/sys/vm/drop_caches
  echo "OS page cache dropped."
fi

go test \
  -run='^$' \
  -bench='.' \
  -benchmem \
  -benchtime=5s \
  -count=3 \
  ./benchmarks/ \
  | tee "$OUTFILE"

echo ""
echo "Results saved to: $OUTFILE"

# If baseline exists, compare
BASELINE="$(dirname "$0")/../results/baseline_linux.txt"
if [[ -f "$BASELINE" ]]; then
  echo ""
  echo "=== Comparing with baseline ==="
  python3 "$(dirname "$0")/compare_baseline.py" "$BASELINE" "$OUTFILE" || true
fi
