# Architecture

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

0. update hint (best-effort)
- After successful non-JSON commands (excluding `self-update`), check latest GitHub release with cache-backed throttling.
- If newer stable version exists, print one-line upgrade hint to stderr.

1. `self-update`
- Fetch latest (or requested) GitHub release metadata.
- Resolve platform-specific archive asset and release checksum file.
- Verify SHA256 before install and atomically replace/install CLI binary in user-owned path.

2. `init` / `doctor`
- Create/verify workspace paths (`runs/`, `config/projects.json`).
- Verify runtime dependencies (`yt-dlp`, `ffmpeg`).

3. `add` / `list` / `remove`
- Manage named project definitions in `config/projects.json`.
- Persist optional per-project execution overrides, including yt-dlp JavaScript runtime selection (`js_runtime`).

4. `settings`
- Manage global runtime defaults in `config/projects.json` (`global` block).
- Configure workers, per-worker proxy pool, and global MB/s download cap.

5. `sync`
- Resolve targets from project selection, source URL, or fetchlist.
- For each source, upsert run (create or refresh by source URL).
- Execute archive run unless `--no-run`.

6. `status`
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
- `config/projects.json` stores:
  - `global` runtime settings (workers, proxy mode/list, download limit)
  - per-project settings (including optional worker override where `0` means inherit, and `js_runtime` where `auto` follows yt-dlp default behavior)

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

Runtime precedence:

1. CLI one-off overrides
2. Project overrides
3. Global settings
4. Built-in defaults

For yt-dlp JavaScript runtime selection specifically, precedence is:

1. CLI `--js-runtime`
2. Project `js_runtime` (single runtime or ordered fallback list, for example `node,quickjs`)
3. `auto` (delegates to yt-dlp default behavior)

## Test Harness

Deterministic harness coverage uses fake `yt-dlp`/`ffmpeg` binaries for:

- discovery/refresh/sync idempotency and merge behavior
- run classification for retryable failures
- CLI project lifecycle (add/sync/status/remove)
- state transition contract tests
