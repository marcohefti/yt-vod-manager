package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"yt-vod-manager/internal/discovery"
)

func newManageGlobalForm(global discovery.GlobalSettings, width int) *manageForm {
	f := &manageForm{
		Kind:  manageFormKindGlobal,
		Title: "Global Settings",
		Fields: []manageFormField{
			{Key: "workers", Label: "Workers", Help: "Default workers when project override is 0", Kind: manageFieldInt, Value: strconv.Itoa(global.Workers)},
			{Key: "download_limit_mb_s", Label: "Download Limit MB/s", Help: "0 disables global rate limit", Kind: manageFieldString, Value: formatFloat(global.DownloadLimitMBps)},
			{Key: "proxy_mode", Label: "Proxy Mode", Help: "off or per_worker", Kind: manageFieldSelect, Value: defaultIfEmpty(global.ProxyMode, discovery.ProxyModeOff), Options: []string{discovery.ProxyModeOff, discovery.ProxyModePerWorker}},
			{Key: "proxies", Label: "Proxies", Help: "Comma-separated list. One proxy per worker when mode=per_worker.", Kind: manageFieldString, Value: strings.Join(global.Proxies, ", ")},
		},
	}

	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 2048
	input.Width = clampInt(width-8, 20, 120)
	f.Input = input
	f.loadFieldIntoInput()
	f.Input.Focus()
	return f
}

func (f *manageForm) toGlobalSettings() (discovery.GlobalSettings, error) {
	if f == nil {
		return discovery.GlobalSettings{}, fmt.Errorf("internal form error")
	}
	vals := make(map[string]string, len(f.Fields))
	for _, field := range f.Fields {
		v := strings.TrimSpace(field.Value)
		switch field.Kind {
		case manageFieldInt:
			if v == "" {
				v = "0"
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return discovery.GlobalSettings{}, fmt.Errorf("%s must be an integer >= 0", strings.ToLower(field.Label))
			}
		case manageFieldSelect:
			matched := false
			for _, opt := range field.Options {
				if strings.EqualFold(opt, v) {
					v = opt
					matched = true
					break
				}
			}
			if !matched {
				return discovery.GlobalSettings{}, fmt.Errorf("%s has invalid value", strings.ToLower(field.Label))
			}
		}
		vals[field.Key] = v
	}

	workers, _ := strconv.Atoi(defaultIfEmpty(vals["workers"], "0"))
	if workers <= 0 {
		return discovery.GlobalSettings{}, fmt.Errorf("workers must be >= 1")
	}
	downloadLimit, err := strconv.ParseFloat(defaultIfEmpty(vals["download_limit_mb_s"], "0"), 64)
	if err != nil || downloadLimit < 0 {
		return discovery.GlobalSettings{}, fmt.Errorf("download limit mb/s must be a number >= 0")
	}

	mode := strings.ToLower(strings.TrimSpace(vals["proxy_mode"]))
	proxies := parseProxyValueList(vals["proxies"])
	if mode == discovery.ProxyModePerWorker && len(proxies) == 0 {
		return discovery.GlobalSettings{}, fmt.Errorf("proxy mode per_worker requires at least one proxy")
	}

	return discovery.GlobalSettings{
		Workers:           workers,
		DownloadLimitMBps: downloadLimit,
		ProxyMode:         mode,
		Proxies:           proxies,
	}, nil
}

func saveGlobalSettingsCmd(configPath string, global discovery.GlobalSettings) tea.Cmd {
	return func() tea.Msg {
		res, err := discovery.UpdateGlobalSettings(discovery.UpdateGlobalSettingsOptions{
			ConfigPath: configPath,
			Global:     global,
		})
		if err != nil {
			return manageSaveMsg{err: err}
		}
		return manageSaveMsg{
			message: fmt.Sprintf(
				"updated global settings: workers=%d limit=%sMB/s proxy_mode=%s proxies=%d",
				res.Global.Workers,
				formatFloat(res.Global.DownloadLimitMBps),
				res.Global.ProxyMode,
				len(res.Global.Proxies),
			),
		}
	}
}

func parseProxyValueList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
