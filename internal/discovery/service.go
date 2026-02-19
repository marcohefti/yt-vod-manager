package discovery

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"yt-vod-manager/internal/model"
	"yt-vod-manager/internal/runstore"
	"yt-vod-manager/internal/ytdlp"
)

type Options struct {
	SourceURL          string
	Profile            string
	RunsDir            string
	CookiesPath        string
	CookiesFromBrowser string
	JSRuntime          string
}

type Result struct {
	RunID            string
	RunDir           string
	RawManifestPath  string
	JobsManifestPath string
	TotalEntries     int
	Pending          int
	SkippedPrivate   int
}

type RefreshOptions struct {
	RunID              string
	RunDir             string
	RunsDir            string
	Latest             bool
	SourceURL          string
	CookiesPath        string
	CookiesFromBrowser string
	JSRuntime          string
}

type RefreshResult struct {
	RunID            string
	RunDir           string
	RawManifestPath  string
	JobsManifestPath string
	TotalEntries     int
	Pending          int
	SkippedPrivate   int
	Added            int
}

type UpsertOptions struct {
	SourceURL          string
	Profile            string
	RunsDir            string
	CookiesPath        string
	CookiesFromBrowser string
	JSRuntime          string
}

type UpsertResult struct {
	Created bool
	Result  Result
	Refresh RefreshResult
}

type ytDLPCollection struct {
	ID            string       `json:"id"`
	Title         string       `json:"title"`
	PlaylistCount int          `json:"playlist_count"`
	WebpageURL    string       `json:"webpage_url"`
	Entries       []ytDLPEntry `json:"entries"`
}

type ytDLPEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type sourceManifest struct {
	ID      string
	Title   string
	Type    string
	Raw     []byte
	Entries []sourceEntry
}

type sourceEntry struct {
	ID       string
	Title    string
	VideoURL string
	Private  bool
}

func Run(opts Options) (Result, error) {
	now := time.Now().UTC()
	profile := strings.TrimSpace(opts.Profile)
	if profile == "" {
		profile = "default"
	}
	runsDir := strings.TrimSpace(opts.RunsDir)
	if runsDir == "" {
		runsDir = "runs"
	}

	src, err := fetchSourceManifest(opts.SourceURL, opts.CookiesPath, opts.CookiesFromBrowser, opts.JSRuntime)
	if err != nil {
		return Result{}, err
	}

	runID := fmt.Sprintf("%s_%s", now.Format("20060102T150405Z"), sanitizeID(src.ID))
	runDir := filepath.Join(runsDir, runID)
	if err := runstore.Mkdir(runDir); err != nil {
		return Result{}, err
	}

	rawPath := filepath.Join(runDir, "manifest.raw.json")
	if err := runstore.WriteBytes(rawPath, src.Raw); err != nil {
		return Result{}, err
	}

	jobs, _ := mergeJobs(nil, src)
	pending, skippedPrivate := countJobs(jobs)

	mf := model.JobsManifest{
		SchemaVersion:   1,
		GeneratedAt:     now.Format(time.RFC3339),
		RunID:           runID,
		Profile:         profile,
		SourceURL:       strings.TrimSpace(opts.SourceURL),
		SourceID:        src.ID,
		SourceTitle:     src.Title,
		SourceType:      src.Type,
		PlaylistID:      src.ID,
		PlaylistTitle:   src.Title,
		Total:           len(jobs),
		Pending:         pending,
		Running:         0,
		Completed:       0,
		FailedRetryable: 0,
		FailedPermanent: 0,
		SkippedPrivate:  skippedPrivate,
		Jobs:            jobs,
	}

	jobsPath := filepath.Join(runDir, "manifest.jobs.json")
	if err := runstore.WriteJSON(jobsPath, mf); err != nil {
		return Result{}, err
	}

	meta := runstore.RunMeta{
		RunID:            runID,
		CreatedAt:        now.Format(time.RFC3339),
		UpdatedAt:        now.Format(time.RFC3339),
		Profile:          profile,
		SourceURL:        strings.TrimSpace(opts.SourceURL),
		SourceID:         src.ID,
		SourceTitle:      src.Title,
		SourceType:       src.Type,
		RawManifestPath:  rawPath,
		JobsManifestPath: jobsPath,
		TotalEntries:     len(jobs),
		Pending:          pending,
		SkippedPrivate:   skippedPrivate,
	}
	if err := runstore.SaveRunMeta(runDir, meta); err != nil {
		return Result{}, err
	}

	return Result{
		RunID:            runID,
		RunDir:           runDir,
		RawManifestPath:  rawPath,
		JobsManifestPath: jobsPath,
		TotalEntries:     len(jobs),
		Pending:          pending,
		SkippedPrivate:   skippedPrivate,
	}, nil
}

