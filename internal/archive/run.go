package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"yt-vod-manager/internal/model"
	"yt-vod-manager/internal/runstore"
	"yt-vod-manager/internal/ytdlp"
)

type RunOptions struct {
	RunID              string
	RunDir             string
	RunsDir            string
	Latest             bool
	OutputDir          string
	CookiesPath        string
	CookiesFromBrowser string
	SubLangs           string
	Fragments          int
	MaxJobs            int
	Workers            int
	DownloadLimitMBps  float64
	ProxyMode          string
	Proxies            []string
	NoSubs             bool
	RetryPermanent     bool
	StopOnRetryable    bool
	Progress           bool
	RawOutput          bool
	Order              string
	Quality            string
	JSRuntime          string
	DeliveryMode       string
}

type RunResult struct {
	RunID                  string
	RunDir                 string
	Processed              int
	Completed              int
	FailedRetryable        int
	FailedPermanent        int
	Pending                int
	SkippedPrivate         int
	Remaining              int
	EstimatedTotalBytes    int64
	EstimatedCompleteBytes int64
}

func Run(opts RunOptions) (RunResult, error) {
	runDir, err := resolveRunDir(opts)
	if err != nil {
		return RunResult{}, err
	}
	runLock, err := runstore.AcquireRunLock(runDir)
	if err != nil {
		return RunResult{}, err
	}
	defer func() {
		_ = runLock.Release()
	}()

	if err := ytdlp.CheckDependencies(); err != nil {
		return RunResult{}, err
	}

	jobsPath := filepath.Join(runDir, "manifest.jobs.json")
	var mf model.JobsManifest
	if err := runstore.ReadJSON(jobsPath, &mf); err != nil {
		return RunResult{}, err
	}
	if mf.RunID == "" {
		mf.RunID = filepath.Base(runDir)
	}
	resetStaleRunningJobs(&mf)
	recomputeCounts(&mf)
	if err := runstore.WriteJSON(jobsPath, mf); err != nil {
		return RunResult{}, err
	}
	sizeEstimator := loadRunSizeEstimator(runDir, mf, opts.Quality)

	runMeta, _ := runstore.LoadRunMeta(runDir)
	outputDir := strings.TrimSpace(opts.OutputDir)
	if outputDir == "" {
		if strings.TrimSpace(runMeta.OutputDir) != "" {
			outputDir = strings.TrimSpace(runMeta.OutputDir)
		} else {
			outputDir = filepath.Join(runDir, "downloads")
		}
	}
	if err := runstore.Mkdir(outputDir); err != nil {
		return RunResult{}, err
	}
	archiveFile := filepath.Join(runDir, "download-archive.txt")
	missingIDs, err := reconcileCompletedJobsWithDisk(&mf, outputDir)
	if err != nil {
		return RunResult{}, err
	}
	if len(missingIDs) > 0 {
		if _, err := pruneDownloadArchive(archiveFile, missingIDs); err != nil {
			return RunResult{}, err
		}
		recomputeCounts(&mf)
		if err := runstore.WriteJSON(jobsPath, mf); err != nil {
			return RunResult{}, err
		}
	}
	if err := saveRunMetaSnapshot(runDir, mf, outputDir); err != nil {
		return RunResult{}, err
	}

	logsDir := filepath.Join(runDir, "logs")
	if err := runstore.Mkdir(logsDir); err != nil {
		return RunResult{}, err
	}

	fragments := opts.Fragments
	if fragments <= 0 {
		fragments = 10
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = 5
	}
	proxyMode := normalizeProxyMode(opts.ProxyMode)
	proxies := normalizeProxyList(opts.Proxies)
	if proxyMode == proxyModePerWorker {
		if len(proxies) == 0 {
			return RunResult{}, fmt.Errorf("proxy mode %q requires at least one proxy", proxyModePerWorker)
		}
		if workers > len(proxies) {
			return RunResult{}, fmt.Errorf("proxy mode %q requires at least %d proxies for %d workers", proxyModePerWorker, workers, workers)
		}
	}
	dashboardEnabled := opts.Progress && workers > 1
	var dash *multiDashboard
	if dashboardEnabled {
		dash = newMultiDashboard(workers)
		dash.SetTotals(mf.Completed, mf.Total-mf.SkippedPrivate, mf.Pending, mf.FailedRetryable, mf.FailedPermanent)
		if sizeEstimator.hasEstimate() {
			dash.SetSizeEstimate(sizeEstimator.completedBytes(mf.Jobs), sizeEstimator.totalBytes)
		}
		dash.Start()
		defer dash.Stop()
	}

	orderIdx := orderedJobIndexes(mf.Jobs, opts.Order)
	jobCh := make(chan int)

	var processed atomic.Int64
	var stopRetryable atomic.Bool
	var stopAll atomic.Bool
	var stateMu sync.Mutex
	var logMu sync.Mutex
	var wg sync.WaitGroup
	var fatalErr atomic.Value
	setFatal := func(err error) {
		if err == nil {
			return
		}
		if fatalErr.Load() == nil {
			fatalErr.Store(err.Error())
		}
		stopAll.Store(true)
	}

	workerFn := func(workerID int) {
		defer wg.Done()
		workerProxy := proxyForWorker(workerID, proxyMode, proxies)
		for i := range jobCh {
			if stopAll.Load() {
				continue
			}
			if opts.StopOnRetryable && stopRetryable.Load() {
				continue
			}

			stateMu.Lock()
			if !isRunnable(mf.Jobs[i].Status, opts.RetryPermanent) {
				stateMu.Unlock()
				continue
			}

			job := &mf.Jobs[i]
			if strings.TrimSpace(job.VideoURL) == "" {
				if err := model.TransitionJobStatus(job, model.StatusFailedPermanent, "missing_video_url"); err != nil {
					stateMu.Unlock()
					setFatal(err)
					continue
				}
				job.LastError = "video URL missing in manifest"
				job.Attempts++
				job.LastAttemptAt = time.Now().UTC().Format(time.RFC3339)
				recomputeCounts(&mf)
				if err := runstore.WriteJSON(jobsPath, mf); err != nil {
					stateMu.Unlock()
					setFatal(fmt.Errorf("persist jobs manifest: %w", err))
					continue
				}
				stateMu.Unlock()
				processed.Add(1)
				continue
			}

			now := time.Now().UTC().Format(time.RFC3339)
			if err := model.TransitionJobStatus(job, model.StatusRunning, ""); err != nil {
				stateMu.Unlock()
				setFatal(err)
				continue
			}
			job.Attempts++
			job.LastAttemptAt = now
			recomputeCounts(&mf)
			if err := runstore.WriteJSON(jobsPath, mf); err != nil {
				stateMu.Unlock()
				setFatal(fmt.Errorf("persist jobs manifest: %w", err))
				continue
			}
			if dashboardEnabled {
				dash.SetTotals(mf.Completed, mf.Total-mf.SkippedPrivate, mf.Pending, mf.FailedRetryable, mf.FailedPermanent)
				if sizeEstimator.hasEstimate() {
					dash.SetSizeEstimate(sizeEstimator.completedBytes(mf.Jobs), sizeEstimator.totalBytes)
				}
			}

			downloadTarget := mf.Total - mf.SkippedPrivate
			startCompleted := mf.Completed
			startFailR := mf.FailedRetryable
			startFailP := mf.FailedPermanent

			jobIndex := job.Index
			videoID := job.VideoID
			videoURL := job.VideoURL
			title := job.Title
			stateMu.Unlock()

			progressEnabled := opts.Progress && workers == 1
			progress := newLiveProgress(
				opts.Progress,
				jobIndex,
				mf.Total,
				startCompleted,
				downloadTarget,
				startFailR,
				startFailP,
				videoID,
				title,
			)
			if progressEnabled {
				progress.Start()
			}
			if dashboardEnabled {
				dash.SetWorker(workerID, progress)
			}
			progress.SetPhase("starting")
			if !progressEnabled && !dashboardEnabled {
				logMu.Lock()
				fmt.Printf("[w%d %d/%d] start %s\n", workerID, jobIndex, mf.Total, videoID)
				logMu.Unlock()
			}

			logPath := filepath.Join(logsDir, fmt.Sprintf("%04d_%s.log", jobIndex, safeFileID(videoID, jobIndex)))
			logFile, err := os.Create(logPath)
			if err != nil {
				stateMu.Lock()
				j := &mf.Jobs[i]
				if trErr := model.TransitionJobStatus(j, model.StatusFailedPermanent, "log_file_error"); trErr != nil {
					stateMu.Unlock()
					setFatal(trErr)
					processed.Add(1)
					continue
				}
				j.LastError = err.Error()
				recomputeCounts(&mf)
				if err := runstore.WriteJSON(jobsPath, mf); err != nil {
					stateMu.Unlock()
					setFatal(fmt.Errorf("persist jobs manifest: %w", err))
					processed.Add(1)
					continue
				}
				stateMu.Unlock()
				processed.Add(1)
				continue
			}

			_, dlErr := ytdlp.DownloadVideo(ytdlp.DownloadOptions{
				VideoURL:           videoURL,
				OutputDir:          outputDir,
				Fragments:          fragments,
				DownloadArchive:    archiveFile,
				CookiesPath:        opts.CookiesPath,
				CookiesFromBrowser: opts.CookiesFromBrowser,
				Quality:            opts.Quality,
				DeliveryMode:       opts.DeliveryMode,
				DownloadLimitMBps:  opts.DownloadLimitMBps,
				ProxyURL:           workerProxy,
				Stdout:             os.Stdout,
				Stderr:             os.Stderr,
				LogWriter:          logFile,
				EchoOutput:         opts.RawOutput && !dashboardEnabled,
				Progress:           progress.Handle,
				JSRuntime:          opts.JSRuntime,
			})

			processed.Add(1)
			shouldStop := false

			stateMu.Lock()
			j := &mf.Jobs[i]
			if dlErr == nil {
				if err := model.TransitionJobStatus(j, model.StatusCompleted, ""); err != nil {
					stateMu.Unlock()
					setFatal(err)
					_ = logFile.Close()
					continue
				}
				j.LastError = ""
				j.CompletedAt = time.Now().UTC().Format(time.RFC3339)
				if !opts.NoSubs {
					progress.SetPhase("subtitles")
					_, subErr := ytdlp.DownloadSubtitles(ytdlp.DownloadOptions{
						VideoURL:           videoURL,
						OutputDir:          outputDir,
						CookiesPath:        opts.CookiesPath,
						CookiesFromBrowser: opts.CookiesFromBrowser,
						DeliveryMode:       opts.DeliveryMode,
						SubLangs:           opts.SubLangs,
						DownloadLimitMBps:  opts.DownloadLimitMBps,
						ProxyURL:           workerProxy,
						Stdout:             os.Stdout,
						Stderr:             os.Stderr,
						LogWriter:          logFile,
						EchoOutput:         opts.RawOutput && !dashboardEnabled,
						Progress:           progress.Handle,
						JSRuntime:          opts.JSRuntime,
					})
					if subErr != nil {
						logMu.Lock()
						fmt.Printf("[%d/%d] warn  subtitles failed for %s (non-fatal)\n", jobIndex, mf.Total, videoID)
						logMu.Unlock()
					}
				}
				recomputeCounts(&mf)
				if err := runstore.WriteJSON(jobsPath, mf); err != nil {
					stateMu.Unlock()
					setFatal(fmt.Errorf("persist jobs manifest: %w", err))
					_ = logFile.Close()
					continue
				}
				if dashboardEnabled {
					dash.SetTotals(mf.Completed, mf.Total-mf.SkippedPrivate, mf.Pending, mf.FailedRetryable, mf.FailedPermanent)
					if sizeEstimator.hasEstimate() {
						dash.SetSizeEstimate(sizeEstimator.completedBytes(mf.Jobs), sizeEstimator.totalBytes)
					}
				}
				stateMu.Unlock()
				doneMsg := fmt.Sprintf("[%d/%d] done  %s", jobIndex, mf.Total, videoID)
				if progressEnabled {
					progress.Stop(doneMsg)
				}
				if dashboardEnabled {
					dash.RemoveWorker(workerID, doneMsg)
				}
			} else {
				j.CompletedAt = ""
				j.LastError = truncate(dlErr.Error(), 1200)
				if isDependencyError(dlErr.Error()) {
					if err := model.TransitionJobStatus(j, model.StatusFailedPermanent, "missing_dependency"); err != nil {
						stateMu.Unlock()
						setFatal(err)
						_ = logFile.Close()
						continue
					}
				} else if isRetryableError(dlErr.Error()) {
					if err := model.TransitionJobStatus(j, model.StatusFailedRetryable, "transient_or_rate_limited"); err != nil {
						stateMu.Unlock()
						setFatal(err)
						_ = logFile.Close()
						continue
					}
					shouldStop = opts.StopOnRetryable
				} else {
					if err := model.TransitionJobStatus(j, model.StatusFailedPermanent, "download_error"); err != nil {
						stateMu.Unlock()
						setFatal(err)
						_ = logFile.Close()
						continue
					}
				}
				recomputeCounts(&mf)
				if err := runstore.WriteJSON(jobsPath, mf); err != nil {
					stateMu.Unlock()
					setFatal(fmt.Errorf("persist jobs manifest: %w", err))
					_ = logFile.Close()
					continue
				}
				if dashboardEnabled {
					dash.SetTotals(mf.Completed, mf.Total-mf.SkippedPrivate, mf.Pending, mf.FailedRetryable, mf.FailedPermanent)
					if sizeEstimator.hasEstimate() {
						dash.SetSizeEstimate(sizeEstimator.completedBytes(mf.Jobs), sizeEstimator.totalBytes)
					}
				}
				stateMu.Unlock()

				failMsg := ""
				if isDependencyError(dlErr.Error()) {
					failMsg = fmt.Sprintf("[%d/%d] fail  %s (dependency)", jobIndex, mf.Total, videoID)
				} else if isRetryableError(dlErr.Error()) {
					failMsg = fmt.Sprintf("[%d/%d] fail  %s (retryable)", jobIndex, mf.Total, videoID)
				} else {
					failMsg = fmt.Sprintf("[%d/%d] fail  %s (permanent)", jobIndex, mf.Total, videoID)
				}
				if progressEnabled {
					progress.Stop(failMsg)
				}
				if dashboardEnabled {
					dash.RemoveWorker(workerID, failMsg)
				}
			}

			_ = logFile.Close()
			if shouldStop {
				stopRetryable.Store(true)
				logMu.Lock()
				fmt.Println("stopping run after retryable failure; resume later to continue")
				logMu.Unlock()
			}
		}
	}

	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go workerFn(w)
	}

	dispatched := 0
	for _, i := range orderIdx {
		if stopAll.Load() {
			break
		}
		if opts.MaxJobs > 0 && dispatched >= opts.MaxJobs {
			break
		}
		if opts.StopOnRetryable && stopRetryable.Load() {
			break
		}
		stateMu.Lock()
		ok := isRunnable(mf.Jobs[i].Status, opts.RetryPermanent)
		stateMu.Unlock()
		if !ok {
			continue
		}
		jobCh <- i
		dispatched++
	}
	close(jobCh)
	wg.Wait()
	if msg := fatalErr.Load(); msg != nil {
		return RunResult{}, fmt.Errorf("%s", msg.(string))
	}

	stateMu.Lock()
	recomputeCounts(&mf)
	if err := runstore.WriteJSON(jobsPath, mf); err != nil {
		stateMu.Unlock()
		return RunResult{}, err
	}
	if err := saveRunMetaSnapshot(runDir, mf, outputDir); err != nil {
		stateMu.Unlock()
		return RunResult{}, err
	}
	remaining := mf.Pending + mf.FailedRetryable + mf.Running
	result := RunResult{
		RunID:                  mf.RunID,
		RunDir:                 runDir,
		Processed:              int(processed.Load()),
		Completed:              mf.Completed,
		FailedRetryable:        mf.FailedRetryable,
		FailedPermanent:        mf.FailedPermanent,
		Pending:                mf.Pending,
		SkippedPrivate:         mf.SkippedPrivate,
		Remaining:              remaining,
		EstimatedTotalBytes:    sizeEstimator.totalBytes,
		EstimatedCompleteBytes: sizeEstimator.completedBytes(mf.Jobs),
	}
	stateMu.Unlock()
	return result, nil
}

