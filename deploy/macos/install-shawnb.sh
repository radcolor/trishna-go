#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
RUNTIME_DIR="${TRISHNA_RUNTIME_DIR:-$HOME/Library/Application Support/trishna-go}"
PLIST_LABEL="com.radcolor.shawnb"
PLIST_DEST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"

echo "Installing shawnb AI chat bot for macOS launchd"

if [ ! -f "$INSTALL_DIR/.env" ]; then
  echo "error: $INSTALL_DIR/.env not found"
  echo "copy .env.example to .env and fill in your secrets first"
  exit 1
fi

if [ ! -f "$INSTALL_DIR/SOUL.md" ]; then
  echo "error: $INSTALL_DIR/SOUL.md not found"
  echo "copy SOUL.md.example to SOUL.md and fill in your personality details first"
  exit 1
fi

mkdir -p "$RUNTIME_DIR/dist" "$RUNTIME_DIR/logs" "$RUNTIME_DIR/data/shawnb/chats"
rm -f "$RUNTIME_DIR/logs/shawnb.log" "$RUNTIME_DIR/logs/shawnb.error.log"
cp "$INSTALL_DIR/.env" "$RUNTIME_DIR/.env"
cp "$INSTALL_DIR/SOUL.md" "$RUNTIME_DIR/SOUL.md"
if [ -d "$INSTALL_DIR/data/shawnb" ]; then
  ditto "$INSTALL_DIR/data/shawnb" "$RUNTIME_DIR/data/shawnb"
fi
xattr -cr "$RUNTIME_DIR" 2>/dev/null || true
chmod 700 "$RUNTIME_DIR/data/shawnb" "$RUNTIME_DIR/data/shawnb/chats" 2>/dev/null || true

echo "Building binary..."
(
  cd "$INSTALL_DIR"
  LDFLAGS="$(sh "$INSTALL_DIR/scripts/ldflags.sh")"
  go build -trimpath -ldflags="$LDFLAGS" -o "$RUNTIME_DIR/dist/shawnb" ./cmd/shawnb
)

echo "Writing launch agent..."
"$SCRIPT_DIR/sync-shawnb-plist.sh"

chmod 600 "$PLIST_DEST"

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
echo "shawnb installed and started."
echo "  runtime: $RUNTIME_DIR"
echo "  logs: $RUNTIME_DIR/logs/shawnb.log"
echo "  errors: $RUNTIME_DIR/logs/shawnb.error.log"
echo "  chat logs: $RUNTIME_DIR/data/shawnb/chats/"
echo
echo "Useful commands:"
echo "  ./deploy/macos/status-shawnb.sh"
echo "  tail -f \"$RUNTIME_DIR/logs/shawnb.log\""
echo "  tail -f \"$RUNTIME_DIR/data/shawnb/chats/\$(date +%F).jsonl\""
echo "  ./deploy/macos/restart-shawnb.sh"
echo "  ./deploy/macos/uninstall-shawnb.sh"
