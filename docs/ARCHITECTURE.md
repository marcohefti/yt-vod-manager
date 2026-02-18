# Architecture

Last Reviewed: 2026-02-18

## Purpose

`yt-vod-manager` is a stateful YouTube VOD archive manager. It turns source URLs into long-lived, resumable runs with deterministic job state and now exposes a project-first operator layer.

## Package Dependency Map

- `internal/cli` -> `internal/discovery`, `internal/archive`
- `internal/discovery` -> `internal/model`, `internal/runstore`, `internal/ytdlp`
- `internal/archive` -> `internal/model`, `internal/runstore`, `internal/ytdlp`
- `internal/model` -> stdlib only
- `internal/runstore` -> stdlib only
- `internal/ytdlp` -> stdlib only

Forbidden examples:

- `internal/cli` importing `internal/ytdlp` directly
- `internal/discovery` importing `internal/archive`
- `internal/archive` importing `internal/discovery`
- leaf packages (`model`, `runstore`, `ytdlp`) importing orchestration layers

These boundaries are enforced by:

- `.golangci.yml` (`depguard`)
- `scripts/check_arch_boundaries.go`
- `scripts/check_golden_rules.sh` (engineering invariants)

## Runtime Flow

Project-first flow:

1. `init` / `doctor`
- Create/verify workspace paths (`runs/`, `config/projects.json`).
- Verify runtime dependencies (`yt-dlp`, `ffmpeg`).

2. `add` / `list` / `remove`
- Manage named project definitions in `config/projects.json`.
- Persist optional per-project execution defaults.

3. `sync`
- Resolve targets from project selection, source URL, or fetchlist.
- For each source, upsert run (create or refresh by source URL).
- Execute archive run unless `--no-run`.

4. `status`
- Resolve projects.
- Load each latest run by source URL.
- Produce multi-project health rollup.

Advanced flow remains available:

- `discover` -> manifest snapshot + normalized jobs
- `refresh` -> merge updates into existing run
- `run` -> execute pending/retryable jobs
  - guarded by per-run `.run.lock` to prevent concurrent writers

## State Model

Project registry:
- `config/projects.json` stores project definitions and defaults.

Run state (per run directory):
- `manifest.raw.json`
- `manifest.jobs.json`
- `run.json`

Job statuses:

- `pending`
- `running`
- `completed`
- `failed_retryable`
- `failed_permanent`
- `skipped_private`

Transitions are defined in `internal/model/status.go` and enforced at runtime through `model.TransitionJobStatus`.

## Test Harness

Deterministic harness coverage uses fake `yt-dlp`/`ffmpeg` binaries for:

- discovery/refresh/sync idempotency and merge behavior
- run classification for retryable failures
- CLI project lifecycle (add/sync/status/remove)
- state transition contract tests
