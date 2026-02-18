package archive

import "strings"

const (
	proxyModeOff       = "off"
	proxyModePerWorker = "per_worker"
)

func normalizeProxyMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", proxyModeOff:
		return proxyModeOff
	case proxyModePerWorker:
		return proxyModePerWorker
	default:
		return proxyModeOff
	}
}

func normalizeProxyList(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, p := range raw {
		v := strings.TrimSpace(p)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func proxyForWorker(workerID int, mode string, proxies []string) string {
	if normalizeProxyMode(mode) != proxyModePerWorker {
		return ""
	}
	if workerID <= 0 || workerID > len(proxies) {
		return ""
	}
	return proxies[workerID-1]
}
