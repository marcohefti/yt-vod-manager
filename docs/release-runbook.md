# Release Runbook: yt-vod-manager

This runbook documents the release system designed to keep stable releases predictable and low-risk.

## Release Model

There are now two separate workflows:

- `release-readiness` (`.github/workflows/release-readiness.yml`)
  - Runs on `main` pushes, pull requests, and manual dispatch.
  - Validates release-critical behavior before any stable publish.
- `release` (`.github/workflows/release.yml`)
  - Manual-only (`workflow_dispatch`).
  - Publishes one explicit stable version.
  - Hard-fails if readiness/CI are not green for the exact commit.

This separation prevents "debug by cutting more stable tags".

## One-Time Repository Setup

Configure branch protection on `main` so these checks are required:

- `verify` (from `ci.yml`)
- `windows-smoke` (from `ci.yml`)
- `packaging-readiness` (from `release-readiness.yml`)
- `winget-readiness` (from `release-readiness.yml`)

Without required checks, the release workflow still validates at runtime, but branch protection is the primary safety barrier.

## Required Secrets

- `HOMEBREW_TAP_GITHUB_TOKEN`
- `WINGET_CREATE_GITHUB_TOKEN`

## What `release-readiness` Verifies

- Cross-platform release artifact build logic.
- npm package publish surface via `npm pack --dry-run`.
- WinGet manifest generation via `packaging/winget/generate-manifests.sh`.
- WinGet manifest validation/install in `windows-latest` runner.
- Release workflow invariants that guard known failure modes:
  - release is manual-only
  - WinGet manifest asset naming uses `RELEASE_TAG`
  - release preflight requires successful `release-readiness`

## Stable Release Procedure

1. Ensure target commit is on `main`.
2. Wait for `ci` and `release-readiness` to pass on that exact commit SHA.
3. Trigger `release` workflow manually from `main` with input:
   - `version`: stable SemVer without leading `v` (example `0.1.9`).
4. Monitor workflow run:
   - `preflight`
   - `publish`
   - `publish-winget`

## `release` Preflight Gates

Before publishing anything, `release` enforces:

- Workflow was started from `main`.
- Version format is valid stable SemVer (`X.Y.Z`).
- `ci` is green for current commit SHA.
- `release-readiness` is green for current commit SHA.
- Version is not already published in:
  - git tags
  - GitHub releases
  - npm
  - Homebrew tap formula

If all checks pass, the workflow creates the GitHub release (and corresponding tag) from the selected commit.

## Publish Behavior

`release` publishes in this order:

- GitHub release + artifacts
- npm package
- Homebrew formula update
- WinGet update PR submission (if package exists, version is new, and no other WinGet PR is already open for this package)

Fast customer channel:

- Users can run `yt-vod-manager self-update` to pull the latest GitHub release directly.
- CLI commands also show a periodic one-line update hint when a newer release exists.
- Operators can disable hint checks in automation with `YTVM_DISABLE_UPDATE_CHECK=1`.
- This is independent of WinGet review latency and is the preferred path for rapid iteration.

WinGet maintainer review and merge in `microsoft/winget-pkgs` is still external and cannot be forced.

## WinGet PR Follow-Up

Find current PR for a specific version:

- `VERSION=0.1.9`
- `PR_URL="$(gh pr list -R microsoft/winget-pkgs --search "MarcoHefti.YTVodManager version ${VERSION}" --state open --json url --jq '.[0].url')"`

Check status:

- `gh pr view "$PR_URL" --repo microsoft/winget-pkgs --json state,mergeStateStatus,reviewDecision,statusCheckRollup`

Check queue state before trying another WinGet submit:

- `gh pr list -R microsoft/winget-pkgs --state open --search "MarcoHefti.YTVodManager" --json number,title,url`

If an open PR already exists for this package, keep one PR active and do not open another.
The `release` workflow now enforces this and skips new WinGet submit when such a PR is detected.

Check checklist lines:

- `gh pr view "$PR_URL" --repo microsoft/winget-pkgs --json body --jq '.body' | grep '^-'`

If checkbox sync ever misses due upstream template wording changes:

- `gh pr view "$PR_URL" --repo microsoft/winget-pkgs --json body --jq '.body' > /tmp/winget-pr-body.md`
- Edit only the three WinGet technical checkbox lines from `- [ ]` to `- [x]`.
- `gh pr edit "$PR_URL" --repo microsoft/winget-pkgs --body-file /tmp/winget-pr-body.md`

## Operational Rules

- Never cut stable release tags manually outside the `release` workflow.
- Never use stable versions to test release workflow fixes.
- If `release` fails preflight, fix readiness/CI first and rerun release dispatch.
- If `publish-winget` fails but `publish` succeeded, treat it as post-release integration work; do not publish a new stable version only to retry WinGet.
