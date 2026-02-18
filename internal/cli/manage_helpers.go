package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"yt-vod-manager/internal/discovery"
)

func parseBool(raw string) (bool, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "y", "yes", "true", "1":
		return true, true
	case "n", "no", "false", "0", "":
		return false, true
	default:
		return false, false
	}
}

func boolToYN(v bool) string {
	if v {
		return "y"
	}
	return "n"
}

func kv(k, v string) string {
	return fmt.Sprintf("%s: %s", k, v)
}

func listWindow(total, cursor, maxRows int) (int, int) {
	if total <= maxRows {
		return 0, total
	}
	half := maxRows / 2
	start := cursor - half
	if start < 0 {
		start = 0
	}
	end := start + maxRows
	if end > total {
		end = total
		start = end - maxRows
	}
	return start, end
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

func wrapOrTrim(s string, width int) string {
	if width <= 0 {
		return s
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return truncateRunes(s, width)
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func defaultIfEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func formatIntDefault(v int) string {
	if v <= 0 {
		return "(runtime default)"
	}
	return strconv.Itoa(v)
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func normalizeSubtitleChoice(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", "english", "en", "en.*,en,-live_chat":
		return "english"
	case "all", "all,-live_chat":
		return "all"
	default:
		return "english"
	}
}

func normalizeSubtitleValue(raw string) string {
	switch normalizeSubtitleChoice(raw) {
	case "all":
		return "all"
	default:
		return discovery.DefaultSubtitleLanguage
	}
}

func isProjectActive(p discovery.Project) bool {
	if p.Active == nil {
		return true
	}
	return *p.Active
}
