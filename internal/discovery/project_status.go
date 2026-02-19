package discovery

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"yt-vod-manager/internal/model"
	"yt-vod-manager/internal/runstore"
)

type ProjectStatusOptions struct {
	ConfigPath string
	Project    string
	All        bool
	RunsDir    string
}

type ProjectStatusResult struct {
	ConfigPath string              `json:"config_path"`
	Rows       []ProjectStatusItem `json:"projects"`
	Totals     ProjectStatusTotals `json:"totals"`
}

type ProjectStatusItem struct {
	Project         string `json:"project"`
	SourceURL       string `json:"source_url"`
	JSRuntime       string `json:"js_runtime,omitempty"`
	RunID           string `json:"run_id,omitempty"`
	RunDir          string `json:"run_dir,omitempty"`
	SourceTitle     string `json:"source_title,omitempty"`
	State           string `json:"state"`
	UpdatedAt       string `json:"updated_at,omitempty"`
	Total           int    `json:"total_count"`
	Completed       int    `json:"completed_count"`
	Pending         int    `json:"pending_count"`
	Running         int    `json:"running_count"`
	FailedRetryable int    `json:"retryable_failure_count"`
	FailedPermanent int    `json:"permanent_failure_count"`
	SkippedPrivate  int    `json:"skipped_private_count"`
	Remaining       int    `json:"remaining"`
}

type ProjectStatusTotals struct {
	Projects        int `json:"projects"`
	Healthy         int `json:"healthy"`
	Attention       int `json:"attention"`
	NeverSynced     int `json:"never_synced"`
	TotalVideos     int `json:"total_videos"`
	Completed       int `json:"completed_count"`
	Pending         int `json:"pending_count"`
	Running         int `json:"running_count"`
	FailedRetryable int `json:"retryable_failure_count"`
	FailedPermanent int `json:"permanent_failure_count"`
}

func ProjectStatus(opts ProjectStatusOptions) (ProjectStatusResult, error) {
	configPath := normalizeConfigPath(opts.ConfigPath)
	runsDir := strings.TrimSpace(opts.RunsDir)
	if runsDir == "" {
		runsDir = "runs"
	}

	projects, err := ResolveProjectSelection(configPath, opts.Project, opts.All)
	if err != nil {
		return ProjectStatusResult{}, err
	}

	rows := make([]ProjectStatusItem, 0, len(projects))
	totals := ProjectStatusTotals{}

	for _, p := range projects {
		row, err := buildProjectStatusRow(runsDir, p)
		if err != nil {
			return ProjectStatusResult{}, err
		}
		rows = append(rows, row)
		totals.Projects++
		totals.TotalVideos += row.Total
		totals.Completed += row.Completed
		totals.Pending += row.Pending
		totals.Running += row.Running
		totals.FailedRetryable += row.FailedRetryable
		totals.FailedPermanent += row.FailedPermanent
		switch row.State {
		case "healthy":
			totals.Healthy++
		case "never_synced":
			totals.NeverSynced++
		default:
			totals.Attention++
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Project < rows[j].Project
	})

	return ProjectStatusResult{
		ConfigPath: configPath,
		Rows:       rows,
		Totals:     totals,
	}, nil
}

func ResolveRunDirForProject(configPath, projectName, runsDir string) (string, Project, error) {
	project, err := FindProjectByName(configPath, projectName)
	if err != nil {
		return "", Project{}, err
	}
	baseRuns := strings.TrimSpace(runsDir)
	if baseRuns == "" {
		baseRuns = "runs"
	}
	runDir, err := latestRunDirBySource(baseRuns, project.SourceURL)
	if err != nil {
		return "", Project{}, err
	}
	if runDir == "" {
		return "", Project{}, fmt.Errorf("project %q has no run yet; use sync first", project.Name)
	}
	return runDir, project, nil
}

func buildProjectStatusRow(runsDir string, project Project) (ProjectStatusItem, error) {
	row := ProjectStatusItem{
		Project:   project.Name,
		SourceURL: project.SourceURL,
		JSRuntime: projectJSRuntime(project),
		State:     "never_synced",
	}

	runDir, err := latestRunDirBySource(runsDir, project.SourceURL)
	if err != nil {
		return ProjectStatusItem{}, err
	}
	if runDir == "" {
		return row, nil
	}

	row.RunDir = runDir
	row.RunID = filepath.Base(runDir)

	meta, metaErr := runstore.LoadRunMeta(runDir)
	if metaErr == nil {
		if strings.TrimSpace(meta.RunID) != "" {
			row.RunID = strings.TrimSpace(meta.RunID)
		}
		row.UpdatedAt = strings.TrimSpace(meta.UpdatedAt)
		row.SourceTitle = strings.TrimSpace(meta.SourceTitle)
	}

	var mf model.JobsManifest
	if err := runstore.ReadJSON(filepath.Join(runDir, "manifest.jobs.json"), &mf); err != nil {
		if metaErr != nil {
			return ProjectStatusItem{}, err
		}
		row.Total = meta.TotalEntries
		row.Pending = meta.Pending
		row.SkippedPrivate = meta.SkippedPrivate
		row.Remaining = row.Pending
		row.State = summarizeState(row)
		return row, nil
	}

	row.Total = mf.Total
	row.Completed = mf.Completed
	row.Pending = mf.Pending
	row.Running = mf.Running
	row.FailedRetryable = mf.FailedRetryable
	row.FailedPermanent = mf.FailedPermanent
	row.SkippedPrivate = mf.SkippedPrivate
	row.Remaining = mf.Pending + mf.Running + mf.FailedRetryable
	if strings.TrimSpace(row.SourceTitle) == "" {
		row.SourceTitle = strings.TrimSpace(mf.SourceTitle)
	}
	if row.UpdatedAt == "" {
		row.UpdatedAt = strings.TrimSpace(mf.GeneratedAt)
	}
	row.State = summarizeState(row)
	return row, nil
}

func projectJSRuntime(project Project) string {
	v, ok := parseJSRuntime(project.JSRuntime)
	if !ok {
		return DefaultJSRuntime
	}
	return v
}

func summarizeState(row ProjectStatusItem) string {
	if row.RunID == "" {
		return "never_synced"
	}
	if row.Running > 0 {
		return "in_progress"
	}
	if row.FailedPermanent > 0 {
		return "permanent_failures"
	}
	if row.FailedRetryable > 0 {
		return "needs_retry"
	}
	if row.Pending > 0 {
		return "queued"
	}
	return "healthy"
}
