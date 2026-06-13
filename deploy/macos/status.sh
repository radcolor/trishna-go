#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
PLIST_LABEL="com.radcolor.trishna"

echo "== launchctl =="
launchctl print "gui/$(id -u)/$PLIST_LABEL" 2>/dev/null | rg "state =|pid =|last exit code =|runs =|path =" || echo "not loaded"

echo
echo "== recent logs =="
if [ -f "$INSTALL_DIR/logs/trishna.log" ]; then
  tail -n 20 "$INSTALL_DIR/logs/trishna.log"
else
  echo "no stdout log yet"
fi

echo
if [ -f "$INSTALL_DIR/logs/trishna.error.log" ] && [ -s "$INSTALL_DIR/logs/trishna.error.log" ]; then
  echo "== recent errors =="
  tail -n 20 "$INSTALL_DIR/logs/trishna.error.log"
fi
