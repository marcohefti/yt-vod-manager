#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLS_BIN="${ROOT_DIR}/bin/tools"

mkdir -p "$TOOLS_BIN"

echo "installing golangci-lint to ${TOOLS_BIN}"
GOBIN="$TOOLS_BIN" go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

echo "installed:"
"${TOOLS_BIN}/golangci-lint" version
