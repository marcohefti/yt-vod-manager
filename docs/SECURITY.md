# Security

Last Reviewed: 2026-02-18

## Threat Model (Current Scope)

- Local operator workstation usage.
- External dependency execution (`yt-dlp`, `ffmpeg`).
- Optional authenticated source access via cookies.

## Security Controls

- Cookie paths are resolved to absolute paths and validated before command execution.
- `yt-dlp` invocation uses argument arrays (`exec.Command`) rather than shell interpolation.
- Command output is bounded in memory (`appendLimited`) to avoid unbounded growth.
- Run metadata and manifests are JSON-only local files; no remote storage writes.

## Sensitive Data Handling

- Cookies files are operator-provided and never persisted into manifests.
- Local operator config (`config/projects.json`) is environment-local; keep repository-safe defaults in `config/projects.example.json`.
- Logs may contain source URLs and yt-dlp output; treat run directories as sensitive operational artifacts.
- Do not commit cookies files or private run logs.

## Dependency Surface

- Runtime binaries: `yt-dlp`, `ffmpeg`
- Go dependencies: stdlib only (current state)

## Security Maintenance

- Keep `yt-dlp` and `ffmpeg` patched.
- Keep docs and boundaries current through CI doc checks and scheduled quality sweeps.
- Review logs before sharing externally.
