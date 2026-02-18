# yt-vod-manager AGENTS

This file defines the working contract for changes in this repository.

## Scope

These rules apply to the entire repo unless a deeper `AGENTS.md` overrides them.

## Core Principles

- Keep behavior clear and predictable over cleverness.
- Prefer small composable changes that preserve restart safety.
- Keep user-facing workflows fast and obvious.
- If code behavior changes, update docs in the same change.

## Hard Rules

- Respect architecture boundaries from `docs/ARCHITECTURE.md`.
- Do not execute external processes outside `internal/ytdlp`.
- Do not use raw job status string literals outside `internal/model/status.go`.
- Keep non-test Go files in `internal/` at or below 900 lines.
- Do not commit runtime/operator state:
  - `runs/`
  - `downloads/`
  - local `config/projects.json`
  - generated manifests/logs/temp artifacts

## Documentation Contract

- Core docs are required and must stay accurate:
  - `docs/ARCHITECTURE.md`
  - `docs/GOLDEN_RULES.md`
  - `docs/RELIABILITY.md`
  - `docs/SECURITY.md`
- If code in `internal/` or `cmd/` changes, update relevant core docs in the same change.

## Required Validation Before Push

Run:

1. `go test ./...`
2. `go run ./scripts/check_arch_boundaries.go`
3. `./scripts/check_golden_rules.sh`
4. `./scripts/verify.sh`

## CI and Releases

- CI workflow: `.github/workflows/ci.yml`
- Release workflow: `.github/workflows/release.yml` (triggered by `v*` tags)

## Reference Index

- [Architecture](docs/ARCHITECTURE.md)
- [Golden Rules](docs/GOLDEN_RULES.md)
- [Reliability](docs/RELIABILITY.md)
- [Security](docs/SECURITY.md)
