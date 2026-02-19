package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yt-vod-manager/internal/model"
	"yt-vod-manager/internal/runstore"
)

func TestDiscoverPassesJSRuntimeArgsToYTDLP(t *testing.T) {
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
	argsLog := filepath.Join(tmp, "ytdlp-args.log")
	setupYTDLPArgsLogger(t, fakeBin, argsLog, fixturePath, true)
	setupRuntimeBinary(t, fakeBin, "node")

	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("YTDLP_FIXTURE", fixturePath)
	t.Setenv("YTDLP_ARGS_LOG", argsLog)

	if err := Run([]string{
		"discover",
		"--source", "https://example.com/source",
		"--runs-dir", filepath.Join(tmp, "runs"),
		"--js-runtime", "node",
	}); err != nil {
		t.Fatalf("discover failed: %v", err)
	}

	logged := readFileOrFatal(t, argsLog)
	if !strings.Contains(logged, "--no-js-runtimes --js-runtimes node") {
		t.Fatalf("expected js runtime args in yt-dlp invocation, got:\n%s", logged)
	}
}

func TestSyncCLIJSRuntimeOverrideBeatsProjectDefault(t *testing.T) {
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
	argsLog := filepath.Join(tmp, "ytdlp-args.log")
	setupYTDLPArgsLogger(t, fakeBin, argsLog, fixturePath, true)
	setupRuntimeBinary(t, fakeBin, "node")

	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("YTDLP_FIXTURE", fixturePath)
	t.Setenv("YTDLP_ARGS_LOG", argsLog)

	configPath := filepath.Join(tmp, "config", "projects.json")
	runsDir := filepath.Join(tmp, "runs")
	if err := Run([]string{
		"add",
		"--name", "demo",
		"--source", "https://example.com/source",
		"--js-runtime", "deno",
		"--config", configPath,
	}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	if err := Run([]string{
		"sync",
		"--project", "demo",
		"--config", configPath,
		"--runs-dir", runsDir,
		"--no-run",
		"--js-runtime", "node",
	}); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	logged := readFileOrFatal(t, argsLog)
	if !strings.Contains(logged, "--no-js-runtimes --js-runtimes node") {
		t.Fatalf("expected node js runtime args in yt-dlp invocation, got:\n%s", logged)
	}
	if strings.Contains(logged, "--js-runtimes deno") {
		t.Fatalf("expected CLI override to win over project default, got:\n%s", logged)
	}
}

func TestRunPassesJSRuntimeArgsToYTDLP(t *testing.T) {
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	argsLog := filepath.Join(tmp, "ytdlp-args.log")
	setupYTDLPArgsLogger(t, fakeBin, argsLog, "", false)
	setupRuntimeBinary(t, fakeBin, "ffmpeg")
	setupRuntimeBinary(t, fakeBin, "node")

	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("YTDLP_ARGS_LOG", argsLog)

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
				VideoID:  "v1",
				VideoURL: "https://www.youtube.com/watch?v=v1",
				Title:    "Video 1",
				Status:   model.StatusPending,
			},
		},
	}
	if err := runstore.WriteJSON(filepath.Join(runDir, "manifest.jobs.json"), mf); err != nil {
		t.Fatal(err)
	}

	if err := Run([]string{
		"run",
		"--run-dir", runDir,
		"--workers", "1",
		"--progress=false",
		"--subtitles", "no",
		"--js-runtime", "node",
	}); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	logged := readFileOrFatal(t, argsLog)
	if !strings.Contains(logged, "--no-js-runtimes --js-runtimes node") {
		t.Fatalf("expected js runtime args in yt-dlp invocation, got:\n%s", logged)
	}
}

func setupYTDLPArgsLogger(t *testing.T, fakeBin, argsLogPath, fixturePath string, flatOnly bool) {
	t.Helper()
	var body string
	if flatOnly {
		body = `if printf '%s ' "$@" | grep -q -- '--flat-playlist'; then
  cat "$YTDLP_FIXTURE"
  exit 0
fi
echo "unexpected yt-dlp invocation" >&2
exit 1
`
	} else {
		body = "exit 0\n"
	}
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"$YTDLP_ARGS_LOG\"\n" + body
	if err := os.WriteFile(filepath.Join(fakeBin, "yt-dlp"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	if fixturePath != "" {
		t.Setenv("YTDLP_FIXTURE", fixturePath)
	}
}

func setupRuntimeBinary(t *testing.T, fakeBin, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fakeBin, name), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
}

func readFileOrFatal(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
