# Release Runbook: yt-vod-manager

Use this whenever doing releases. It documents the current stable path (`v*` tag -> release workflow -> npm/Homebrew/WinGet).

## 1) Before release

- Confirm local repo is clean and on the intended commit:
  - `git status`
  - `git fetch --all --tags --prune`
  - `git log --oneline -n 5`
- Confirm required secrets are available in GitHub settings:
  - `HOMEBREW_TAP_GITHUB_TOKEN`
  - `WINGET_CREATE_GITHUB_TOKEN`
- Confirm release artifacts already build locally if needed (optional smoke):
  - `go test ./...`
  - `go run ./scripts/check_arch_boundaries.go`
  - `./scripts/check_golden_rules.sh`
  - `./scripts/verify.sh`
- Confirm last release tag and next semver tag.

## 2) Trigger release

- Create and push the stable tag:
  - `git tag vX.Y.Z`
  - `git push origin vX.Y.Z`
- The push to tag `v*` triggers `.github/workflows/release.yml`.
- Expected jobs:
  - `publish` on `ubuntu-latest`
  - `publish-winget` on `windows-latest` (depends on `publish`, only for stable tags)

## 3) Watch CI run and map outcomes

- Run list:
  - `gh run list --workflow release --json databaseId,status,conclusion,headBranch,updatedAt`
- Run details:
  - `gh run view <run_id> --json jobs`

Green path:
- `publish` success
- `publish-winget` success

Known failure pattern encountered and fixed:
- `wingetcreate` did not accept `--no-open` in workflow runtime.
- Fix was to remove `--no-open` from `.github/workflows/release.yml` submit step.

## 4) Post-release verification

- GitHub release:
  - `gh release view vX.Y.Z --json tagName,assets`
- npm:
  - `npm view @marcohefti/yt-vod-manager@<semver> version`
- Homebrew tap:
  - check `.github` action output for tap push success OR inspect formula:
  - `curl -fsSL https://raw.githubusercontent.com/marcohefti/homebrew-yt-vod-manager/main/Formula/yt-vod-manager.rb`
- WinGet:
  - open PR created by bot in `microsoft/winget-pkgs`
  - capture PR URL for release notes/changelog

## 5) WinGet PR follow-up (manual part)

The Winget submission is now auto-created, but human follow-through is still needed to get it merged.

What to verify before waiting:
- confirm the PR only touches one package path under `manifests/m/MarcoHefti/YTVodManager/0.1.5`
- confirm no other open PR exists for the same package manifest path
- confirm CLA check is passing in the PR status checks
- collect and post links for any pipeline checks (validation/publish) if present

For local validation, run on Windows when possible:
- `winget validate --manifest <path>`
- `winget install --manifest <path>`
- verify schema target is at least `1.10`

If all checks are green, post a short completion note in the PR and monitor for maintainer merge.

## 6) Suggested response when PR is waiting

- Post a short note:
  - “Release completed (`vX.Y.Z`), release artifacts are published on GitHub/npm/Homebrew, WinGet PR is this one: `<PR URL>`. Validation/checks posted above.”
