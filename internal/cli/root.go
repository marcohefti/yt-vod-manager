package cli

import "fmt"

func Run(args []string) error {
	if len(args) == 0 {
		printRootUsage()
		return nil
	}

	var err error
	switch args[0] {
	case "discover":
		err = runDiscover(args[1:])
	case "refresh":
		err = runRefresh(args[1:])
	case "sync":
		err = runSync(args[1:])
	case "run":
		err = runArchive(args[1:])
	case "init":
		err = runInit(args[1:])
	case "doctor":
		err = runDoctor(args[1:])
	case "add":
		err = runAddProject(args[1:])
	case "list":
		err = runListProjects(args[1:])
	case "manage":
		err = runManage(args[1:])
	case "settings":
		err = runSettings(args[1:])
	case "self-update":
		err = runSelfUpdate(args[1:])
	case "status":
		err = runStatus(args[1:])
	case "remove":
		err = runRemoveProject(args[1:])
	case "help", "-h", "--help":
		printRootUsage()
		return nil
	default:
		printRootUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}

	if err != nil {
		return err
	}

	maybePrintUpdateHint(args)
	return nil
}

func printRootUsage() {
	fmt.Println("yt-vod-manager: project-first YouTube archive orchestrator")
	fmt.Println()
	fmt.Println("Quick Start:")
	fmt.Println("  yt-vod-manager init")
	fmt.Println("  yt-vod-manager add --source <url> [--name <project>]")
	fmt.Println("  yt-vod-manager sync --all-projects")
	fmt.Println("  yt-vod-manager status --all")
	fmt.Println()
	fmt.Println("Project Commands:")
	fmt.Println("  init      create workspace config + run environment checks")
	fmt.Println("  doctor    run dependency and filesystem preflight checks")
	fmt.Println("  add       add/update a project source in config")
	fmt.Println("  list      list configured projects")
	fmt.Println("  manage    interactive project manager (wizard + editor)")
	fmt.Println("  settings  show/update global runtime settings")
	fmt.Println("  self-update update the CLI from GitHub Releases")
	fmt.Println("  sync      sync project(s), source URL(s), or fetchlist")
	fmt.Println("  status    status rollup for project(s)")
	fmt.Println("  remove    remove a project from config")
	fmt.Println()
	fmt.Println("Advanced Commands:")
	fmt.Println("  discover  fetch source manifest via yt-dlp and write normalized jobs")
	fmt.Println("  refresh   merge new source entries into an existing run")
	fmt.Println("  run       download pending jobs and checkpoint progress after each video")
	fmt.Println()
	fmt.Println("Notes:")
	fmt.Println("  - Use --json on commands for machine-readable output")
	fmt.Println("  - For advanced run/refresh commands, explicit targeting is safer:")
	fmt.Println("      --run-id <id>, --run-dir <path>, --project <name>, or --latest")
}
