#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

LINT_BIN=""
if command -v golangci-lint >/dev/null 2>&1; then
  LINT_BIN="$(command -v golangci-lint)"
elif [[ -x "${ROOT_DIR}/bin/tools/golangci-lint" ]]; then
  LINT_BIN="${ROOT_DIR}/bin/tools/golangci-lint"
else
  echo "golangci-lint not found; installing local dev tools"
  ./scripts/install_dev_tools.sh
  LINT_BIN="${ROOT_DIR}/bin/tools/golangci-lint"
fi

echo "==> go test ./..."
go test ./...

echo "==> go run ./scripts/check_arch_boundaries.go"
go run ./scripts/check_arch_boundaries.go

echo "==> ./scripts/check_golden_rules.sh"
./scripts/check_golden_rules.sh

echo "==> ${LINT_BIN} run ./..."
"${LINT_BIN}" run ./...

echo "==> go build -o bin/yt-vod-manager ./cmd/yt-vod-manager"
go build -o bin/yt-vod-manager ./cmd/yt-vod-manager

echo "verify: OK"
