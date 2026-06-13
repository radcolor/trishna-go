#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
PLIST_LABEL="com.radcolor.shawnb"
PLIST_DEST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"

echo "Rebuilding shawnb..."
(
  cd "$INSTALL_DIR"
  LDFLAGS="$(sh "$INSTALL_DIR/scripts/ldflags.sh")"
  go build -trimpath -ldflags="$LDFLAGS" -o "$INSTALL_DIR/dist/shawnb" ./cmd/shawnb
)

echo "Syncing launch agent env from .env..."
"$SCRIPT_DIR/sync-shawnb-plist.sh"

echo "Restarting launch agent..."
launchctl bootout "gui/$(id -u)/$PLIST_LABEL" 2>/dev/null || \
  launchctl unload "$PLIST_DEST" 2>/dev/null || true
sleep 1
if ! launchctl bootstrap "gui/$(id -u)" "$PLIST_DEST" 2>/dev/null; then
  launchctl load "$PLIST_DEST"
fi

echo "shawnb restarted."
