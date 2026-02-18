package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"yt-vod-manager/internal/archive"
	"yt-vod-manager/internal/discovery"
)

type syncSourceItem struct {
	Project            string
	SourceURL          string
	Profile            string
	OutputDir          string
	CookiesPath        string
	CookiesFromBrowser string
	Workers            int
	Fragments          int
	Order              string
	Quality            string
	DeliveryMode       string
	NoSubs             bool
	SubLangs           string
}

type syncSourceReport struct {
	Project             string `json:"project,omitempty"`
	SourceURL           string `json:"source_url"`
	RunID               string `json:"run_id,omitempty"`
	RunDir              string `json:"run_dir,omitempty"`
	Created             bool   `json:"created"`
	AddedNewEntries     int    `json:"added_new_entries"`
	TotalEntries        int    `json:"total_entries"`
	Pending             int    `json:"pending_count"`
	SkippedPrivate      int    `json:"skipped_private_count"`
	ProcessedNow        int    `json:"processed_now,omitempty"`
	CompletedTotal      int    `json:"completed_total,omitempty"`
	PendingTotal        int    `json:"pending_total,omitempty"`
	FailedRetryable     int    `json:"failed_retryable,omitempty"`
	FailedPermanent     int    `json:"failed_permanent,omitempty"`
	Remaining           int    `json:"remaining,omitempty"`
	EstimatedTotalBytes int64  `json:"estimated_total_bytes,omitempty"`
	EstimatedDoneBytes  int64  `json:"estimated_done_bytes,omitempty"`
	Error               string `json:"error,omitempty"`
}

