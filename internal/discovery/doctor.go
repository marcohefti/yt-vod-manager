package discovery

import (
	"os"
	"path/filepath"
	"strings"

	"yt-vod-manager/internal/runstore"
	"yt-vod-manager/internal/ytdlp"
)

type DoctorOptions struct {
	RunsDir    string
	ConfigPath string
}

type DoctorResult struct {
	OK     bool          `json:"ok"`
	Checks []DoctorCheck `json:"checks"`
}

type DoctorCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type InitWorkspaceOptions struct {
	RunsDir    string
	ConfigPath string
}

type InitWorkspaceResult struct {
	RunsDir        string       `json:"runs_dir"`
	ConfigPath     string       `json:"config_path"`
	CreatedRunsDir bool         `json:"created_runs_dir"`
	CreatedConfig  bool         `json:"created_config"`
	DoctorResult   DoctorResult `json:"doctor"`
}

func Doctor(opts DoctorOptions) (DoctorResult, error) {
	runsDir := strings.TrimSpace(opts.RunsDir)
	if runsDir == "" {
		runsDir = "runs"
	}
	configPath := normalizeConfigPath(opts.ConfigPath)

	checks := make([]DoctorCheck, 0, 4)
	dep := ytdlp.DependencyStatus()
	checks = append(checks, DoctorCheck{
		Name:    "dependency:yt-dlp",
		OK:      dep.YTDLPFound,
		Message: dependencyMessage(dep.YTDLPFound, dep.YTDLPPath, "yt-dlp"),
	})
	checks = append(checks, DoctorCheck{
		Name:    "dependency:ffmpeg",
		OK:      dep.FFmpegFound,
		Message: dependencyMessage(dep.FFmpegFound, dep.FFmpegPath, "ffmpeg"),
	})

	runsDirOK, runsDirMessage := ensureWritableDir(runsDir)
	checks = append(checks, DoctorCheck{
		Name:    "directory:runs",
		OK:      runsDirOK,
		Message: runsDirMessage,
	})

	cfgDir := filepath.Dir(configPath)
	cfgOK, cfgMessage := ensureWritableDir(cfgDir)
	checks = append(checks, DoctorCheck{
		Name:    "directory:config",
		OK:      cfgOK,
		Message: cfgMessage,
	})

	ok := true
	for _, c := range checks {
		if !c.OK {
			ok = false
			break
		}
	}

	return DoctorResult{OK: ok, Checks: checks}, nil
}

func InitWorkspace(opts InitWorkspaceOptions) (InitWorkspaceResult, error) {
	runsDir := strings.TrimSpace(opts.RunsDir)
	if runsDir == "" {
		runsDir = "runs"
	}
	configPath := normalizeConfigPath(opts.ConfigPath)

	createdRunsDir := false
	if _, err := os.Stat(runsDir); os.IsNotExist(err) {
		createdRunsDir = true
	}
	if err := runstore.Mkdir(runsDir); err != nil {
		return InitWorkspaceResult{}, err
	}

	_, createdConfig, err := EnsureProjectRegistry(configPath)
	if err != nil {
		return InitWorkspaceResult{}, err
	}

	doc, err := Doctor(DoctorOptions{RunsDir: runsDir, ConfigPath: configPath})
	if err != nil {
		return InitWorkspaceResult{}, err
	}

	return InitWorkspaceResult{
		RunsDir:        runsDir,
		ConfigPath:     configPath,
		CreatedRunsDir: createdRunsDir,
		CreatedConfig:  createdConfig,
		DoctorResult:   doc,
	}, nil
}

func dependencyMessage(ok bool, path, name string) string {
	if ok {
		return name + " found at " + path
	}
	return name + " not found on PATH"
}

func ensureWritableDir(path string) (bool, string) {
	if strings.TrimSpace(path) == "" {
		return false, "empty path"
	}
	if err := runstore.Mkdir(path); err != nil {
		return false, err.Error()
	}
	f, err := os.CreateTemp(path, "yt-vod-manager-check-*.tmp")
	if err != nil {
		return false, err.Error()
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return true, "writable"
}
