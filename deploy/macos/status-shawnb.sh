#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
RUNTIME_DIR="${TRISHNA_RUNTIME_DIR:-$HOME/Library/Application Support/trishna-go}"
PLIST_LABEL="com.radcolor.shawnb"

echo "== launchctl =="
launchctl print "gui/$(id -u)/$PLIST_LABEL" 2>/dev/null | rg "state =|pid =|last exit code =|runs =|path =" || echo "not loaded"

echo
echo "== recent logs =="
if [ -f "$RUNTIME_DIR/logs/shawnb.log" ]; then
  tail -n 20 "$RUNTIME_DIR/logs/shawnb.log"
elif [ -f "$INSTALL_DIR/logs/shawnb.log" ]; then
  tail -n 20 "$INSTALL_DIR/logs/shawnb.log"
else
  echo "no stdout log yet"
fi

echo
if [ -f "$RUNTIME_DIR/logs/shawnb.error.log" ] && [ -s "$RUNTIME_DIR/logs/shawnb.error.log" ]; then
  echo "== recent errors =="
  tail -n 20 "$RUNTIME_DIR/logs/shawnb.error.log"
elif [ -f "$INSTALL_DIR/logs/shawnb.error.log" ] && [ -s "$INSTALL_DIR/logs/shawnb.error.log" ]; then
  echo "== recent errors =="
  tail -n 20 "$INSTALL_DIR/logs/shawnb.error.log"
fi
