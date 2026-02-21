# yt-vod-manager

[![Release](https://img.shields.io/github/v/release/marcohefti/yt-vod-manager?sort=semver)](https://github.com/marcohefti/yt-vod-manager/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/marcohefti/yt-vod-manager/ci.yml?branch=main&label=ci)](https://github.com/marcohefti/yt-vod-manager/actions/workflows/ci.yml)
[![npm downloads](https://img.shields.io/npm/dm/%40marcohefti%2Fyt-vod-manager?logo=npm)](https://www.npmjs.com/package/@marcohefti/yt-vod-manager)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

A CLI app to download YouTube channels and playlists and keep them up to date locally.

It gives you simple daily commands, safe progress tracking, and optional background sync.

## Quick Start

1. Install runtime dependencies (`yt-dlp` and `ffmpeg`):

```bash
# macOS (Homebrew)
brew install yt-dlp ffmpeg
```

```bash
# Ubuntu/Debian example
sudo apt update
sudo apt install -y yt-dlp ffmpeg
```

2. Install `yt-vod-manager` (pick one):

Homebrew:

```bash
brew tap marcohefti/yt-vod-manager
brew install yt-vod-manager
```

WinGet:

```powershell
winget install --id MarcoHefti.YTVodManager
```

npm:

```bash
npm install -g @marcohefti/yt-vod-manager
```

3. Initialize workspace:

```bash
yt-vod-manager init
```

4. Add one or more sources:

```bash
yt-vod-manager add --name mkbhd --source "https://www.youtube.com/@mkbhd/videos"
yt-vod-manager add --name my-playlist --source "https://www.youtube.com/playlist?list=PLFs19LVskfNzQLZkGG_zf6yfYTp_3v_e6"
```

5. Sync all projects:

```bash
yt-vod-manager sync --all-projects
```

6. Check status:

```bash
yt-vod-manager status --all
```

If you see `no projects configured`, start with:

```bash
yt-vod-manager init
yt-vod-manager add --source "<url>"
```

## How It Works

Think in three layers:

1. Projects
- Each project is a named source URL in `config/projects.json`.

2. Runs
- Every sync writes/updates a run under `runs/<run_id>/` with manifests and state.

3. Sync cycle
- `sync` does: refresh source from YouTube every run -> merge by `video_id` -> download pending/retryable jobs.

This means you can stop and continue later without re-downloading completed videos.

## Common Commands

- List configured projects:

```bash
yt-vod-manager list
```

- Open interactive manager (arrow keys + wizard):

```bash
yt-vod-manager manage
```

Manager controls:
- `up/down` (or `j/k`) move selection
- `enter` / `e` edit selected project
- `n` create new project (wizard)
- `d` delete selected project
- `space` toggle `active` on selected project
- move down into the `Actions` panel and press `enter` on `Sync Active Projects` to launch sync
- move down into the `Actions` panel and press `enter` on `Global Settings` to edit global defaults
- `left/right` or `space` toggle select/yes-no fields in the wizard
- `q` quit

The manager auto-adapts layout for narrow and wide terminals.

- Sync one project:

```bash
yt-vod-manager sync --project mkbhd
```

- Sync all projects:

```bash
yt-vod-manager sync --all-projects
```

- Refresh manifests only (no download):

```bash
yt-vod-manager sync --all-projects --no-run
```

- Remove a project:

```bash
yt-vod-manager remove --name my-playlist --yes
```

- Show or update global settings:

```bash
yt-vod-manager settings show
yt-vod-manager settings set --workers 6 --download-limit-mb-s 80 --proxy-mode per_worker
yt-vod-manager settings proxy add --value "http://user:pass@proxy-1:8080"
```

- Update CLI directly from GitHub releases (useful when Winget review is pending):

```bash
yt-vod-manager self-update
```

The CLI also prints a periodic update hint after successful commands when a newer release exists.
Set `YTVM_DISABLE_UPDATE_CHECK=1` to disable this (useful for CI/non-interactive automation).

## Useful Options

- `--workers 5` download multiple videos in parallel (default is `5`).
- On `add`, `--workers 0` means "inherit global/default workers".
- `--fragments 10` stream chunks per video (default is `10`).
- `--download-limit-mb-s 80` cap transfer speed to 80 MB/s for this invocation (`0` disables cap).
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

## Configuration

Default config path: `config/projects.json`

Repository example: `config/projects.example.json` (safe template, no local secrets/paths).

Managed by:
- `init`
- `add`
- `remove`

Each saved source can keep defaults like workers/fragments/order/cookies/subtitle options.
Global settings in the same file (`global`) control:
- default workers
- global download limit in MB/s
- proxy mode and proxy list (one proxy per worker when `proxy_mode=per_worker`)

Runtime precedence:
1. CLI invocation flags
2. Project overrides
3. Global settings
4. Built-in defaults

## Output Layout

- Project config: `config/projects.json`
- Runs: `runs/<run_id>/`
- Run state files:
  - `manifest.raw.json`
  - `manifest.jobs.json`
  - `run.json`
- Downloaded media (default): `runs/<run_id>/downloads/`

## Advanced Commands (Technical)

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

- macOS (`launchd`): create `~/Library/LaunchAgents/com.marcohefti.yt-vod-manager-sync.plist` that runs `/absolute/path/to/yt-vod-manager/scripts/sync-active.sh`, then `launchctl load ~/Library/LaunchAgents/com.marcohefti.yt-vod-manager-sync.plist`.
- Unix (`systemd --user`): create a `marcohefti-yt-vod-manager-sync.service` + `marcohefti-yt-vod-manager-sync.timer` that executes `/absolute/path/to/yt-vod-manager/scripts/sync-active.sh`, then `systemctl --user enable --now marcohefti-yt-vod-manager-sync.timer`.

## Local Build (From Source)

If you want to build locally instead of using npm/Homebrew:

1. Install dependencies (`yt-dlp`, `ffmpeg`, and `go`):

```bash
# macOS (Homebrew)
brew install yt-dlp ffmpeg go
```

```bash
# Ubuntu/Debian example
sudo apt update
sudo apt install -y yt-dlp ffmpeg golang-go
```

2. Build:

```bash
go build -o bin/yt-vod-manager ./cmd/yt-vod-manager
```

3. Run:

```bash
./bin/yt-vod-manager init
```

## Dev Verification

```bash
./scripts/verify.sh
```

This runs tests, architecture checks, linting, and build.

## Releases

GitHub Releases are built automatically via `.github/workflows/release.yml`:

- push to `main` -> versioned prerelease snapshot (`v0.1.0-dev.<run>` initially; after `vX.Y.Z`, next snapshots use `vX.Y.(Z+1)-dev.<run>`)
- push version tag `v*` -> normal versioned release
- each release includes a changelog generated from commits since the previous release tag
- stable tag releases also publish to npm, update Homebrew formula, and generate WinGet manifests (if repo secrets are set, WinGet updates are auto-submitted)

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
- `winget-manifests_<tag>.zip` (stable tags only)

### Package Manager Install

Homebrew:

```bash
brew tap marcohefti/yt-vod-manager
brew install yt-vod-manager
```

WinGet:

```powershell
winget install --id MarcoHefti.YTVodManager
```

npm:

```bash
npm install -g @marcohefti/yt-vod-manager
```

For automation in GitHub Actions:
- set `HOMEBREW_TAP_GITHUB_TOKEN` (repo write access to `marcohefti/homebrew-yt-vod-manager`)
- set `WINGET_CREATE_GITHUB_TOKEN` (GitHub PAT with `public_repo` scope for `wingetcreate` submit)
- configure npm Trusted Publisher for `@marcohefti/yt-vod-manager`:
  - owner: `marcohefti`
  - repository: `yt-vod-manager`
  - workflow file: `.github/workflows/release.yml`
- first WinGet submission is manual in `microsoft/winget-pkgs`; after package `MarcoHefti.YTVodManager` exists, releases auto-submit updates

## License

MIT (`LICENSE`).
