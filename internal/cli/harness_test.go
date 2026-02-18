package cli

import (
	"os"
	"path/filepath"
	"testing"

	"yt-vod-manager/internal/model"
	"yt-vod-manager/internal/runstore"
)

func TestHarnessSyncNoRunIdempotent(t *testing.T) {
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}

	fixturePath := filepath.Join(tmp, "flat.json")
	fixture := `{"id":"SRC1","title":"Source 1","entries":[{"id":"v1","title":"Video 1","url":"v1"},{"id":"v2","title":"Video 2","url":"v2"}]}`
	if err := os.WriteFile(fixturePath, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	ytScript := `#!/usr/bin/env bash
set -euo pipefail
if printf '%s ' "$@" | grep -q -- '--flat-playlist'; then
  cat "$YTDLP_FIXTURE"
  exit 0
fi
echo "unexpected download invocation in no-run harness" >&2
exit 1
`
	if err := os.WriteFile(filepath.Join(fakeBin, "yt-dlp"), []byte(ytScript), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("YTDLP_FIXTURE", fixturePath)

	fetchlist := filepath.Join(tmp, "fetchlist.txt")
	if err := os.WriteFile(fetchlist, []byte("Alias|https://example.com/source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runsDir := filepath.Join(tmp, "runs")

	if err := Run([]string{"sync", "--fetchlist", fetchlist, "--runs-dir", runsDir, "--no-run"}); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if err := Run([]string{"sync", "--fetchlist", fetchlist, "--runs-dir", runsDir, "--no-run"}); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	dirs, err := runstore.ListRunDirs(runsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 1 {
		t.Fatalf("expected one run dir after idempotent sync, got %d", len(dirs))
	}

	var mf model.JobsManifest
	if err := runstore.ReadJSON(filepath.Join(dirs[0], "manifest.jobs.json"), &mf); err != nil {
		t.Fatal(err)
	}
	if mf.Total != 2 || mf.Pending != 2 {
		t.Fatalf("unexpected manifest totals: total=%d pending=%d", mf.Total, mf.Pending)
	}
}
