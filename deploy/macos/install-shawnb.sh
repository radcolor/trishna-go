#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
PLIST_LABEL="com.radcolor.shawnb"
PLIST_DEST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
ENV_BLOCK_FILE="$(mktemp)"
trap 'rm -f "$ENV_BLOCK_FILE"' EXIT

echo "Installing shawnb AI chat bot for macOS launchd"
echo "  install dir: $INSTALL_DIR"

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

mkdir -p "$INSTALL_DIR/dist" "$INSTALL_DIR/logs" "$INSTALL_DIR/data/shawnb/chats"
rm -f "$INSTALL_DIR/logs/shawnb.log" "$INSTALL_DIR/logs/shawnb.error.log"
xattr -cr "$INSTALL_DIR/logs" 2>/dev/null || true
chmod 700 "$INSTALL_DIR/data/shawnb" "$INSTALL_DIR/data/shawnb/chats" 2>/dev/null || true

echo "Building binary..."
(
  cd "$INSTALL_DIR"
  LDFLAGS="$(sh "$INSTALL_DIR/scripts/ldflags.sh")"
  go build -trimpath -ldflags="$LDFLAGS" -o "$INSTALL_DIR/dist/shawnb" ./cmd/shawnb
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
echo "  logs: $INSTALL_DIR/logs/shawnb.log"
echo "  errors: $INSTALL_DIR/logs/shawnb.error.log"
echo "  chat logs: $INSTALL_DIR/data/shawnb/chats/"
echo
echo "Useful commands:"
echo "  ./deploy/macos/status-shawnb.sh"
echo "  tail -f $INSTALL_DIR/logs/shawnb.log"
echo "  tail -f $INSTALL_DIR/data/shawnb/chats/\$(date +%F).jsonl"
echo "  ./deploy/macos/restart-shawnb.sh"
echo "  ./deploy/macos/uninstall-shawnb.sh"
