#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

VERSION="$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo none)"
BUILT_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

printf '%s' "-s -w \
  -X github.com/radcolor/trishna-go/internal/buildinfo.Version=${VERSION} \
  -X github.com/radcolor/trishna-go/internal/buildinfo.Commit=${COMMIT} \
  -X github.com/radcolor/trishna-go/internal/buildinfo.BuiltAt=${BUILT_AT}"
