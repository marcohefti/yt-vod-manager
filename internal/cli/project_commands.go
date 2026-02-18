package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"yt-vod-manager/internal/discovery"
)

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	runsDir := fs.String("runs-dir", "runs", "runs directory")
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := discovery.InitWorkspace(discovery.InitWorkspaceOptions{
		RunsDir:    strings.TrimSpace(*runsDir),
		ConfigPath: strings.TrimSpace(*config),
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(res)
	}

	fmt.Println("workspace initialized")
	fmt.Printf("runs_dir: %s\n", res.RunsDir)
	fmt.Printf("config: %s\n", res.ConfigPath)
	fmt.Printf("created_runs_dir: %t\n", res.CreatedRunsDir)
	fmt.Printf("created_config: %t\n", res.CreatedConfig)
	fmt.Println("checks:")
	for _, c := range res.DoctorResult.Checks {
		status := "ok"
		if !c.OK {
			status = "fail"
		}
		fmt.Printf("  %s: %s (%s)\n", c.Name, status, c.Message)
	}
	if !res.DoctorResult.OK {
		return errors.New("doctor checks failed")
	}
	fmt.Println("next: yt-vod-manager add --source <url>")
	return nil
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	runsDir := fs.String("runs-dir", "runs", "runs directory")
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := discovery.Doctor(discovery.DoctorOptions{
		RunsDir:    strings.TrimSpace(*runsDir),
		ConfigPath: strings.TrimSpace(*config),
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(res)
	}

	for _, c := range res.Checks {
		status := "ok"
		if !c.OK {
			status = "fail"
		}
		fmt.Printf("%s: %s (%s)\n", c.Name, status, c.Message)
	}
	if !res.OK {
		return errors.New("doctor checks failed")
	}
	fmt.Println("doctor: all checks passed")
	return nil
}

func runAddProject(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	name := fs.String("name", "", "project name (optional; auto-generated from source)")
	source := fs.String("source", "", "source URL (playlist/channel)")
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	outputDir := fs.String("output-dir", "", "project output directory override")
	cookies := fs.String("cookies", "", "path to cookies.txt")
	useBrowserCookies := fs.Bool("browser-cookies", false, browserCookiesFlagHelp)
	workers := fs.Int("workers", 0, "project worker override (0 = inherit global/default)")
	fragments := fs.Int("fragments", discovery.DefaultFragments, "default yt-dlp fragment concurrency for this project")
	order := fs.String("order", discovery.DefaultOrder, "default order: oldest|newest|manifest")
	quality := fs.String("quality", discovery.DefaultQuality, "quality preset: best|1080p|720p")
	delivery := fs.String("delivery", "", "default delivery mode: auto|fragmented")
	subtitles := fs.Bool("subtitles", true, "download subtitles by default")
	subLangs := fs.String("sub-langs", discovery.DefaultSubtitleLanguage, "default subtitle language: english|all")
	replace := fs.Bool("replace", false, "replace project if it already exists")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	src := strings.TrimSpace(*source)
	if src == "" {
		var err error
		src, err = promptRequired("source URL")
		if err != nil {
			return err
		}
	}
	cookiesFromBrowser := ""
	if *useBrowserCookies {
		cookiesFromBrowser = discovery.DefaultBrowserCookieAgent
	}

	res, err := discovery.AddProject(discovery.AddProjectOptions{
		ConfigPath:          strings.TrimSpace(*config),
		Name:                strings.TrimSpace(*name),
		SourceURL:           src,
		Profile:             discovery.DefaultProfileName,
		OutputDir:           strings.TrimSpace(*outputDir),
		CookiesPath:         strings.TrimSpace(*cookies),
		CookiesFromBrowser:  cookiesFromBrowser,
		Workers:             *workers,
		Fragments:           *fragments,
		Order:               strings.TrimSpace(*order),
		Quality:             strings.TrimSpace(*quality),
		DeliveryMode:        strings.TrimSpace(*delivery),
		NoSubs:              !*subtitles,
		SubLangs:            strings.TrimSpace(*subLangs),
		Active:              boolPtr(true),
		ReplaceIfNameExists: *replace,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(res)
	}

	action := "added"
	if !res.Created {
		action = "updated"
	}
	fmt.Printf("project %s: %s\n", action, res.Project.Name)
	fmt.Printf("source: %s\n", res.Project.SourceURL)
	fmt.Printf("config: %s\n", strings.TrimSpace(*config))
	fmt.Printf("next: yt-vod-manager sync --project %s\n", res.Project.Name)
	return nil
}

func runListProjects(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := discovery.ListProjects(discovery.ListProjectsOptions{ConfigPath: strings.TrimSpace(*config)})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(res)
	}

	fmt.Printf("config: %s\n", res.ConfigPath)
	if len(res.Projects) == 0 {
		fmt.Println("no projects configured")
		fmt.Println("next: yt-vod-manager add --source <url>")
		return nil
	}
	for _, p := range res.Projects {
		fmt.Printf("- %s | %s\n", p.Name, p.SourceURL)
	}
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	project := fs.String("project", "", "project name or comma-separated names")
	all := fs.Bool("all", true, "show all configured projects")
	runsDir := fs.String("runs-dir", "runs", "runs directory")
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*project) != "" {
		*all = false
	}

	res, err := discovery.ProjectStatus(discovery.ProjectStatusOptions{
		ConfigPath: strings.TrimSpace(*config),
		Project:    strings.TrimSpace(*project),
		All:        *all,
		RunsDir:    strings.TrimSpace(*runsDir),
	})
	if err != nil {
		if errors.Is(err, discovery.ErrNoProjectsConfigured) {
			fmt.Println("no projects configured")
			fmt.Println("start here:")
			fmt.Println("  yt-vod-manager init")
			fmt.Println("  yt-vod-manager add --source <url> [--name <project>]")
			fmt.Println("then sync:")
			fmt.Println("  yt-vod-manager sync --all-projects")
			return nil
		}
		return err
	}
	if *jsonOut {
		return printJSON(res)
	}

	for _, row := range res.Rows {
		fmt.Printf("%s [%s]\n", row.Project, row.State)
		fmt.Printf("  source: %s\n", row.SourceURL)
		if row.RunID != "" {
			fmt.Printf("  run: %s\n", row.RunID)
		}
		fmt.Printf("  completed/pending/fail: %d/%d/%d\n", row.Completed, row.Pending, row.FailedRetryable+row.FailedPermanent)
	}
	fmt.Println("totals")
	fmt.Printf("  projects: %d\n", res.Totals.Projects)
	fmt.Printf("  healthy: %d\n", res.Totals.Healthy)
	fmt.Printf("  attention: %d\n", res.Totals.Attention)
	fmt.Printf("  never_synced: %d\n", res.Totals.NeverSynced)
	return nil
}

func runRemoveProject(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	name := fs.String("name", "", "project name")
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	yes := fs.Bool("yes", false, "skip confirmation")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	target := strings.TrimSpace(*name)
	if target == "" {
		return errors.New("--name is required")
	}
	if !*yes {
		ok, err := promptConfirm(fmt.Sprintf("remove project %q? [y/N] ", target))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("aborted")
			return nil
		}
	}

	res, err := discovery.RemoveProject(discovery.RemoveProjectOptions{
		ConfigPath: strings.TrimSpace(*config),
		Name:       target,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(res)
	}
	fmt.Printf("removed project: %s (%s)\n", res.Project.Name, res.Project.SourceURL)
	return nil
}
