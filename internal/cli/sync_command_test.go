package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yt-vod-manager/internal/discovery"
)

func TestCollectSyncItemsActiveOnly(t *testing.T) {
	tmp := t.TempDir()
	cfg := tmp + "/projects.json"

	_, err := discovery.AddProject(discovery.AddProjectOptions{
		ConfigPath: cfg,
		Name:       "active-one",
		SourceURL:  "https://example.com/a",
		Active:     boolPtr(true),
	})
	if err != nil {
		t.Fatalf("add active project failed: %v", err)
	}
	_, err = discovery.AddProject(discovery.AddProjectOptions{
		ConfigPath: cfg,
		Name:       "inactive-one",
		SourceURL:  "https://example.com/b",
		Active:     boolPtr(false),
	})
	if err != nil {
		t.Fatalf("add inactive project failed: %v", err)
	}

	items, err := collectSyncItems("", "", "", true, true, cfg)
	if err != nil {
		t.Fatalf("collect sync items failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 active item, got %d", len(items))
	}
	if items[0].Project != "active-one" {
		t.Fatalf("expected active-one, got %q", items[0].Project)
	}
}

func TestCollectSyncItemsActiveOnlyRequiresProjectMode(t *testing.T) {
	_, err := collectSyncItems("https://example.com/src", "", "", false, true, "config/projects.json")
	if err == nil {
		t.Fatal("expected error for active-only in source mode")
	}
}

func TestRunSyncNoRunShowsRefreshStages(t *testing.T) {
	tmp := t.TempDir()
	setupFakeFlatPlaylistYTDLP(t, tmp)
	runsDir := filepath.Join(tmp, "runs")

	output := captureStdout(t, func() {
		err := runSync([]string{
			"--source", "https://example.com/source",
			"--runs-dir", runsDir,
			"--no-run",
		})
		if err != nil {
			t.Fatalf("runSync failed: %v", err)
		}
	})

	if !strings.Contains(output, "sync: refreshing 1 source(s)...") {
		t.Fatalf("expected refresh start line, got:\n%s", output)
	}
	if !strings.Contains(output, "[1/1] refreshing https://example.com/source") {
		t.Fatalf("expected per-source refresh line, got:\n%s", output)
	}
	if !strings.Contains(output, "[1/1] refreshed in") {
		t.Fatalf("expected refresh completion line, got:\n%s", output)
	}
}

func TestRunSyncJSONNoRunRemainsMachineReadable(t *testing.T) {
	tmp := t.TempDir()
	setupFakeFlatPlaylistYTDLP(t, tmp)
	runsDir := filepath.Join(tmp, "runs")

	output := captureStdout(t, func() {
		err := runSync([]string{
			"--source", "https://example.com/source",
			"--runs-dir", runsDir,
			"--no-run",
			"--json",
		})
		if err != nil {
			t.Fatalf("runSync failed: %v", err)
		}
	})

	if strings.Contains(output, "sync: refreshing") {
		t.Fatalf("expected no human status lines in JSON mode, got:\n%s", output)
	}
	if strings.Contains(output, "[1/1] refreshing") {
		t.Fatalf("expected no per-source status lines in JSON mode, got:\n%s", output)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput:\n%s", err, output)
	}
}

func setupFakeFlatPlaylistYTDLP(t *testing.T, tmp string) {
	t.Helper()
	fakeBin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	fixturePath := filepath.Join(tmp, "flat.json")
	fixture := `{"id":"SRC1","title":"Source 1","entries":[{"id":"v1","title":"Video 1","url":"v1"}]}`
	if err := os.WriteFile(fixturePath, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	ytScript := `#!/usr/bin/env bash
set -euo pipefail
if printf '%s ' "$@" | grep -q -- '--flat-playlist'; then
  cat "$YTDLP_FIXTURE"
  exit 0
fi
echo "unexpected yt-dlp invocation" >&2
exit 1
`
	if err := os.WriteFile(filepath.Join(fakeBin, "yt-dlp"), []byte(ytScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("YTDLP_FIXTURE", fixturePath)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()
	defer r.Close()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
