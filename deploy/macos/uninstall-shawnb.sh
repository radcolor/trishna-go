#!/usr/bin/env bash
set -euo pipefail

PLIST_LABEL="com.radcolor.shawnb"
PLIST_DEST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"

echo "Stopping shawnb launch agent..."

launchctl bootout "gui/$(id -u)/$PLIST_LABEL" 2>/dev/null || \
  launchctl unload "$PLIST_DEST" 2>/dev/null || true

if [ -f "$PLIST_DEST" ]; then
  rm "$PLIST_DEST"
fi

echo "shawnb launch agent removed."
echo "Runtime binary, logs, and chat data were left in place."
