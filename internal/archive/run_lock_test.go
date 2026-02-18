package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yt-vod-manager/internal/model"
	"yt-vod-manager/internal/runstore"
)

func TestRun_FailsWhenRunDirectoryIsLocked(t *testing.T) {
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}

	ytScript := `#!/usr/bin/env bash
set -euo pipefail
exit 0
`
	ffmpegScript := `#!/usr/bin/env bash
set -euo pipefail
exit 0
`
	if err := os.WriteFile(filepath.Join(fakeBin, "yt-dlp"), []byte(ytScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "ffmpeg"), []byte(ffmpegScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))

	runDir := filepath.Join(tmp, "run")
	if err := runstore.Mkdir(runDir); err != nil {
		t.Fatal(err)
	}
	mf := model.JobsManifest{
		SchemaVersion: 1,
		RunID:         "run-lock",
		Total:         0,
		Jobs:          []model.Job{},
	}
	if err := runstore.WriteJSON(filepath.Join(runDir, "manifest.jobs.json"), mf); err != nil {
		t.Fatal(err)
	}

	lock, err := runstore.AcquireRunLock(runDir)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	defer func() {
		_ = lock.Release()
	}()

	_, err = Run(RunOptions{
		RunDir:   runDir,
		Workers:  1,
		Progress: false,
	})
	if err == nil {
		t.Fatalf("expected locked run to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "locked") {
		t.Fatalf("expected lock error, got %v", err)
	}
}
