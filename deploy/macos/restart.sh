#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
PLIST_LABEL="com.radcolor.trishna"

echo "Rebuilding Trishna..."
(
  cd "$INSTALL_DIR"
  LDFLAGS="$(sh "$INSTALL_DIR/scripts/ldflags.sh")"
  go build -trimpath -ldflags="$LDFLAGS" -o "$INSTALL_DIR/dist/trishna" ./cmd/trishna
)

echo "Restarting launch agent..."
launchctl kickstart -k "gui/$(id -u)/$PLIST_LABEL"

echo "Trishna restarted."
