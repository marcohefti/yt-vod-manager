# WinGet Packaging

This directory contains tooling for `yt-vod-manager` WinGet manifests.

## Package Identifier

- `MarcoHefti.YTVodManager`

## Generate Manifests

Use `generate-manifests.sh` with a stable release tag, the Windows zip asset URL, and SHA256:

```bash
./packaging/winget/generate-manifests.sh \
  --version 0.1.0 \
  --release-tag v0.1.0 \
  --installer-url "https://github.com/marcohefti/yt-vod-manager/releases/download/v0.1.0/yt-vod-manager_v0.1.0_windows_amd64.zip" \
  --installer-sha "<sha256>" \
  --release-date "2026-02-18" \
  --out-root "dist/winget"
```

This writes manifests under:

- `dist/winget/manifests/m/MarcoHefti/YTVodManager/<version>/`

## Release Automation

- `.github/workflows/release.yml` generates a `winget-manifests_<tag>.zip` release asset for stable tags.
- If `WINGET_CREATE_GITHUB_TOKEN` is configured and package `MarcoHefti.YTVodManager` already exists in `microsoft/winget-pkgs`, the workflow auto-submits updates using `wingetcreate update`.
- First submission still needs to be done manually in `microsoft/winget-pkgs`.
