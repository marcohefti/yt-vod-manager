# Discovery: Playlist Ingestion

Date: 2026-02-17

Target playlist:
`https://www.youtube.com/playlist?list=PLFs19LVskfNzQLZkGG_zf6yfYTp_3v_e6`

## Question

How do we reliably define and fetch "all" videos for archival?

## Method A (Primary): `yt-dlp` playlist JSON

Command:

```bash
yt-dlp --flat-playlist -J "https://www.youtube.com/playlist?list=PLFs19LVskfNzQLZkGG_zf6yfYTp_3v_e6"
```

Observed summary:

- `playlist_count`: `1658`
- `entries.length`: `1658`
- private placeholders (`title == "[Private video]"`): `792`
- live now entries (`live_status == "is_live"`): `1`

Conclusion:

`yt-dlp` already provides a complete machine-readable index for this playlist and should be our source of truth.

## Method B (Fallback Exploration): Browser DOM via SurfWright

We opened the same playlist via SurfWright and inspected the rendered page.

Observed summary:

- Page text reports: `1,658 videos`
- `document.querySelectorAll('ytd-playlist-video-renderer').length`: `100`

Conclusion:

DOM extraction only sees the currently rendered window/chunk unless we script scroll+continuation loops. It is less stable than `yt-dlp` and should be fallback only.

## Decision

Use `yt-dlp --flat-playlist -J` for discovery, then persist:

1. Raw manifest (`manifest.raw.json`)
2. Normalized archive job list (`manifest.jobs.json`)

## Risks / Edge Cases

- Private/unavailable videos appear as placeholders and cannot be downloaded without access.
- Newly uploaded videos can appear while a run is in progress; discovery should be snapshotted per run.
- Cookie-gated content may require `--cookies`/`--cookies-from-browser` during both discovery and download.

## Next Implementation Step

Build a minimal `discover` command that:

1. Runs `yt-dlp --flat-playlist -J`.
2. Stores raw JSON in a run directory.
3. Emits normalized jobs with deterministic IDs and statuses (`pending`, `skipped_private`, etc.).
