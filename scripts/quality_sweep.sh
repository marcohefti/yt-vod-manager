#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

stamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
day="$(date -u +%Y-%m-%d)"
report="docs/exec-plans/active/quality-sweep-${day}.md"

run_check() {
  local name="$1"
  shift
  local out
  if out="$("$@" 2>&1)"; then
    echo "- ${name}: PASS" >> "$report"
  else
    echo "- ${name}: FAIL" >> "$report"
    {
      echo
      echo "### ${name} output"
      echo
      echo '```text'
      echo "$out"
      echo '```'
    } >> "$report"
  fi
}

cat > "$report" <<EOF
# Quality Sweep ${day}

Last Reviewed: ${day}

Generated at: ${stamp}

## Automated Checks

EOF

run_check "go test ./..." go test ./...
run_check "architecture boundaries" go run ./scripts/check_arch_boundaries.go
run_check "documentation checks" ./scripts/check_docs.sh
run_check "golden rules" ./scripts/check_golden_rules.sh
run_check "go mod tidy" go mod tidy

if rg -q '^Last Sweep:' docs/QUALITY_SCORE.md; then
  if sed --version >/dev/null 2>&1; then
    sed -i -E "s/^Last Sweep: .*/Last Sweep: ${day}/" docs/QUALITY_SCORE.md
  else
    sed -i '' -E "s/^Last Sweep: .*/Last Sweep: ${day}/" docs/QUALITY_SCORE.md
  fi
fi

echo >> "$report"
echo "## Notes" >> "$report"
echo >> "$report"
echo "- Review failing checks and open targeted follow-up plans under docs/exec-plans/active." >> "$report"
