#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════
#  IMS High-Throughput Load Test
#  Target: 10,000 requests per second across 10 components
# ═══════════════════════════════════════════════════════════
set -euo pipefail

API_URL="${1:-http://localhost:8080/api/v1/signals}"
TOTAL_SIGNALS="${2:-10000}" # Here instead of 10000, write number of signals you want to send

CONCURRENCY="${3:-100}" # Here instead of 100, write number of concurrent workers you want to use

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"

echo "Compiling and running Go load tester..."
cd "$SCRIPT_DIR"

# Run the Go load tester
go run load_tester.go -url="$API_URL" -n="$TOTAL_SIGNALS" -c="$CONCURRENCY"
