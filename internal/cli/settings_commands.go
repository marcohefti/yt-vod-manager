package cli

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"yt-vod-manager/internal/discovery"
)

func runSettings(args []string) error {
	if len(args) == 0 {
		printSettingsUsage()
		return nil
	}
	switch args[0] {
	case "show":
		return runSettingsShow(args[1:])
	case "set":
		return runSettingsSet(args[1:])
	case "proxy":
		return runSettingsProxy(args[1:])
	case "help", "-h", "--help":
		printSettingsUsage()
		return nil
	default:
		printSettingsUsage()
		return fmt.Errorf("unknown settings subcommand %q", args[0])
	}
}

func runSettingsShow(args []string) error {
	fs := flag.NewFlagSet("settings show", flag.ContinueOnError)
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	global, err := discovery.GetGlobalSettings(strings.TrimSpace(*config))
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]any{
			"config_path": strings.TrimSpace(*config),
			"global":      global,
		})
	}

	fmt.Printf("config: %s\n", strings.TrimSpace(*config))
	fmt.Printf("workers: %d\n", global.Workers)
	fmt.Printf("download_limit_mb_s: %s\n", formatFloat(global.DownloadLimitMBps))
	fmt.Printf("proxy_mode: %s\n", global.ProxyMode)
	if len(global.Proxies) == 0 {
		fmt.Println("proxies: (none)")
		return nil
	}
	fmt.Println("proxies:")
	for i, p := range global.Proxies {
		fmt.Printf("  %d. %s\n", i+1, p)
	}
	return nil
}

func runSettingsSet(args []string) error {
	fs := flag.NewFlagSet("settings set", flag.ContinueOnError)
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	workers := fs.Int("workers", -1, "global worker default (>=1, -1 keeps current)")
	downloadLimit := fs.Float64("download-limit-mb-s", -1, "global download limit in MB/s (>=0, 0 disables, -1 keeps current)")
	proxyMode := fs.String("proxy-mode", "", "proxy mode: off|per_worker (empty keeps current)")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	configPath := strings.TrimSpace(*config)
	global, err := discovery.GetGlobalSettings(configPath)
	if err != nil {
		return err
	}

	if *workers != -1 {
		if *workers <= 0 {
			return errors.New("--workers must be >= 1")
		}
		global.Workers = *workers
	}
	if *downloadLimit != -1 {
		if *downloadLimit < 0 {
			return errors.New("--download-limit-mb-s must be >= 0")
		}
		global.DownloadLimitMBps = *downloadLimit
	}
	if strings.TrimSpace(*proxyMode) != "" {
		mode := strings.ToLower(strings.TrimSpace(*proxyMode))
		if mode != discovery.ProxyModeOff && mode != discovery.ProxyModePerWorker {
			return errors.New("--proxy-mode must be off or per_worker")
		}
		global.ProxyMode = mode
	}

	res, err := discovery.UpdateGlobalSettings(discovery.UpdateGlobalSettingsOptions{
		ConfigPath: configPath,
		Global:     global,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(res)
	}

	fmt.Printf("updated global settings in %s\n", res.ConfigPath)
	fmt.Printf("workers: %d\n", res.Global.Workers)
	fmt.Printf("download_limit_mb_s: %s\n", formatFloat(res.Global.DownloadLimitMBps))
	fmt.Printf("proxy_mode: %s\n", res.Global.ProxyMode)
	fmt.Printf("proxies: %d\n", len(res.Global.Proxies))
	return nil
}

func runSettingsProxy(args []string) error {
	if len(args) == 0 {
		printSettingsProxyUsage()
		return nil
	}
	switch args[0] {
	case "list":
		return runSettingsProxyList(args[1:])
	case "add":
		return runSettingsProxyAdd(args[1:])
	case "remove":
		return runSettingsProxyRemove(args[1:])
	case "help", "-h", "--help":
		printSettingsProxyUsage()
		return nil
	default:
		printSettingsProxyUsage()
		return fmt.Errorf("unknown settings proxy subcommand %q", args[0])
	}
}

func runSettingsProxyList(args []string) error {
	fs := flag.NewFlagSet("settings proxy list", flag.ContinueOnError)
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	global, err := discovery.GetGlobalSettings(strings.TrimSpace(*config))
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]any{
			"config_path": strings.TrimSpace(*config),
			"proxy_mode":  global.ProxyMode,
			"proxies":     global.Proxies,
		})
	}
	if len(global.Proxies) == 0 {
		fmt.Println("no proxies configured")
		return nil
	}
	for i, p := range global.Proxies {
		fmt.Printf("%d. %s\n", i+1, p)
	}
	return nil
}

func runSettingsProxyAdd(args []string) error {
	fs := flag.NewFlagSet("settings proxy add", flag.ContinueOnError)
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	value := fs.String("value", "", "proxy URL to add")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*value) == "" {
		return errors.New("--value is required")
	}

	configPath := strings.TrimSpace(*config)
	global, err := discovery.GetGlobalSettings(configPath)
	if err != nil {
		return err
	}
	global.Proxies = append(global.Proxies, strings.TrimSpace(*value))

	res, err := discovery.UpdateGlobalSettings(discovery.UpdateGlobalSettingsOptions{
		ConfigPath: configPath,
		Global:     global,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(res)
	}
	fmt.Printf("proxy added. total proxies: %d\n", len(res.Global.Proxies))
	return nil
}

func runSettingsProxyRemove(args []string) error {
	fs := flag.NewFlagSet("settings proxy remove", flag.ContinueOnError)
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	value := fs.String("value", "", "proxy URL to remove")
	index := fs.Int("index", 0, "1-based proxy index to remove")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*value) == "" && *index <= 0 {
		return errors.New("set --value or --index")
	}

	configPath := strings.TrimSpace(*config)
	global, err := discovery.GetGlobalSettings(configPath)
	if err != nil {
		return err
	}

	next := make([]string, 0, len(global.Proxies))
	removed := false
	if strings.TrimSpace(*value) != "" {
		target := strings.TrimSpace(*value)
		for _, p := range global.Proxies {
			if !removed && p == target {
				removed = true
				continue
			}
			next = append(next, p)
		}
	} else {
		targetIdx := *index - 1
		if targetIdx < 0 || targetIdx >= len(global.Proxies) {
			return fmt.Errorf("--index out of range (1..%d)", len(global.Proxies))
		}
		for i, p := range global.Proxies {
			if i == targetIdx {
				removed = true
				continue
			}
			next = append(next, p)
		}
	}
	if !removed {
		return errors.New("proxy not found")
	}

	global.Proxies = next
	res, err := discovery.UpdateGlobalSettings(discovery.UpdateGlobalSettingsOptions{
		ConfigPath: configPath,
		Global:     global,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(res)
	}
	fmt.Printf("proxy removed. total proxies: %d\n", len(res.Global.Proxies))
	return nil
}

func printSettingsUsage() {
	fmt.Println("settings commands:")
	fmt.Println("  settings show")
	fmt.Println("  settings set [--workers N] [--download-limit-mb-s N] [--proxy-mode off|per_worker]")
	fmt.Println("  settings proxy list")
	fmt.Println("  settings proxy add --value <proxy-url>")
	fmt.Println("  settings proxy remove --value <proxy-url> | --index <n>")
}

func printSettingsProxyUsage() {
	fmt.Println("settings proxy commands:")
	fmt.Println("  settings proxy list")
	fmt.Println("  settings proxy add --value <proxy-url>")
	fmt.Println("  settings proxy remove --value <proxy-url> | --index <n>")
}

func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