type syncResult struct {
	Sources         int                `json:"sources"`
	AddedNewEntries int                `json:"added_new_entries"`
	ProcessedNow    int                `json:"processed_now,omitempty"`
	CompletedTotal  int                `json:"completed_total,omitempty"`
	PendingTotal    int                `json:"pending_total,omitempty"`
	Failures        int                `json:"failures"`
	Reports         []syncSourceReport `json:"reports"`
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	source := fs.String("source", "", "single source URL (playlist/channel)")
	fetchlist := fs.String("fetchlist", "", "file with one source URL per line")
	project := fs.String("project", "", "project name or comma-separated project names")
	allProjects := fs.Bool("all-projects", false, "sync all configured projects")
	activeOnly := fs.Bool("active-only", false, "only sync projects marked active")
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	runsDir := fs.String("runs-dir", "runs", "runs directory")
	noRun := fs.Bool("no-run", false, "only refresh/discover; do not download")
	continueOnError := fs.Bool("continue-on-error", true, "continue processing other sources if one fails")

	maxJobs := fs.Int("max-jobs", 0, "max jobs per source this invocation (0 = no limit)")
	workers := fs.Int("workers", 0, "number of parallel video workers (0 = project/default)")
	retryPermanent := fs.Bool("retry-permanent", false, "also retry jobs currently marked failed_permanent")
	stopOnRetryable := fs.Bool("stop-on-retryable", true, "stop run after first retryable failure")
	fragments := fs.Int("fragments", 0, "yt-dlp fragment concurrency (-N); 0 = project/default")
	order := fs.String("order", "", "job processing order: oldest|newest|manifest")
	quality := fs.String("quality", "", "quality preset: best|1080p|720p")
	delivery := fs.String("delivery", "", "delivery mode: auto|fragmented")
	progress := fs.Bool("progress", true, "show live progress renderer")
	rawOutput := fs.Bool("raw-output", false, "print raw yt-dlp/ffmpeg output lines (verbose)")
	outputDir := fs.String("output-dir", "", "download output dir (default: <run_dir>/downloads)")
	subtitles := fs.String("subtitles", "auto", "subtitle download: auto|yes|no")
	subLangs := fs.String("sub-langs", "", "subtitle language preference: english|all")
	cookies := fs.String("cookies", "", "path to cookies.txt")
	useBrowserCookies := fs.Bool("browser-cookies", false, browserCookiesFlagHelp)
	jsonOut := fs.Bool("json", false, "print JSON output")

	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	items, err := collectSyncItems(
		strings.TrimSpace(*source),
		strings.TrimSpace(*fetchlist),
		strings.TrimSpace(*project),
		*allProjects,
		*activeOnly,
		strings.TrimSpace(*config),
	)
	if err != nil {
		return err
	}
	cliCookiesFromBrowser := ""
	if *useBrowserCookies {
		cliCookiesFromBrowser = discovery.DefaultBrowserCookieAgent
	}
	progressEnabled := *progress && !*jsonOut
	if !*jsonOut {
		if *noRun {
			fmt.Printf("sync: refreshing %d source(s)...\n", len(items))
		} else {
			fmt.Printf("sync: refreshing %d source(s) before downloads...\n", len(items))
		}
	}

	totalProcessed := 0
	totalCompleted := 0
	totalPending := 0
	totalAdded := 0
	totalEstimatedBytes := int64(0)
	totalEstimatedDoneBytes := int64(0)
	failures := 0
	reports := make([]syncSourceReport, 0, len(items))

	for idx, item := range items {
		report := syncSourceReport{
			Project:   item.Project,
			SourceURL: item.SourceURL,
		}
		sourceLabel := firstNonEmpty(item.Project, item.SourceURL)
		if !*jsonOut {
			fmt.Printf("[%d/%d] refreshing %s\n", idx+1, len(items), sourceLabel)
		}
		refreshStart := time.Now()
		upsert, err := discovery.UpsertBySource(discovery.UpsertOptions{
			SourceURL:          item.SourceURL,
			Profile:            firstNonEmpty(item.Profile, discovery.DefaultProfileName),
			RunsDir:            strings.TrimSpace(*runsDir),
			CookiesPath:        firstNonEmpty(strings.TrimSpace(*cookies), item.CookiesPath),
			CookiesFromBrowser: firstNonEmpty(cliCookiesFromBrowser, item.CookiesFromBrowser),
		})
		if err != nil {
			failures++
			report.Error = err.Error()
			reports = append(reports, report)
			fmt.Fprintf(os.Stderr, "sync failed for %s: %v\n", item.SourceURL, err)
			if !*continueOnError {
				result := syncResult{Sources: len(items), AddedNewEntries: totalAdded, Failures: failures, Reports: reports}
				if !*noRun {
					result.ProcessedNow = totalProcessed
					result.CompletedTotal = totalCompleted
					result.PendingTotal = totalPending
				}
				if *jsonOut {
					_ = printJSON(result)
				}
				return err
			}
			continue
		}

		runDir := upsert.Result.RunDir
		runID := upsert.Result.RunID
		report.RunID = runID
		report.RunDir = runDir
		if upsert.Created {
			report.Created = true
			report.TotalEntries = upsert.Result.TotalEntries
			report.Pending = upsert.Result.Pending
			report.SkippedPrivate = upsert.Result.SkippedPrivate
			report.AddedNewEntries = upsert.Result.Pending + upsert.Result.SkippedPrivate
		} else {
			runDir = upsert.Refresh.RunDir
			runID = upsert.Refresh.RunID
			report.RunID = runID
			report.RunDir = runDir
			report.TotalEntries = upsert.Refresh.TotalEntries
			report.Pending = upsert.Refresh.Pending
			report.SkippedPrivate = upsert.Refresh.SkippedPrivate
			report.AddedNewEntries = upsert.Refresh.Added
		}
		totalAdded += report.AddedNewEntries
		if !*jsonOut {
			fmt.Printf("[%d/%d] refreshed in %s (+%d new, pending %d)\n",
				idx+1,
				len(items),
				time.Since(refreshStart).Round(time.Millisecond),
				report.AddedNewEntries,
				report.Pending,
			)
		}

		if *noRun {
			reports = append(reports, report)
			continue
		}

		effectiveWorkers := firstNonZero(*workers, item.Workers)
		effectiveFragments := firstNonZero(*fragments, item.Fragments)
		effectiveOrder := firstNonEmpty(strings.TrimSpace(*order), item.Order, discovery.DefaultOrder)
		effectiveQuality := firstNonEmpty(strings.TrimSpace(*quality), item.Quality, discovery.DefaultQuality)
		effectiveDelivery := firstNonEmpty(strings.TrimSpace(*delivery), item.DeliveryMode, "auto")
		effectiveOutputDir := firstNonEmpty(strings.TrimSpace(*outputDir), item.OutputDir)
		effectiveSubLangs := firstNonEmpty(strings.TrimSpace(*subLangs), item.SubLangs, discovery.DefaultSubtitleLanguage)
		effectiveNoSubs, err := resolveNoSubs(strings.TrimSpace(*subtitles), item.NoSubs)
		if err != nil {
			return err
		}

		if !*jsonOut {
			fmt.Printf("[%d/%d] starting download phase...\n", idx+1, len(items))
		}
		res, runErr := archive.Run(archive.RunOptions{
			RunDir:             runDir,
			RunID:              runID,
			RunsDir:            strings.TrimSpace(*runsDir),
			Latest:             false,
			OutputDir:          effectiveOutputDir,
			CookiesPath:        firstNonEmpty(strings.TrimSpace(*cookies), item.CookiesPath),
			CookiesFromBrowser: firstNonEmpty(cliCookiesFromBrowser, item.CookiesFromBrowser),
			SubLangs:           effectiveSubLangs,
			Fragments:          effectiveFragments,
			MaxJobs:            *maxJobs,
			Workers:            effectiveWorkers,
			NoSubs:             effectiveNoSubs,
			RetryPermanent:     *retryPermanent,
			StopOnRetryable:    *stopOnRetryable,
			Progress:           progressEnabled,
			RawOutput:          *rawOutput,
			Order:              effectiveOrder,
			Quality:            effectiveQuality,
			DeliveryMode:       effectiveDelivery,
		})
		if runErr != nil {
			failures++
			report.Error = runErr.Error()
			reports = append(reports, report)
			fmt.Fprintf(os.Stderr, "run failed for %s: %v\n", item.SourceURL, runErr)
			if !*continueOnError {
				result := syncResult{Sources: len(items), AddedNewEntries: totalAdded, Failures: failures, Reports: reports}
				if !*noRun {
					result.ProcessedNow = totalProcessed
					result.CompletedTotal = totalCompleted
					result.PendingTotal = totalPending
				}
				if *jsonOut {
					_ = printJSON(result)
				}
				return runErr
			}
			continue
		}

		totalProcessed += res.Processed
		totalCompleted += res.Completed
		totalPending += res.Pending
		report.ProcessedNow = res.Processed
		report.CompletedTotal = res.Completed
		report.PendingTotal = res.Pending
		report.FailedRetryable = res.FailedRetryable
		report.FailedPermanent = res.FailedPermanent
		report.Remaining = res.Remaining
		report.EstimatedTotalBytes = res.EstimatedTotalBytes
		report.EstimatedDoneBytes = res.EstimatedCompleteBytes
		totalEstimatedBytes += res.EstimatedTotalBytes
		totalEstimatedDoneBytes += res.EstimatedCompleteBytes
		reports = append(reports, report)
	}

	result := syncResult{
		Sources:         len(items),
		AddedNewEntries: totalAdded,
		Failures:        failures,
		Reports:         reports,
	}
	if !*noRun {
		result.ProcessedNow = totalProcessed
		result.CompletedTotal = totalCompleted
		result.PendingTotal = totalPending
	}
	if *jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		fmt.Println("sync summary")
		fmt.Printf("sources: %d\n", len(items))
		fmt.Printf("added_new_entries: %d\n", totalAdded)
		if !*noRun {
			fmt.Printf("processed_now: %d\n", totalProcessed)
			fmt.Printf("completed_total: %d\n", totalCompleted)
			fmt.Printf("pending_total: %d\n", totalPending)
			if totalEstimatedBytes > 0 {
				fmt.Printf("estimated_size_total: %s\n", formatBytesIEC(totalEstimatedBytes))
				fmt.Printf("estimated_size_downloaded: %s\n", formatBytesIEC(totalEstimatedDoneBytes))
			}
		}
		fmt.Printf("failures: %d\n", failures)
	}

	if failures > 0 {
		return fmt.Errorf("sync finished with %d failure(s)", failures)
	}
	return nil
}