func resolveRunDir(opts RunOptions) (string, error) {
	if strings.TrimSpace(opts.RunDir) != "" {
		return opts.RunDir, nil
	}
	runsDir := strings.TrimSpace(opts.RunsDir)
	if runsDir == "" {
		runsDir = "runs"
	}
	if strings.TrimSpace(opts.RunID) != "" {
		return filepath.Join(runsDir, opts.RunID), nil
	}
	if opts.Latest {
		return runstore.LatestRunDir(runsDir)
	}
	return "", fmt.Errorf("run target not specified")
}

func isRunnable(status string, retryPermanent bool) bool {
	switch status {
	case model.StatusPending, model.StatusFailedRetryable:
		return true
	case model.StatusFailedPermanent:
		return retryPermanent
	default:
		return false
	}
}

func recomputeCounts(mf *model.JobsManifest) {
	pending := 0
	running := 0
	completed := 0
	failedRetryable := 0
	failedPermanent := 0
	skippedPrivate := 0

	for _, j := range mf.Jobs {
		switch j.Status {
		case model.StatusPending:
			pending++
		case model.StatusRunning:
			running++
		case model.StatusCompleted:
			completed++
		case model.StatusFailedRetryable:
			failedRetryable++
		case model.StatusFailedPermanent:
			failedPermanent++
		case model.StatusSkippedPrivate:
			skippedPrivate++
		}
	}

	mf.Total = len(mf.Jobs)
	mf.Pending = pending
	mf.Running = running
	mf.Completed = completed
	mf.FailedRetryable = failedRetryable
	mf.FailedPermanent = failedPermanent
	mf.SkippedPrivate = skippedPrivate
}

