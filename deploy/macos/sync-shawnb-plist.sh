#!/usr/bin/env bash
# Regenerate com.radcolor.shawnb.plist EnvironmentVariables from .env (keeps ProgramArguments etc.)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
PLIST_LABEL="com.radcolor.shawnb"
PLIST_DEST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
ENV_BLOCK_FILE="$(mktemp)"
trap 'rm -f "$ENV_BLOCK_FILE"' EXIT

if [ ! -f "$INSTALL_DIR/.env" ]; then
  echo "error: $INSTALL_DIR/.env not found"
  exit 1
fi

python3 - "$INSTALL_DIR/.env" > "$ENV_BLOCK_FILE" <<'PY'
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

sed \
  -e "s|@@INSTALL_DIR@@|$INSTALL_DIR|g" \
  -e "/@@ENV_VARS@@/r $ENV_BLOCK_FILE" \
  -e "/@@ENV_VARS@@/d" \
  "$SCRIPT_DIR/com.radcolor.shawnb.plist" > "$PLIST_DEST"

chmod 600 "$PLIST_DEST"
