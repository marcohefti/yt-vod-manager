# yt-vod-manager

Project-first YouTube archive manager for channels and playlists.

It is built for operators who want long-running, resumable sync with clear terminal workflows.

## 2-Minute Quick Start

1. Install runtime dependencies:
- `yt-dlp`
- `ffmpeg`

2. Build binary:

```bash
go build -o bin/yt-vod-manager ./cmd/yt-vod-manager
```

3. Initialize workspace:

```bash
./bin/yt-vod-manager init
```

4. Add one or more projects:

```bash
./bin/yt-vod-manager add --name mkbhd --source "https://www.youtube.com/@mkbhd/videos"
./bin/yt-vod-manager add --name my-playlist --source "https://www.youtube.com/playlist?list=PLFs19LVskfNzQLZkGG_zf6yfYTp_3v_e6"
```

5. Sync all projects:

```bash
./bin/yt-vod-manager sync --all-projects
```

6. Check status:

```bash
./bin/yt-vod-manager status --all
```

If you see `no projects configured`, start with:

```bash
./bin/yt-vod-manager init
./bin/yt-vod-manager add --source "<url>"
```

## How It Works

Think in three layers:

1. Projects
- Each project is a named source URL in `config/projects.json`.

2. Runs
- Every sync writes/updates a run under `runs/<run_id>/` with manifests and state.

3. Sync cycle
- `sync` does: refresh source from YouTube every run -> merge by `video_id` -> download pending/retryable jobs.

This means you can stop and resume safely without losing completed work.

## Daily Commands

- List configured projects:

```bash
./bin/yt-vod-manager list
```

- Open interactive manager (arrow keys + wizard):

```bash
./bin/yt-vod-manager manage
```

Manager controls:
- `up/down` (or `j/k`) move selection
- `enter` / `e` edit selected project
- `n` create new project (wizard)
- `d` delete selected project
- `space` toggle `active` on selected project
- move down into the `Actions` panel and press `enter` on `Sync Active Projects` to launch sync
- `left/right` or `space` toggle select/yes-no fields in the wizard
- `q` quit

The manager auto-adapts layout for narrow and wide terminals.

- Sync one project:

```bash
./bin/yt-vod-manager sync --project mkbhd
```

- Sync all projects:

```bash
./bin/yt-vod-manager sync --all-projects
```

- Refresh manifests only (no download):

```bash
./bin/yt-vod-manager sync --all-projects --no-run
```

- Remove a project:

```bash
./bin/yt-vod-manager remove --name my-playlist --yes
```

## Useful Options

- `--workers 5` download multiple videos in parallel (default is `5`).
- `--fragments 10` stream chunks per video (default is `10`).
- `--order oldest` process oldest-first by default.
- `--quality best|1080p|720p` choose a simple quality preset.
- `--subtitles auto|yes|no` choose whether subtitles are downloaded.
- `--sub-langs english|all` choose subtitle language preference (default `english`).
- `--browser-cookies` use logged-in browser cookies for age-restricted videos.
- Browser cookie auth can trigger OS security prompts and account notifications from YouTube/Google/browser.
- `--active-only` sync only projects marked active (with `--project`/`--all-projects`).
- `--max-jobs 10` process only a limited batch.
- `--retry-permanent` re-attempt permanent failures.
- `--stop-on-retryable` stop cleanly after transient/rate-limit failures.
- `--cookies /path/to/cookies.txt` use authenticated access.
- `--json` print machine-readable output.

## Project Config

Default config path: `config/projects.json`

Repository example: `config/projects.example.json` (safe template, no local secrets/paths).

Managed by:
- `init`
- `add`
- `remove`

Project entries can store defaults like workers/fragments/order/cookies/subtitle options.

## Output Layout

- Project config: `config/projects.json`
- Runs: `runs/<run_id>/`
- Run state files:
  - `manifest.raw.json`
  - `manifest.jobs.json`
  - `run.json`
- Downloaded media (default): `runs/<run_id>/downloads/`

## Advanced Commands

Most users only need `init/add/list/sync/status/remove`.

Advanced commands are still available for low-level control:
- `discover`
- `refresh`
- `run`

For `refresh` and `run`, target selection is explicit and safer:
- `--run-id <id>`
- `--run-dir <path>`
- `--project <name>`
- `--latest`

## Reliability Notes

- Download state is checkpointed after each attempt.
- Run-level lock prevents concurrent writers on the same run directory.
- Interrupted `running` jobs are recovered as retryable.
- Subtitle failures are non-fatal.
- Missing previously-downloaded local media is detected and re-queued.
- Manifest writes are atomic (temp-file + rename) to reduce partial-write corruption risk.
- Playlist size shown in live progress is an estimate (metadata/duration based), not an exact byte guarantee.

## Daemon Setup

`sync` is safe for background execution. Use `scripts/sync-active.sh` (includes locking and `--active-only` defaults).

- macOS (`launchd`): create `~/Library/LaunchAgents/ch.hefti.yt-vod-manager-sync.plist` that runs `/absolute/path/to/yt-vod-manager/scripts/sync-active.sh`, then `launchctl load ~/Library/LaunchAgents/ch.hefti.yt-vod-manager-sync.plist`.
- Unix (`systemd --user`): create a `yt-vod-manager-sync.service` + `yt-vod-manager-sync.timer` that executes `/absolute/path/to/yt-vod-manager/scripts/sync-active.sh`, then `systemctl --user enable --now yt-vod-manager-sync.timer`.

## Dev Verification

```bash
./scripts/verify.sh
```

This runs tests, architecture checks, docs checks, linting, and build.

## Releases

GitHub Releases are built automatically when you push a version tag (`v*`), via `.github/workflows/release.yml`.

Create a release tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Artifacts are attached to the release page:
- `darwin` (`amd64`, `arm64`)
- `linux` (`amd64`, `arm64`)
- `windows` (`amd64`)
- `checksums` file for integrity verification

## License

MIT (`LICENSE`).
