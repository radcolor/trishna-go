#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

"$SCRIPT_DIR/restart.sh"
"$SCRIPT_DIR/restart-shawnb.sh"

echo "Trishna and shawnb restarted."
