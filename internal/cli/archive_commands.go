package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"yt-vod-manager/internal/archive"
	"yt-vod-manager/internal/discovery"
)

func runDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	source := fs.String("source", "", "source URL (playlist/channel/etc)")
	runsDir := fs.String("runs-dir", "runs", "runs directory")
	cookies := fs.String("cookies", "", "path to cookies.txt")
	useBrowserCookies := fs.Bool("browser-cookies", false, browserCookiesFlagHelp)
	jsRuntime := fs.String("js-runtime", "", "JavaScript runtime override for yt-dlp extractor scripts: auto|deno|node|quickjs|bun")
	jsonOut := fs.Bool("json", false, "print JSON output")

	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*source) == "" {
		fs.Usage()
		return errors.New("--source is required")
	}

	cookiesFromBrowser := ""
	if *useBrowserCookies {
		cookiesFromBrowser = discovery.DefaultBrowserCookieAgent
	}

	result, err := discovery.Run(discovery.Options{
		SourceURL:          *source,
		Profile:            discovery.DefaultProfileName,
		RunsDir:            *runsDir,
		CookiesPath:        strings.TrimSpace(*cookies),
		CookiesFromBrowser: cookiesFromBrowser,
		JSRuntime:          firstNonEmpty(strings.TrimSpace(*jsRuntime), discovery.DefaultJSRuntime),
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("run_id: %s\n", result.RunID)
	fmt.Printf("run_dir: %s\n", result.RunDir)
	fmt.Printf("raw_manifest: %s\n", result.RawManifestPath)
	fmt.Printf("jobs_manifest: %s\n", result.JobsManifestPath)
	fmt.Printf("total_entries: %d\n", result.TotalEntries)
	fmt.Printf("pending: %d\n", result.Pending)
	fmt.Printf("skipped_private: %d\n", result.SkippedPrivate)
	fmt.Printf("effective_js_runtime: %s\n", firstNonEmpty(strings.TrimSpace(*jsRuntime), discovery.DefaultJSRuntime))
	return nil
}

func runRefresh(args []string) error {
	fs := flag.NewFlagSet("refresh", flag.ContinueOnError)
	runID := fs.String("run-id", "", "run id from runs/<run_id>")
	runDir := fs.String("run-dir", "", "explicit run directory path")
	runsDir := fs.String("runs-dir", "runs", "runs directory")
	latest := fs.Bool("latest", false, "use latest run when run-id/run-dir/project are not set")
	project := fs.String("project", "", "project name (uses latest run for that project)")
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	source := fs.String("source", "", "optional source URL override")
	cookies := fs.String("cookies", "", "path to cookies.txt")
	useBrowserCookies := fs.Bool("browser-cookies", false, browserCookiesFlagHelp)
	jsRuntime := fs.String("js-runtime", "", "JavaScript runtime override for yt-dlp extractor scripts: auto|deno|node|quickjs|bun")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	targetRunDir := strings.TrimSpace(*runDir)
	projectDefaults := discovery.Project{}
	if strings.TrimSpace(*project) != "" {
		resolved, proj, err := discovery.ResolveRunDirForProject(strings.TrimSpace(*config), strings.TrimSpace(*project), strings.TrimSpace(*runsDir))
		if err != nil {
			return err
		}
		targetRunDir = resolved
		projectDefaults = proj
		if strings.TrimSpace(*source) == "" {
			*source = proj.SourceURL
		}
		if strings.TrimSpace(*cookies) == "" {
			*cookies = proj.CookiesPath
		}
	}
	if targetRunDir == "" && strings.TrimSpace(*runID) == "" && !*latest {
		return errors.New("refresh target required: set --run-id, --run-dir, --project, or --latest")
	}
	cookiesFromBrowser := strings.TrimSpace(projectDefaults.CookiesFromBrowser)
	if *useBrowserCookies {
		cookiesFromBrowser = discovery.DefaultBrowserCookieAgent
	}

	res, err := discovery.Refresh(discovery.RefreshOptions{
		RunID:              strings.TrimSpace(*runID),
		RunDir:             targetRunDir,
		RunsDir:            strings.TrimSpace(*runsDir),
		Latest:             *latest,
		SourceURL:          strings.TrimSpace(*source),
		CookiesPath:        strings.TrimSpace(*cookies),
		CookiesFromBrowser: cookiesFromBrowser,
		JSRuntime:          firstNonEmpty(strings.TrimSpace(*jsRuntime), projectDefaults.JSRuntime, discovery.DefaultJSRuntime),
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(res)
	}
	fmt.Printf("run_id: %s\n", res.RunID)
	fmt.Printf("run_dir: %s\n", res.RunDir)
	fmt.Printf("added_new: %d\n", res.Added)
	fmt.Printf("total_entries: %d\n", res.TotalEntries)
	fmt.Printf("pending: %d\n", res.Pending)
	fmt.Printf("skipped_private: %d\n", res.SkippedPrivate)
	fmt.Printf("effective_js_runtime: %s\n", firstNonEmpty(strings.TrimSpace(*jsRuntime), projectDefaults.JSRuntime, discovery.DefaultJSRuntime))
	return nil
}

func runArchive(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	runID := fs.String("run-id", "", "run id from runs/<run_id>")
	runDir := fs.String("run-dir", "", "explicit run directory path")
	runsDir := fs.String("runs-dir", "runs", "runs directory")
	latest := fs.Bool("latest", false, "use latest run when run-id/run-dir/project are not set")
	project := fs.String("project", "", "project name (uses latest run for that project)")
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	maxJobs := fs.Int("max-jobs", 0, "max jobs to process this invocation (0 = no limit)")
	workers := fs.Int("workers", 0, "number of parallel video workers (0 = project/default)")
	downloadLimitMBps := fs.Float64("download-limit-mb-s", -1, "download limit in MB/s (0 = unlimited, -1 = global/default)")
	retryPermanent := fs.Bool("retry-permanent", false, "also retry jobs currently marked failed_permanent")
	stopOnRetryable := fs.Bool("stop-on-retryable", true, "stop run after first retryable failure")
	fragments := fs.Int("fragments", 0, "yt-dlp fragment concurrency (-N); 0 = project/default")
	order := fs.String("order", "", "job processing order: oldest|newest|manifest")
	quality := fs.String("quality", "", "quality preset: best|1080p|720p")
	jsRuntime := fs.String("js-runtime", "", "JavaScript runtime override for yt-dlp extractor scripts: auto|deno|node|quickjs|bun")
	delivery := fs.String("delivery", "auto", "delivery mode: auto|fragmented")
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
	if *downloadLimitMBps < -1 {
		return errors.New("--download-limit-mb-s must be >= 0, or -1 to keep global/default")
	}
	configPath := strings.TrimSpace(*config)

	targetRunDir := strings.TrimSpace(*runDir)
	projectDefaults := discovery.Project{}
	if strings.TrimSpace(*project) != "" {
		resolved, proj, err := discovery.ResolveRunDirForProject(configPath, strings.TrimSpace(*project), strings.TrimSpace(*runsDir))
		if err != nil {
			return err
		}
		targetRunDir = resolved
		projectDefaults = proj
		if strings.TrimSpace(*cookies) == "" {
			*cookies = proj.CookiesPath
		}
	}
	if targetRunDir == "" && strings.TrimSpace(*runID) == "" && !*latest {
		return errors.New("run target required: set --run-id, --run-dir, --project, or --latest")
	}
	effectiveCookiesFromBrowser := strings.TrimSpace(projectDefaults.CookiesFromBrowser)
	if *useBrowserCookies {
		effectiveCookiesFromBrowser = discovery.DefaultBrowserCookieAgent
	}

	global, err := discovery.ReadGlobalSettings(configPath)
	if err != nil {
		return err
	}
	var limitOverride *float64
	if *downloadLimitMBps >= 0 {
		v := *downloadLimitMBps
		limitOverride = &v
	}
	networkSettings, err := discovery.ResolveRuntimeNetworkSettings(projectDefaults, global, *workers, limitOverride)
	if err != nil {
		return err
	}

	effectiveFragments := firstNonZero(*fragments, projectDefaults.Fragments, discovery.DefaultFragments)
	effectiveOrder := firstNonEmpty(strings.TrimSpace(*order), projectDefaults.Order, discovery.DefaultOrder)
	effectiveQuality := firstNonEmpty(strings.TrimSpace(*quality), projectDefaults.Quality, discovery.DefaultQuality)
	effectiveSubLangs := firstNonEmpty(strings.TrimSpace(*subLangs), projectDefaults.SubLangs, discovery.DefaultSubtitleLanguage)
	effectiveJSRuntime := firstNonEmpty(strings.TrimSpace(*jsRuntime), projectDefaults.JSRuntime, discovery.DefaultJSRuntime)
	effectiveDelivery := firstNonEmpty(strings.TrimSpace(*delivery), projectDefaults.DeliveryMode, "auto")
	effectiveNoSubs, err := resolveNoSubs(strings.TrimSpace(*subtitles), projectDefaults.NoSubs)
	if err != nil {
		return err
	}

	result, err := archive.Run(archive.RunOptions{
		RunID:              strings.TrimSpace(*runID),
		RunDir:             targetRunDir,
		RunsDir:            strings.TrimSpace(*runsDir),
		Latest:             *latest,
		OutputDir:          strings.TrimSpace(*outputDir),
		CookiesPath:        strings.TrimSpace(*cookies),
		CookiesFromBrowser: effectiveCookiesFromBrowser,
		SubLangs:           effectiveSubLangs,
		Fragments:          effectiveFragments,
		MaxJobs:            *maxJobs,
		Workers:            networkSettings.Workers,
		DownloadLimitMBps:  networkSettings.DownloadLimitMBps,
		ProxyMode:          networkSettings.ProxyMode,
		Proxies:            networkSettings.Proxies,
		NoSubs:             effectiveNoSubs,
		RetryPermanent:     *retryPermanent,
		StopOnRetryable:    *stopOnRetryable,
		Progress:           *progress,
		RawOutput:          *rawOutput,
		Order:              effectiveOrder,
		Quality:            effectiveQuality,
		JSRuntime:          effectiveJSRuntime,
		DeliveryMode:       effectiveDelivery,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(result)
	}
	fmt.Println("run summary")
	fmt.Printf("run_id: %s\n", result.RunID)
	fmt.Printf("run_dir: %s\n", result.RunDir)
	fmt.Printf("processed_now: %d\n", result.Processed)
	fmt.Printf("completed_total: %d\n", result.Completed)
	fmt.Printf("failed_retryable: %d\n", result.FailedRetryable)
	fmt.Printf("failed_permanent: %d\n", result.FailedPermanent)
	fmt.Printf("pending: %d\n", result.Pending)
	fmt.Printf("skipped_private: %d\n", result.SkippedPrivate)
	fmt.Printf("effective_js_runtime: %s\n", effectiveJSRuntime)
	fmt.Printf("downloaded_progress: %d/%d\n", result.Completed, result.Completed+result.Pending+result.FailedRetryable+result.FailedPermanent)
	if result.EstimatedTotalBytes > 0 {
		fmt.Printf("estimated_size_total: %s\n", formatBytesIEC(result.EstimatedTotalBytes))
		fmt.Printf("estimated_size_downloaded: %s\n", formatBytesIEC(result.EstimatedCompleteBytes))
	}
	fmt.Printf("remaining_runnable: %d\n", result.Remaining)
	if result.Remaining > 0 {
		fmt.Println("next: rerun `yt-vod-manager run` for the same target to continue")
	}
	return nil
}