func Refresh(opts RefreshOptions) (RefreshResult, error) {
	runDir, err := resolveRunDir(opts.RunDir, opts.RunID, opts.RunsDir, opts.Latest)
	if err != nil {
		return RefreshResult{}, err
	}
	runLock, err := runstore.AcquireRunLock(runDir)
	if err != nil {
		return RefreshResult{}, err
	}
	defer func() {
		_ = runLock.Release()
	}()

	jobsPath := filepath.Join(runDir, "manifest.jobs.json")
	var mf model.JobsManifest
	if err := runstore.ReadJSON(jobsPath, &mf); err != nil {
		return RefreshResult{}, err
	}
	if mf.RunID == "" {
		mf.RunID = filepath.Base(runDir)
	}

	sourceURL := strings.TrimSpace(opts.SourceURL)
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(mf.SourceURL)
	}
	if sourceURL == "" {
		meta, err := runstore.LoadRunMeta(runDir)
		if err == nil {
			sourceURL = strings.TrimSpace(meta.SourceURL)
		}
	}
	if sourceURL == "" {
		return RefreshResult{}, fmt.Errorf("refresh requires source URL in run metadata or --source")
	}

	src, err := fetchSourceManifest(sourceURL, opts.CookiesPath, opts.CookiesFromBrowser, opts.JSRuntime)
	if err != nil {
		return RefreshResult{}, err
	}

	jobs, added := mergeJobs(mf.Jobs, src)
	pending, skippedPrivate := countJobs(jobs)
	now := time.Now().UTC()

	mf.GeneratedAt = now.Format(time.RFC3339)
	mf.SourceURL = sourceURL
	mf.SourceID = src.ID
	mf.SourceTitle = src.Title
	mf.SourceType = src.Type
	mf.PlaylistID = src.ID
	mf.PlaylistTitle = src.Title
	mf.Jobs = jobs
	mf.Total = len(jobs)
	mf.Pending = pending
	mf.Running = 0
	mf.Completed = countStatus(jobs, model.StatusCompleted)
	mf.FailedRetryable = countStatus(jobs, model.StatusFailedRetryable)
	mf.FailedPermanent = countStatus(jobs, model.StatusFailedPermanent)
	mf.SkippedPrivate = skippedPrivate

	if err := runstore.WriteJSON(jobsPath, mf); err != nil {
		return RefreshResult{}, err
	}

	rawPath := filepath.Join(runDir, "manifest.raw.json")
	if err := runstore.WriteBytes(rawPath, src.Raw); err != nil {
		return RefreshResult{}, err
	}

	meta, err := runstore.LoadRunMeta(runDir)
	if err != nil {
		meta = runstore.RunMeta{
			RunID:     mf.RunID,
			CreatedAt: now.Format(time.RFC3339),
		}
	}
	if strings.TrimSpace(meta.RunID) == "" {
		meta.RunID = mf.RunID
	}
	if strings.TrimSpace(meta.CreatedAt) == "" {
		meta.CreatedAt = now.Format(time.RFC3339)
	}
	meta.UpdatedAt = now.Format(time.RFC3339)
	meta.SourceURL = sourceURL
	meta.SourceID = src.ID
	meta.SourceTitle = src.Title
	meta.SourceType = src.Type
	meta.RawManifestPath = rawPath
	meta.JobsManifestPath = jobsPath
	meta.TotalEntries = len(jobs)
	meta.Pending = pending
	meta.SkippedPrivate = skippedPrivate
	if err := runstore.SaveRunMeta(runDir, meta); err != nil {
		return RefreshResult{}, err
	}

	return RefreshResult{
		RunID:            mf.RunID,
		RunDir:           runDir,
		RawManifestPath:  rawPath,
		JobsManifestPath: jobsPath,
		TotalEntries:     len(jobs),
		Pending:          pending,
		SkippedPrivate:   skippedPrivate,
		Added:            added,
	}, nil
}

