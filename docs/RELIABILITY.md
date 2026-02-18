# Reliability

## Reliability Objectives

- Resume safely after interruption without losing completed work.
- Keep source snapshots and normalized job state deterministic.
- Recover when local media disappears after previous completion.
- Distinguish retryable failures from permanent failures.
- Keep multi-project operations observable and explicit.

## Current Safeguards

- Manifest checkpointing after each status update (`manifest.jobs.json`).
- Run-level lock file (`.run.lock`) blocks concurrent writers on the same run directory.
- Stale `running` recovery at run start.
- Retryable failure classification for rate-limit/network failures.
- Missing local media detection and automatic re-queue.
- Download archive pruning when re-queueing missing media.
- Source refresh merge by stable `video_id`.
- Project status rollup (`status`) across configured sources.
- Explicit run targeting for advanced commands (`run`/`refresh`) unless `--latest` is chosen.
- State file writes are atomic (write temp + rename) to reduce partial-write corruption.

## Boundaries That Protect Reliability

- Discovery owns source ingestion, project metadata, and status rollups.
- Archive owns execution and status transitions.
- Storage is isolated in `runstore`.
- Raw external command interaction is isolated in `ytdlp`.

## Failure Handling Contract

- Retryable: transient network/rate-limit/service errors.
- Permanent: missing dependencies, malformed/missing URL, hard yt-dlp failures.
- Subtitles are non-fatal and do not fail completed media downloads.

## Operational Checks

- `go test ./...`
- `go run ./scripts/check_arch_boundaries.go`

## Next Reliability Milestones

- Add stale-lock recovery policy (TTL + forced unlock option).
- Add optional checksum verification for downloaded media.
- Add periodic scheduler mode for unattended project sync.
