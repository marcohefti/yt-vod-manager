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
- Quick status helper for the active run:
  - `RUN=<run_id>`
  - `gh run view "$RUN" --json status,conclusion,jobs`

Green path:
- `publish` success
- `publish-winget` success

Known failure pattern encountered and fixed:
- `wingetcreate` did not accept `--no-open` in workflow runtime.
- Fix was to remove `--no-open` from `.github/workflows/release.yml` submit step.

### Current Winget PR reality check

For this release flow, the PR is usually created automatically in `microsoft/winget-pkgs` and then enters normal upstream review.
- It can show `REVIEW_REQUIRED` while still being technically correct.
- It can stay `OPEN` with merge blocked until maintainer review is complete.
- The three template checkboxes in that PR are not required to be completed for us by default in our repo pipeline.

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

Use this command set to close the loop from this repo side:
- Find the PR:
  - `PR_URL="$(gh pr list -R microsoft/winget-pkgs --search \"MarcoHefti.YTVodManager version vX.Y.Z\" --state open --json url --jq '.[0].url')"`
- Confirm status:
  - `gh pr view "$PR_URL" --repo microsoft/winget-pkgs --json state,mergeStateStatus,reviewDecision,checks,statusCheckRollup`
- Add maintainer-facing context:
  - Post a short note that the release, npm, and Homebrew are published and include pipeline links.

What’s left for full autonomy:
- Add automated manifest checks in `.github/workflows/release.yml` before submit in `publish-winget`:
  - run `winget validate --manifest <path-to-generated-manifests>` 
  - run `winget install --manifest <path-to-generated-manifests>`
- Post/update PR status in `microsoft/winget-pkgs` as evidence.
- Note: only Microsoft maintainer review can complete merge, so upstream approval remains a manual dependency even after automated checks.

## 6) Suggested response when PR is waiting

- Post a short note:
  - “Release completed (`vX.Y.Z`), release artifacts are published on GitHub/npm/Homebrew, WinGet PR is this one: `<PR URL>`. Validation/checks posted above.”