func UpsertBySource(opts UpsertOptions) (UpsertResult, error) {
	sourceURL := strings.TrimSpace(opts.SourceURL)
	if sourceURL == "" {
		return UpsertResult{}, fmt.Errorf("source URL is required")
	}
	runsDir := strings.TrimSpace(opts.RunsDir)
	if runsDir == "" {
		runsDir = "runs"
	}

	runDir, err := latestRunDirBySource(runsDir, sourceURL)
	if err != nil {
		return UpsertResult{}, err
	}
	if runDir == "" {
		res, err := Run(Options{
			SourceURL:          sourceURL,
			Profile:            opts.Profile,
			RunsDir:            runsDir,
			CookiesPath:        opts.CookiesPath,
			CookiesFromBrowser: opts.CookiesFromBrowser,
			JSRuntime:          opts.JSRuntime,
		})
		if err != nil {
			return UpsertResult{}, err
		}
		return UpsertResult{
			Created: true,
			Result:  res,
		}, nil
	}

	ref, err := Refresh(RefreshOptions{
		RunDir:             runDir,
		RunsDir:            runsDir,
		Latest:             false,
		SourceURL:          sourceURL,
		CookiesPath:        opts.CookiesPath,
		CookiesFromBrowser: opts.CookiesFromBrowser,
		JSRuntime:          opts.JSRuntime,
	})
	if err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{
		Created: false,
		Refresh: ref,
	}, nil
}

func latestRunDirBySource(runsDir, sourceURL string) (string, error) {
	dirs, err := runstore.ListRunDirs(runsDir)
	if err != nil {
		return "", err
	}
	slices.Reverse(dirs)

	target := normalizeSourceURL(sourceURL)
	for _, runDir := range dirs {
		meta, err := runstore.LoadRunMeta(runDir)
		if err == nil && normalizeSourceURL(meta.SourceURL) == target {
			return runDir, nil
		}

		var mf model.JobsManifest
		if err := runstore.ReadJSON(filepath.Join(runDir, "manifest.jobs.json"), &mf); err == nil {
			if normalizeSourceURL(mf.SourceURL) == target {
				return runDir, nil
			}
		}
	}
	return "", nil
}

func normalizeSourceURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	u.Fragment = ""
	if strings.HasSuffix(u.Path, "/") && u.Path != "/" {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	return u.String()
}

func resolveRunDir(runDir, runID, runsDir string, latest bool) (string, error) {
	if strings.TrimSpace(runDir) != "" {
		return strings.TrimSpace(runDir), nil
	}
	baseRuns := strings.TrimSpace(runsDir)
	if baseRuns == "" {
		baseRuns = "runs"
	}
	if strings.TrimSpace(runID) != "" {
		return filepath.Join(baseRuns, strings.TrimSpace(runID)), nil
	}
	if latest {
		return runstore.LatestRunDir(baseRuns)
	}
	return "", fmt.Errorf("run target not specified")
}

func fetchSourceManifest(sourceURL, cookiesPath, cookiesFromBrowser, jsRuntime string) (sourceManifest, error) {
	raw, err := ytdlp.FlatPlaylistJSON(ytdlp.FlatPlaylistOptions{
		SourceURL:          sourceURL,
		CookiesPath:        cookiesPath,
		CookiesFromBrowser: cookiesFromBrowser,
		JSRuntime:          jsRuntime,
	})
	if err != nil {
		return sourceManifest{}, err
	}

	var c ytDLPCollection
	if err := json.Unmarshal(raw, &c); err != nil {
		return sourceManifest{}, fmt.Errorf("parse yt-dlp source JSON: %w", err)
	}
	if strings.TrimSpace(c.ID) == "" {
		c.ID = "unknown_source"
	}

	entries := make([]sourceEntry, 0, len(c.Entries))
	for _, e := range c.Entries {
		id := strings.TrimSpace(e.ID)
		entries = append(entries, sourceEntry{
			ID:       id,
			Title:    strings.TrimSpace(e.Title),
			VideoURL: resolveVideoURL(id, strings.TrimSpace(e.URL)),
			Private:  isPrivateEntryTitle(e.Title),
		})
	}

	return sourceManifest{
		ID:      c.ID,
		Title:   strings.TrimSpace(c.Title),
		Type:    detectSourceType(sourceURL),
		Raw:     raw,
		Entries: entries,
	}, nil
}

