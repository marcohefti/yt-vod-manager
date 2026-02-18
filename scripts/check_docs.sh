#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

DOC_MAX_AGE_DAYS="${DOC_MAX_AGE_DAYS:-60}"
errors=0

date_to_epoch() {
  local d="$1"
  if date -u -d "$d" +%s >/dev/null 2>&1; then
    date -u -d "$d" +%s
    return 0
  fi
  date -j -u -f "%Y-%m-%d" "$d" +%s 2>/dev/null
}

required_docs=(
  "AGENTS.md"
  "docs/ARCHITECTURE.md"
  "docs/GOLDEN_RULES.md"
  "docs/RELIABILITY.md"
  "docs/SECURITY.md"
  "docs/QUALITY_SCORE.md"
  "docs/exec-plans/active/README.md"
  "docs/exec-plans/completed/README.md"
)

echo "checking required docs"
for f in "${required_docs[@]}"; do
  if [[ ! -f "$f" ]]; then
    echo "missing required doc: $f" >&2
    errors=$((errors + 1))
  fi
done

echo "checking freshness stamps"
for f in "${required_docs[@]}"; do
  [[ -f "$f" ]] || continue
  reviewed_line="$(rg -n '^Last Reviewed: [0-9]{4}-[0-9]{2}-[0-9]{2}$' "$f" -o || true)"
  if [[ -z "$reviewed_line" ]]; then
    echo "missing Last Reviewed stamp in $f" >&2
    errors=$((errors + 1))
    continue
  fi
  reviewed_date="$(echo "$reviewed_line" | sed -E 's/.*Last Reviewed: ([0-9]{4}-[0-9]{2}-[0-9]{2})/\1/')"
  now_epoch="$(date -u +%s)"
  reviewed_epoch="$(date_to_epoch "$reviewed_date" || true)"
  if [[ -z "$reviewed_epoch" ]]; then
    echo "invalid Last Reviewed date in $f: $reviewed_date" >&2
    errors=$((errors + 1))
    continue
  fi
  age_days="$(( (now_epoch - reviewed_epoch) / 86400 ))"
  if (( age_days > DOC_MAX_AGE_DAYS )); then
    echo "stale doc ($age_days days): $f" >&2
    errors=$((errors + 1))
  fi
done

echo "checking markdown links"
while IFS= read -r md; do
  while IFS= read -r raw_link; do
    link="${raw_link#*\(}"
    link="${link%\)*}"
    link="${link%%#*}"
    link="${link%%\?*}"
    [[ -z "$link" ]] && continue
    [[ "$link" =~ ^https?:// ]] && continue
    [[ "$link" =~ ^mailto: ]] && continue
    [[ "$link" =~ ^# ]] && continue

    base_dir="$(dirname "$md")"
    target="$link"
    if [[ "$target" == /* ]]; then
      target=".${target}"
    else
      target="${base_dir}/${target}"
    fi

    if [[ ! -e "$target" ]]; then
      echo "broken link in $md -> $link" >&2
      errors=$((errors + 1))
    fi
  done < <(grep -oE '\[[^]]+\]\([^)]+\)' "$md" || true)
done < <(find . -type f -name '*.md' -not -path './.git/*' -not -name 'quality-sweep-*.md' | sort)

echo "checking doc updates for code changes"
base_ref=""
if [[ -n "${GITHUB_BASE_REF:-}" ]] && git rev-parse --verify "origin/${GITHUB_BASE_REF}" >/dev/null 2>&1; then
  base_ref="origin/${GITHUB_BASE_REF}"
elif git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
  base_ref="HEAD~1"
fi

if [[ -n "$base_ref" ]]; then
  changed="$(git diff --name-only "${base_ref}"...HEAD || true)"
  if echo "$changed" | rg -q '^(internal|cmd)/'; then
    if ! echo "$changed" | rg -q '^(AGENTS\.md|docs/ARCHITECTURE\.md|docs/GOLDEN_RULES\.md|docs/RELIABILITY\.md|docs/SECURITY\.md|docs/QUALITY_SCORE\.md|docs/exec-plans/active/)'; then
      echo "code changed but core docs/active plan were not updated" >&2
      errors=$((errors + 1))
    fi
  fi
fi

if (( errors > 0 )); then
  echo "documentation checks failed with $errors error(s)" >&2
  exit 1
fi

echo "documentation checks: OK"
