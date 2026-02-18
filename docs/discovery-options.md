# Discovery Options

Date: 2026-02-17

## Goal

Get a complete, repeatable list of videos for a source URL (playlist first, creator workflows next).

## Option 1: `yt-dlp --flat-playlist -J` (Recommended)

Pros:
- Already handles YouTube continuation/pagination internally.
- Returns stable JSON with IDs and metadata.
- Same toolchain as downloader, so fewer moving parts.

Cons:
- Output schema can evolve across `yt-dlp` releases.

Verdict:
Primary discovery path.

## Option 2: YouTube Data API

Pros:
- Official API.
- Explicit pagination and quotas.

Cons:
- Requires API key/project management.
- Quota costs and operational friction.
- Adds a second integration surface while `yt-dlp` already solves discovery.

Verdict:
Not needed in phase 1.

## Option 3: Browser/DOM scraping (`curl`/headless)

Pros:
- No API key.

Cons:
- YouTube page is virtualized/dynamic; full list needs brittle scroll/continuation logic.
- HTML/JS changes can break parsing frequently.

Verdict:
Fallback only when `yt-dlp` is unavailable/broken.

## Practical Policy

1. Try `yt-dlp` discovery.
2. If discovery fails due auth/geo/rate limits, retry with cookies and backoff.
3. Only consider DOM fallback for emergency extraction, not normal runs.
