package runstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type RunMeta struct {
	RunID            string `json:"run_id"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at,omitempty"`
	Profile          string `json:"profile"`
	SourceURL        string `json:"source_url"`
	SourceID         string `json:"source_id,omitempty"`
	SourceTitle      string `json:"source_title,omitempty"`
	SourceType       string `json:"source_type,omitempty"`
	RawManifestPath  string `json:"raw_manifest_path"`
	JobsManifestPath string `json:"jobs_manifest_path"`
	OutputDir        string `json:"output_dir,omitempty"`
	TotalEntries     int    `json:"total_entries"`
	Pending          int    `json:"pending"`
	SkippedPrivate   int    `json:"skipped_private"`
}

func Mkdir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}

func WriteBytes(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent for %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(dir, ".ytvm-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("atomic rename for %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}

func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON for %s: %w", path, err)
	}
	data = append(data, '\n')
	return WriteBytes(path, data)
}

func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse JSON %s: %w", path, err)
	}
	return nil
}

func LatestRunDir(runsDir string) (string, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return "", fmt.Errorf("read runs directory %s: %w", runsDir, err)
	}

	dirs := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no run directories found in %s", runsDir)
	}

	sort.Strings(dirs)
	return filepath.Join(runsDir, dirs[len(dirs)-1]), nil
}

func ListRunDirs(runsDir string) ([]string, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read runs directory %s: %w", runsDir, err)
	}

	dirs := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(runsDir, e.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

func RunMetaPath(runDir string) string {
	return filepath.Join(runDir, "run.json")
}

func LoadRunMeta(runDir string) (RunMeta, error) {
	var meta RunMeta
	if err := ReadJSON(RunMetaPath(runDir), &meta); err != nil {
		return RunMeta{}, err
	}
	return meta, nil
}

func SaveRunMeta(runDir string, meta RunMeta) error {
	return WriteJSON(RunMetaPath(runDir), meta)
}
