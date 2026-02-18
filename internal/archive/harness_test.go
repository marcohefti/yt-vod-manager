package archive

import (
	"os"
	"path/filepath"
	"testing"

	"yt-vod-manager/internal/model"
	"yt-vod-manager/internal/runstore"
)

func TestHarnessRunClassifiesRetryableFailures(t *testing.T) {
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}

	ytScript := `#!/usr/bin/env bash
set -euo pipefail
echo "HTTP Error 429: Too Many Requests" >&2
exit 1
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
		RunID:         "run1",
		SourceURL:     "https://example.com/source",
		Total:         1,
		Pending:       1,
		Jobs: []model.Job{
			{
				JobID:    "j1",
				Index:    1,
				VideoID:  "retry123",
				VideoURL: "https://www.youtube.com/watch?v=retry123",
				Title:    "Retry Video",
				Status:   model.StatusPending,
			},
		},
	}
	if err := runstore.WriteJSON(filepath.Join(runDir, "manifest.jobs.json"), mf); err != nil {
		t.Fatal(err)
	}

	res, err := Run(RunOptions{
		RunDir:          runDir,
		Latest:          false,
		Progress:        false,
		Workers:         1,
		StopOnRetryable: true,
	})
	if err != nil {
		t.Fatalf("run failed unexpectedly: %v", err)
	}
	if res.FailedRetryable != 1 {
		t.Fatalf("expected failed_retryable=1, got %d", res.FailedRetryable)
	}

	var out model.JobsManifest
	if err := runstore.ReadJSON(filepath.Join(runDir, "manifest.jobs.json"), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(out.Jobs))
	}
	if out.Jobs[0].Status != model.StatusFailedRetryable {
		t.Fatalf("expected retryable status, got %s", out.Jobs[0].Status)
	}
	if out.Jobs[0].Reason != "transient_or_rate_limited" {
		t.Fatalf("unexpected retryable reason: %q", out.Jobs[0].Reason)
	}
}
