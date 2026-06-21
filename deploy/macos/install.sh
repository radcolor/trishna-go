#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
RUNTIME_DIR="${TRISHNA_RUNTIME_DIR:-$HOME/Library/Application Support/trishna-go}"
PLIST_LABEL="com.radcolor.trishna"
PLIST_DEST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"

echo "Installing Trishna for macOS launchd"

if [ ! -f "$INSTALL_DIR/.env" ]; then
  echo "error: $INSTALL_DIR/.env not found"
  echo "copy .env.example to .env and fill in your secrets first"
  exit 1
fi

mkdir -p "$RUNTIME_DIR/dist" "$RUNTIME_DIR/logs" "$RUNTIME_DIR/data/telegram"
rm -f "$RUNTIME_DIR/logs/trishna.log" "$RUNTIME_DIR/logs/trishna.error.log"
cp "$INSTALL_DIR/.env" "$RUNTIME_DIR/.env"
if [ -f "$INSTALL_DIR/SOUL.md" ]; then
  cp "$INSTALL_DIR/SOUL.md" "$RUNTIME_DIR/SOUL.md"
fi
if [ -d "$INSTALL_DIR/data" ]; then
  ditto "$INSTALL_DIR/data" "$RUNTIME_DIR/data"
fi
xattr -cr "$RUNTIME_DIR" 2>/dev/null || true
chmod 600 "$RUNTIME_DIR/.env"
chmod 700 "$RUNTIME_DIR/data" "$RUNTIME_DIR/data/telegram" 2>/dev/null || true

echo "Building binary..."
(
  cd "$INSTALL_DIR"
  LDFLAGS="$(sh "$INSTALL_DIR/scripts/ldflags.sh")"
  go build -trimpath -ldflags="$LDFLAGS" -o "$RUNTIME_DIR/dist/trishna" ./cmd/trishna
)

echo "Writing launch agent..."
"$SCRIPT_DIR/sync-trishna-plist.sh"

launchctl bootout "gui/$(id -u)/$PLIST_LABEL" 2>/dev/null || \
  launchctl unload "$PLIST_DEST" 2>/dev/null || true

sleep 1

if ! launchctl bootstrap "gui/$(id -u)" "$PLIST_DEST" 2>/dev/null; then
  launchctl load "$PLIST_DEST"
fi

if ! launchctl print "gui/$(id -u)/$PLIST_LABEL" >/dev/null 2>&1; then
  echo "error: launch agent failed to load; try: launchctl bootstrap gui/$(id -u) $PLIST_DEST"
  exit 1
fi

echo
echo "Trishna installed and started."
echo "  runtime: $RUNTIME_DIR"
echo "  logs: $RUNTIME_DIR/logs/trishna.log"
echo "  errors: $RUNTIME_DIR/logs/trishna.error.log"
echo "  state: $RUNTIME_DIR/data/youtube-state.json"
echo
echo "Useful commands:"
echo "  ./deploy/macos/status.sh"
echo "  tail -f \"$RUNTIME_DIR/logs/trishna.log\""
echo "  ./deploy/macos/restart.sh"
echo "  ./deploy/macos/uninstall.sh"
