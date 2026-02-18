#!/usr/bin/env bash
set -euo pipefail

# Background-friendly sync runner.
# Override paths with:
#   YTVM_BIN, YTVM_CONFIG, YTVM_RUNS_DIR, YTVM_LOCKDIR

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${YTVM_BIN:-$ROOT_DIR/bin/yt-vod-manager}"
CONFIG="${YTVM_CONFIG:-$ROOT_DIR/config/projects.json}"
RUNS_DIR="${YTVM_RUNS_DIR:-$ROOT_DIR/runs}"
LOCKDIR="${YTVM_LOCKDIR:-$ROOT_DIR/.sync.lock}"

if [[ ! -x "$BIN" ]]; then
  echo "yt-vod-manager binary not found or not executable: $BIN" >&2
  echo "build it first: go build -o bin/yt-vod-manager ./cmd/yt-vod-manager" >&2
  exit 1
fi

if ! mkdir "$LOCKDIR" 2>/dev/null; then
  echo "sync already running; skip"
  exit 0
fi

cleanup() {
  rmdir "$LOCKDIR" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

cd "$ROOT_DIR"
"$BIN" sync \
  --all-projects \
  --active-only \
  --config "$CONFIG" \
  --runs-dir "$RUNS_DIR" \
  --progress=false \
  "$@"
