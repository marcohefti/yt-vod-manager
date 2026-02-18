# Golden Rules

These rules are intentionally strict and machine-checked.

## Rule 1: No direct process execution outside `internal/ytdlp`

- External command execution must be centralized in `internal/ytdlp`.
- Enforcement: `scripts/check_golden_rules.sh`

## Rule 2: No raw job status literals outside `internal/model/status.go`

- Status values must use constants in `internal/model/status.go`.
- Enforcement: `scripts/check_golden_rules.sh`

## Rule 3: Keep files small and composable

- Non-test Go files under `internal/` must not exceed 900 lines.
- Enforcement: `scripts/check_golden_rules.sh`

## Rule 4: Respect package boundaries

- Follow dependency map in `docs/ARCHITECTURE.md`.
- Enforcement:
  - `scripts/check_arch_boundaries.go`
  - `.golangci.yml` (`depguard`)
