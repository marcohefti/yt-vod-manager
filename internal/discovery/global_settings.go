package discovery

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	ProxyModeOff       = "off"
	ProxyModePerWorker = "per_worker"
)

type GlobalSettings struct {
	Workers           int      `json:"workers,omitempty"`
	DownloadLimitMBps float64  `json:"download_limit_mb_s,omitempty"`
	ProxyMode         string   `json:"proxy_mode,omitempty"`
	Proxies           []string `json:"proxies,omitempty"`
}

type RuntimeNetworkSettings struct {
	Workers           int
	DownloadLimitMBps float64
	ProxyMode         string
	Proxies           []string
}

type UpdateGlobalSettingsOptions struct {
	ConfigPath string
	Global     GlobalSettings
}

type UpdateGlobalSettingsResult struct {
	ConfigPath string         `json:"config_path"`
	Global     GlobalSettings `json:"global"`
}

func defaultGlobalSettings() GlobalSettings {
	return GlobalSettings{
		Workers:           DefaultWorkers,
		DownloadLimitMBps: DefaultDownloadLimitMBps,
		ProxyMode:         DefaultProxyMode,
		Proxies:           []string{},
	}
}

func normalizeGlobalSettings(raw GlobalSettings) GlobalSettings {
	norm := raw
	if norm.Workers <= 0 {
		norm.Workers = DefaultWorkers
	}
	if norm.DownloadLimitMBps < 0 {
		norm.DownloadLimitMBps = DefaultDownloadLimitMBps
	}
	norm.ProxyMode = normalizeProxyMode(norm.ProxyMode)
	norm.Proxies = normalizeProxyList(norm.Proxies)
	return norm
}

func normalizeProxyMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", ProxyModeOff:
		return ProxyModeOff
	case ProxyModePerWorker:
		return ProxyModePerWorker
	default:
		return ProxyModeOff
	}
}

func normalizeProxyList(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, p := range raw {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func ReadGlobalSettings(configPath string) (GlobalSettings, error) {
	path := normalizeConfigPath(configPath)
	reg, err := loadProjectRegistry(path)
	if err == nil {
		return reg.Global, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return defaultGlobalSettings(), nil
	}
	return GlobalSettings{}, err
}

func GetGlobalSettings(configPath string) (GlobalSettings, error) {
	reg, _, err := EnsureProjectRegistry(configPath)
	if err != nil {
		return GlobalSettings{}, err
	}
	return reg.Global, nil
}

func UpdateGlobalSettings(opts UpdateGlobalSettingsOptions) (UpdateGlobalSettingsResult, error) {
	configPath := normalizeConfigPath(opts.ConfigPath)
	reg, _, err := EnsureProjectRegistry(configPath)
	if err != nil {
		return UpdateGlobalSettingsResult{}, err
	}
	reg.Global = normalizeGlobalSettings(opts.Global)
	reg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveProjectRegistry(configPath, reg); err != nil {
		return UpdateGlobalSettingsResult{}, err
	}
	return UpdateGlobalSettingsResult{
		ConfigPath: configPath,
		Global:     reg.Global,
	}, nil
}

func ResolveRuntimeNetworkSettings(project Project, global GlobalSettings, workersOverride int, downloadLimitOverride *float64) (RuntimeNetworkSettings, error) {
	if workersOverride < 0 {
		return RuntimeNetworkSettings{}, fmt.Errorf("workers must be >= 0")
	}

	normGlobal := normalizeGlobalSettings(global)
	workers := firstPositive(workersOverride, project.Workers, normGlobal.Workers, DefaultWorkers)
	if workers <= 0 {
		workers = DefaultWorkers
	}

	limit := normGlobal.DownloadLimitMBps
	if downloadLimitOverride != nil {
		if *downloadLimitOverride < 0 {
			return RuntimeNetworkSettings{}, fmt.Errorf("download limit must be >= 0 MB/s")
		}
		limit = *downloadLimitOverride
	}

	mode := normalizeProxyMode(normGlobal.ProxyMode)
	proxies := normalizeProxyList(normGlobal.Proxies)
	if mode == ProxyModePerWorker {
		if len(proxies) == 0 {
			return RuntimeNetworkSettings{}, fmt.Errorf("proxy mode %q requires at least one proxy", ProxyModePerWorker)
		}
		if workers > len(proxies) {
			return RuntimeNetworkSettings{}, fmt.Errorf("proxy mode %q requires at least %d proxies for %d workers", ProxyModePerWorker, workers, workers)
		}
	}

	return RuntimeNetworkSettings{
		Workers:           workers,
		DownloadLimitMBps: limit,
		ProxyMode:         mode,
		Proxies:           proxies,
	}, nil
}

func firstPositive(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