func mergeJobs(existing []model.Job, src sourceManifest) ([]model.Job, int) {
	existingByVideoID := make(map[string]model.Job, len(existing))
	for _, j := range existing {
		id := strings.TrimSpace(j.VideoID)
		if id == "" {
			continue
		}
		if j.Status == model.StatusRunning {
			_ = model.TransitionJobStatus(&j, model.StatusFailedRetryable, "interrupted_previous_run")
		}
		existingByVideoID[id] = j
	}

	jobs := make([]model.Job, 0, len(src.Entries))
	added := 0
	for i, e := range src.Entries {
		videoID := fallbackVideoID(e.ID, i+1)
		jobID := fmt.Sprintf("%s:%d:%s", src.ID, i+1, videoID)

		if old, ok := existingByVideoID[videoID]; ok {
			old.JobID = jobID
			old.Index = i + 1
			old.VideoID = videoID
			if e.Title != "" {
				old.Title = e.Title
			}
			old.VideoURL = resolveVideoURL(videoID, e.VideoURL)

			if e.Private {
				if old.Status != model.StatusCompleted {
					if err := model.TransitionJobStatus(&old, model.StatusSkippedPrivate, "private_or_unavailable"); err != nil {
						old.Status = model.StatusSkippedPrivate
						old.Reason = "private_or_unavailable"
					}
				}
			} else if old.Status == model.StatusSkippedPrivate {
				if err := model.TransitionJobStatus(&old, model.StatusPending, ""); err != nil {
					old.Status = model.StatusPending
					old.Reason = ""
				}
			}
			jobs = append(jobs, old)
			continue
		}

		job := model.Job{
			JobID:    jobID,
			Index:    i + 1,
			VideoID:  videoID,
			VideoURL: resolveVideoURL(videoID, e.VideoURL),
			Title:    e.Title,
			Status:   model.StatusPending,
		}
		if e.Private {
			job.Status = model.StatusSkippedPrivate
			job.Reason = "private_or_unavailable"
		}
		jobs = append(jobs, job)
		added++
	}

	return jobs, added
}

func countJobs(jobs []model.Job) (pending int, skippedPrivate int) {
	for _, j := range jobs {
		switch j.Status {
		case model.StatusPending:
			pending++
		case model.StatusSkippedPrivate:
			skippedPrivate++
		}
	}
	return pending, skippedPrivate
}

func countStatus(jobs []model.Job, status string) int {
	n := 0
	for _, j := range jobs {
		if j.Status == status {
			n++
		}
	}
	return n
}

func isPrivateEntryTitle(title string) bool {
	t := strings.TrimSpace(title)
	return t == "[Private video]" || t == "[Deleted video]"
}

func resolveVideoURL(videoID, maybeURL string) string {
	u := strings.TrimSpace(maybeURL)
	if u != "" {
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			return u
		}
		if strings.HasPrefix(u, "watch?") || strings.HasPrefix(u, "/watch?") {
			return "https://www.youtube.com/" + strings.TrimPrefix(u, "/")
		}
		if len(u) == 11 {
			return "https://www.youtube.com/watch?v=" + u
		}
	}
	if strings.TrimSpace(videoID) != "" {
		return "https://www.youtube.com/watch?v=" + strings.TrimSpace(videoID)
	}
	return ""
}

func detectSourceType(sourceURL string) string {
	u, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil {
		return "unknown"
	}
	if u.Query().Get("list") != "" {
		return "playlist"
	}
	path := strings.ToLower(strings.TrimSpace(u.Path))
	switch {
	case strings.HasPrefix(path, "/channel/"),
		strings.HasPrefix(path, "/@"),
		strings.HasPrefix(path, "/user/"),
		strings.HasPrefix(path, "/c/"):
		return "channel"
	default:
		return "feed"
	}
}

var invalidIDChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return invalidIDChars.ReplaceAllString(s, "_")
}

func fallbackVideoID(id string, idx int) string {
	if strings.TrimSpace(id) != "" {
		return id
	}
	return fmt.Sprintf("unknown_%d", idx)
}
