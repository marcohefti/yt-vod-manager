package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"yt-vod-manager/internal/version"
)

const (
	updateCheckEndpoint      = "https://api.github.com/repos/marcohefti/yt-vod-manager/releases/latest"
	updateCheckInterval      = 24 * time.Hour
	updateNotificationWindow = 12 * time.Hour
)

type updateNoticeCache struct {
	LastChecked  string `json:"last_checked,omitempty"`
	LatestTag    string `json:"latest_tag,omitempty"`
	LastNotified string `json:"last_notified,omitempty"`
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

type semverValue struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func maybePrintUpdateHint(args []string) {
	if shouldSkipUpdateHint(args) {
		return
	}

	currentTag := normalizeVersionTag(version.Value)
	if currentTag == "" {
		return
	}

	cachePath, err := updateNoticeCachePath()
	if err != nil {
		return
	}

	cache := loadUpdateNoticeCache(cachePath)
	now := time.Now().UTC()

	latestTag := strings.TrimSpace(cache.LatestTag)
	lastChecked, hasLastChecked := parseRFC3339(cache.LastChecked)
	if latestTag == "" || !hasLastChecked || now.Sub(lastChecked) >= updateCheckInterval {
		freshLatest, fetchErr := fetchLatestReleaseTag()
		if fetchErr == nil && freshLatest != "" {
			latestTag = freshLatest
			cache.LatestTag = latestTag
			cache.LastChecked = now.Format(time.RFC3339)
			saveUpdateNoticeCache(cachePath, cache)
		}
	}
	if latestTag == "" {
		return
	}

	isNewer, cmpErr := isNewerVersion(latestTag, currentTag)
	if cmpErr != nil || !isNewer {
		return
	}

	lastNotified, hasLastNotified := parseRFC3339(cache.LastNotified)
	if hasLastNotified && now.Sub(lastNotified) < updateNotificationWindow {
		return
	}

	fmt.Fprintf(
		os.Stderr,
		"update available: %s (current %s). Run: yt-vod-manager self-update\n",
		latestTag,
		currentTag,
	)
	cache.LastNotified = now.Format(time.RFC3339)
	saveUpdateNoticeCache(cachePath, cache)
}

func shouldSkipUpdateHint(args []string) bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("YTVM_DISABLE_UPDATE_CHECK")), "1") {
		return true
	}
	if len(args) == 0 {
		return true
	}
	if args[0] == "self-update" {
		return true
	}
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "--json" || strings.HasPrefix(trimmed, "--json=") {
			return true
		}
	}
	return false
}

func updateNoticeCachePath() (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheRoot) == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", homeErr
		}
		cacheRoot = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheRoot, "yt-vod-manager", "update-check.json"), nil
}

func loadUpdateNoticeCache(cachePath string) updateNoticeCache {
	var cache updateNoticeCache
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return cache
	}
	_ = json.Unmarshal(data, &cache)
	return cache
}

func saveUpdateNoticeCache(cachePath string, cache updateNoticeCache) {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmpPath, cachePath)
}

func parseRFC3339(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func fetchLatestReleaseTag() (string, error) {
	req, err := http.NewRequest(http.MethodGet, updateCheckEndpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "yt-vod-manager-update-check")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d fetching latest release", resp.StatusCode)
	}

	var payload latestReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return normalizeVersionTag(payload.TagName), nil
}

func normalizeVersionTag(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "v") {
		raw = "v" + raw
	}
	return raw
}

func isNewerVersion(candidate, current string) (bool, error) {
	a, err := parseSemver(candidate)
	if err != nil {
		return false, err
	}
	b, err := parseSemver(current)
	if err != nil {
		return false, err
	}
	return compareSemver(a, b) > 0, nil
}

func parseSemver(raw string) (semverValue, error) {
	tag := strings.TrimSpace(raw)
	tag = strings.TrimPrefix(tag, "v")
	if tag == "" {
		return semverValue{}, fmt.Errorf("invalid version %q", raw)
	}

	core := tag
	pre := ""
	if idx := strings.Index(tag, "-"); idx >= 0 {
		core = tag[:idx]
		pre = tag[idx+1:]
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semverValue{}, fmt.Errorf("invalid semver core %q", raw)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semverValue{}, err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semverValue{}, err
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return semverValue{}, err
	}

	return semverValue{
		major:      major,
		minor:      minor,
		patch:      patch,
		prerelease: pre,
	}, nil
}

func compareSemver(a, b semverValue) int {
	if a.major != b.major {
		return cmpInt(a.major, b.major)
	}
	if a.minor != b.minor {
		return cmpInt(a.minor, b.minor)
	}
	if a.patch != b.patch {
		return cmpInt(a.patch, b.patch)
	}

	aPre := strings.TrimSpace(a.prerelease)
	bPre := strings.TrimSpace(b.prerelease)
	if aPre == "" && bPre != "" {
		return 1
	}
	if aPre != "" && bPre == "" {
		return -1
	}
	if aPre == bPre {
		return 0
	}
	if aPre > bPre {
		return 1
	}
	return -1
}

func cmpInt(a, b int) int {
	if a > b {
		return 1
	}
	if a < b {
		return -1
	}
	return 0
}
