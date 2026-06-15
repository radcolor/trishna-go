#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
RUNTIME_DIR="${TRISHNA_RUNTIME_DIR:-$HOME/Library/Application Support/trishna-go}"
PLIST_LABEL="com.radcolor.trishna"
PLIST_DEST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"

echo "Rebuilding Trishna..."
mkdir -p "$RUNTIME_DIR/dist" "$RUNTIME_DIR/logs" "$RUNTIME_DIR/data"
cp "$INSTALL_DIR/.env" "$RUNTIME_DIR/.env"
if [ -f "$INSTALL_DIR/SOUL.md" ]; then
  cp "$INSTALL_DIR/SOUL.md" "$RUNTIME_DIR/SOUL.md"
fi
if [ -d "$INSTALL_DIR/data" ]; then
  ditto "$INSTALL_DIR/data" "$RUNTIME_DIR/data"
fi
xattr -cr "$RUNTIME_DIR" 2>/dev/null || true
(
  cd "$INSTALL_DIR"
  LDFLAGS="$(sh "$INSTALL_DIR/scripts/ldflags.sh")"
  go build -trimpath -ldflags="$LDFLAGS" -o "$RUNTIME_DIR/dist/trishna" ./cmd/trishna
)

echo "Syncing launch agent env from .env..."
"$SCRIPT_DIR/sync-trishna-plist.sh"

echo "Restarting launch agent..."
launchctl bootout "gui/$(id -u)/$PLIST_LABEL" 2>/dev/null || \
  launchctl unload "$PLIST_DEST" 2>/dev/null || true
sleep 1
if ! launchctl bootstrap "gui/$(id -u)" "$PLIST_DEST" 2>/dev/null; then
  launchctl load "$PLIST_DEST"
fi

echo "Trishna restarted."