func collectSyncItems(singleSource, fetchlistPath, projectNames string, allProjects bool, activeOnly bool, configPath string) ([]syncSourceItem, error) {
	hasSourceInputs := strings.TrimSpace(singleSource) != "" || strings.TrimSpace(fetchlistPath) != ""
	hasProjectInputs := strings.TrimSpace(projectNames) != "" || allProjects
	if hasSourceInputs && hasProjectInputs {
		return nil, errors.New("sync source mode and project mode are mutually exclusive")
	}
	if activeOnly && !hasProjectInputs {
		return nil, errors.New("--active-only requires --project or --all-projects")
	}

	items := make([]syncSourceItem, 0)
	seen := make(map[string]bool)
	appendSource := func(s syncSourceItem) {
		s.SourceURL = strings.TrimSpace(s.SourceURL)
		if s.SourceURL == "" {
			return
		}
		key := normalizeSourceURLSimple(s.SourceURL)
		if seen[key] {
			return
		}
		seen[key] = true
		items = append(items, s)
	}

	if hasProjectInputs {
		projects, err := discovery.ResolveProjectSelectionFiltered(configPath, projectNames, allProjects, activeOnly)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			appendSource(syncSourceItem{
				Project:            p.Name,
				SourceURL:          p.SourceURL,
				Profile:            firstNonEmpty(p.Profile, discovery.DefaultProfileName),
				OutputDir:          p.OutputDir,
				CookiesPath:        p.CookiesPath,
				CookiesFromBrowser: p.CookiesFromBrowser,
				Workers:            p.Workers,
				Fragments:          p.Fragments,
				Order:              p.Order,
				Quality:            p.Quality,
				DeliveryMode:       p.DeliveryMode,
				NoSubs:             p.NoSubs,
				SubLangs:           p.SubLangs,
			})
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("no projects selected")
		}
		return items, nil
	}

	appendSource(syncSourceItem{
		SourceURL: singleSource,
		Profile:   discovery.DefaultProfileName,
	})

	if strings.TrimSpace(fetchlistPath) != "" {
		f, err := os.Open(fetchlistPath)
		if err != nil {
			return nil, fmt.Errorf("open fetchlist %s: %w", fetchlistPath, err)
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			item := syncSourceItem{Profile: discovery.DefaultProfileName}
			if strings.Contains(line, "|") {
				parts := strings.SplitN(line, "|", 2)
				item.Project = strings.TrimSpace(parts[0])
				item.SourceURL = strings.TrimSpace(parts[1])
			} else {
				item.SourceURL = line
			}
			appendSource(item)
		}
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("read fetchlist %s: %w", fetchlistPath, err)
		}
	}

	if len(items) == 0 {
		return nil, errors.New("sync requires --source, --fetchlist, --project, or --all-projects")
	}
	return items, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func normalizeSourceURLSimple(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	return strings.TrimSuffix(s, "/")
}

func resolveNoSubs(mode string, projectNoSubs bool) (bool, error) {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "", "auto":
		return projectNoSubs, nil
	case "yes", "true":
		return false, nil
	case "no", "false":
		return true, nil
	default:
		return false, fmt.Errorf("invalid --subtitles value %q (use auto|yes|no)", mode)
	}
}
