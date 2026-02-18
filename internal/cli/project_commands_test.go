package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yt-vod-manager/internal/discovery"
	"yt-vod-manager/internal/runstore"
)

func TestHarnessProjectLifecycle(t *testing.T) {
	tmp := t.TempDir()
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

	configPath := filepath.Join(tmp, "config", "projects.json")
	runsDir := filepath.Join(tmp, "runs")

	if err := Run([]string{
		"add",
		"--name", "demo",
		"--source", "https://example.com/source",
		"--config", configPath,
	}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	reg, err := discovery.LoadProjects(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(reg.Projects))
	}
	if reg.Projects[0].Name != "demo" {
		t.Fatalf("expected project name demo, got %q", reg.Projects[0].Name)
	}

	if err := Run([]string{
		"sync",
		"--project", "demo",
		"--config", configPath,
		"--runs-dir", runsDir,
		"--no-run",
	}); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	dirs, err := runstore.ListRunDirs(runsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 1 {
		t.Fatalf("expected one run dir, got %d", len(dirs))
	}

	if err := Run([]string{
		"status",
		"--project", "demo",
		"--config", configPath,
		"--runs-dir", runsDir,
	}); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	if err := Run([]string{
		"remove",
		"--name", "demo",
		"--config", configPath,
		"--yes",
	}); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	reg, err = discovery.LoadProjects(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Projects) != 0 {
		t.Fatalf("expected no projects after remove, got %d", len(reg.Projects))
	}
}

func TestRunRequiresExplicitTarget(t *testing.T) {
	err := Run([]string{"run"})
	if err == nil {
		t.Fatal("expected run to require an explicit target")
	}
	if !strings.Contains(err.Error(), "run target required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRefreshRequiresExplicitTarget(t *testing.T) {
	err := Run([]string{"refresh"})
	if err == nil {
		t.Fatal("expected refresh to require an explicit target")
	}
	if !strings.Contains(err.Error(), "refresh target required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
