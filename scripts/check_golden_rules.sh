#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

errors=0

echo "golden rule: no exec.Command outside internal/ytdlp"
if rg -n 'exec\.Command\(' internal --glob '*.go' --glob '!internal/ytdlp/**' --glob '!**/*_test.go' >/tmp/ytvod_exec_violations.txt; then
  cat /tmp/ytvod_exec_violations.txt >&2
  errors=$((errors + 1))
fi

echo "golden rule: no raw job status literals outside model/status.go"
if rg -n '"pending"|"running"|"completed"|"failed_retryable"|"failed_permanent"|"skipped_private"' \
  internal/archive internal/discovery internal/cli internal/ytdlp \
  --glob '*.go' --glob '!**/*_test.go' >/tmp/ytvod_status_violations.txt; then
  cat /tmp/ytvod_status_violations.txt >&2
  errors=$((errors + 1))
fi

MAX_INTERNAL_LINES="${MAX_INTERNAL_LINES:-900}"
echo "golden rule: internal files must be <= ${MAX_INTERNAL_LINES} lines"
while IFS= read -r f; do
  lines="$(wc -l < "$f" | tr -d ' ')"
  if (( lines > MAX_INTERNAL_LINES )); then
    echo "$f:$lines exceeds ${MAX_INTERNAL_LINES} line limit" >&2
    errors=$((errors + 1))
  fi
done < <(find internal -type f -name '*.go' -not -name '*_test.go' | sort)

if (( errors > 0 )); then
  echo "golden rules failed with $errors violation bucket(s)" >&2
  exit 1
fi

echo "golden rules: OK"
