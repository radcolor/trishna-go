#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
PLIST_LABEL="com.radcolor.trishna"
PLIST_DEST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
ENV_BLOCK_FILE="$(mktemp)"
trap 'rm -f "$ENV_BLOCK_FILE"' EXIT

echo "Installing Trishna for macOS launchd"
echo "  install dir: $INSTALL_DIR"

if [ ! -f "$INSTALL_DIR/.env" ]; then
  echo "error: $INSTALL_DIR/.env not found"
  echo "copy .env.example to .env and fill in your secrets first"
  exit 1
fi

mkdir -p "$INSTALL_DIR/dist" "$INSTALL_DIR/logs" "$INSTALL_DIR/data"
rm -f "$INSTALL_DIR/logs/trishna.log" "$INSTALL_DIR/logs/trishna.error.log"
xattr -cr "$INSTALL_DIR/logs" 2>/dev/null || true

python3 - "$INSTALL_DIR/.env" > "$ENV_BLOCK_FILE" <<'PY'
import plistlib
import sys
import xml.sax.saxutils as xml

env = {}
for raw in open(sys.argv[1], encoding="utf-8"):
    line = raw.strip()
    if not line or line.startswith("#"):
        continue
    key, _, value = line.partition("=")
    key = key.strip()
    value = value.strip()
    if not key:
        continue
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
        value = value[1:-1]
    if not value:
        continue
    env[key] = value

for key, value in env.items():
    print(f"\t\t<key>{xml.escape(key)}</key>")
    print(f"\t\t<string>{xml.escape(value)}</string>")
PY

echo "Building binary..."
(
  cd "$INSTALL_DIR"
  LDFLAGS="$(sh "$INSTALL_DIR/scripts/ldflags.sh")"
  go build -trimpath -ldflags="$LDFLAGS" -o "$INSTALL_DIR/dist/trishna" ./cmd/trishna
)

echo "Writing launch agent..."
sed \
  -e "s|@@INSTALL_DIR@@|$INSTALL_DIR|g" \
  -e "/@@ENV_VARS@@/r $ENV_BLOCK_FILE" \
  -e "/@@ENV_VARS@@/d" \
  "$SCRIPT_DIR/com.radcolor.trishna.plist" > "$PLIST_DEST"

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
echo "Trishna installed and started."
echo "  logs: $INSTALL_DIR/logs/trishna.log"
echo "  errors: $INSTALL_DIR/logs/trishna.error.log"
echo "  state: $INSTALL_DIR/data/youtube-state.json"
echo
echo "Useful commands:"
echo "  ./deploy/macos/status.sh"
echo "  tail -f $INSTALL_DIR/logs/trishna.log"
echo "  ./deploy/macos/restart.sh"
echo "  ./deploy/macos/uninstall.sh"