func resetStaleRunningJobs(mf *model.JobsManifest) {
	for i := range mf.Jobs {
		if mf.Jobs[i].Status != model.StatusRunning {
			continue
		}
		_ = model.TransitionJobStatus(&mf.Jobs[i], model.StatusFailedRetryable, "interrupted_previous_run")
		if mf.Jobs[i].LastError == "" {
			mf.Jobs[i].LastError = "previous run interrupted while this job was running"
		}
	}
}

func isRetryableError(s string) bool {
	text := strings.ToLower(s)
	hints := []string{
		"429",
		"too many requests",
		"rate limit",
		"timed out",
		"timeout",
		"temporarily unavailable",
		"connection reset",
		"service unavailable",
		"network is unreachable",
		"http error 5",
	}
	for _, h := range hints {
		if strings.Contains(text, h) {
			return true
		}
	}
	return false
}

func isDependencyError(s string) bool {
	text := strings.ToLower(s)
	hints := []string{
		"ffmpeg could not be found",
		"ffprobe could not be found",
	}
	for _, h := range hints {
		if strings.Contains(text, h) {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func safeFileID(id string, idx int) string {
	id = strings.TrimSpace(id)
	if id != "" {
		return id
	}
	return fmt.Sprintf("unknown_%d", idx)
}

func orderedJobIndexes(jobs []model.Job, order string) []int {
	idx := make([]int, 0, len(jobs))
	for i := range jobs {
		idx = append(idx, i)
	}

	switch strings.ToLower(strings.TrimSpace(order)) {
	case "", "oldest":
		slices.SortFunc(idx, func(a, b int) int {
			if jobs[a].Index != jobs[b].Index {
				return jobs[b].Index - jobs[a].Index
			}
			return b - a
		})
	case "newest":
		slices.SortFunc(idx, func(a, b int) int {
			if jobs[a].Index != jobs[b].Index {
				return jobs[a].Index - jobs[b].Index
			}
			return a - b
		})
	case "manifest":
		// keep as-is
	default:
		// unknown values fall back to oldest to keep behavior deterministic
		slices.SortFunc(idx, func(a, b int) int {
			if jobs[a].Index != jobs[b].Index {
				return jobs[b].Index - jobs[a].Index
			}
			return b - a
		})
	}
	return idx
}

func saveRunMetaSnapshot(runDir string, mf model.JobsManifest, outputDir string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	meta, err := runstore.LoadRunMeta(runDir)
	if err != nil {
		meta = runstore.RunMeta{
			RunID:     mf.RunID,
			CreatedAt: now,
		}
	}
	if strings.TrimSpace(meta.RunID) == "" {
		meta.RunID = mf.RunID
	}
	if strings.TrimSpace(meta.CreatedAt) == "" {
		meta.CreatedAt = now
	}
	if strings.TrimSpace(meta.Profile) == "" {
		meta.Profile = mf.Profile
	}
	meta.UpdatedAt = now
	meta.SourceURL = mf.SourceURL
	meta.SourceID = mf.SourceID
	meta.SourceTitle = mf.SourceTitle
	meta.SourceType = mf.SourceType
	meta.RawManifestPath = filepath.Join(runDir, "manifest.raw.json")
	meta.JobsManifestPath = filepath.Join(runDir, "manifest.jobs.json")
	meta.OutputDir = outputDir
	meta.TotalEntries = mf.Total
	meta.Pending = mf.Pending
	meta.SkippedPrivate = mf.SkippedPrivate
	return runstore.SaveRunMeta(runDir, meta)
}

func reconcileCompletedJobsWithDisk(mf *model.JobsManifest, outputDir string) ([]string, error) {
	present, err := indexMediaByVideoID(outputDir)
	if err != nil {
		return nil, err
	}
	missingIDs := make([]string, 0)
	for i := range mf.Jobs {
		j := &mf.Jobs[i]
		if j.Status != model.StatusCompleted {
			continue
		}
		videoID := strings.TrimSpace(j.VideoID)
		if videoID == "" {
			continue
		}
		if !present[videoID] {
			if err := model.TransitionJobStatus(j, model.StatusPending, "missing_local_media"); err != nil {
				return nil, err
			}
			j.CompletedAt = ""
			j.LastError = "previously completed but media file is missing locally"
			missingIDs = append(missingIDs, videoID)
		}
	}
	return missingIDs, nil
}

var outputIDPattern = regexp.MustCompile(`\[([A-Za-z0-9_-]{6,})\]\.[^.]+$`)

func indexMediaByVideoID(root string) (map[string]bool, error) {
	ids := make(map[string]bool)
	if strings.TrimSpace(root) == "" {
		return ids, nil
	}
	mediaExt := map[string]struct{}{
		"mp4": {}, "mkv": {}, "webm": {}, "m4v": {},
		"mov": {}, "avi": {}, "flv": {}, "ts": {}, "m4a": {}, "mp3": {},
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".part") || strings.HasSuffix(lower, ".ytdl") || strings.HasSuffix(lower, ".tmp") {
			return nil
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		if _, ok := mediaExt[ext]; !ok {
			return nil
		}
		m := outputIDPattern.FindStringSubmatch(name)
		if len(m) > 1 {
			ids[m[1]] = true
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return ids, nil
		}
		return nil, err
	}
	return ids, nil
}

func pruneDownloadArchive(path string, videoIDs []string) (int, error) {
	if len(videoIDs) == 0 {
		return 0, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	toDrop := make(map[string]struct{}, len(videoIDs))
	for _, id := range videoIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			toDrop[id] = struct{}{}
		}
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines))
	removed := 0
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		fields := strings.Fields(t)
		if len(fields) > 0 {
			if _, ok := toDrop[fields[len(fields)-1]]; ok {
				removed++
				continue
			}
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return 0, nil
	}

	out := strings.Join(kept, "\n")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return 0, err
	}
	return removed, nil
}
