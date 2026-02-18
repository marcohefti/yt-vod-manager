package archive

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"yt-vod-manager/internal/model"
)

type runSizeEstimator struct {
	bytesByVideoID map[string]int64
	totalBytes     int64
}

type rawManifestEstimate struct {
	Entries []rawManifestEstimateEntry `json:"entries"`
}

type rawManifestEstimateEntry struct {
	ID             string   `json:"id"`
	Filesize       *float64 `json:"filesize"`
	FilesizeApprox *float64 `json:"filesize_approx"`
	Duration       *float64 `json:"duration"`
}

func loadRunSizeEstimator(runDir string, mf model.JobsManifest, quality string) runSizeEstimator {
	rawPath := filepath.Join(runDir, "manifest.raw.json")
	raw, err := os.ReadFile(rawPath)
	if err != nil || len(raw) == 0 {
		return runSizeEstimator{}
	}

	var src rawManifestEstimate
	if err := json.Unmarshal(raw, &src); err != nil {
		return runSizeEstimator{}
	}

	byID := make(map[string]int64, len(src.Entries))
	for _, entry := range src.Entries {
		videoID := strings.TrimSpace(entry.ID)
		if videoID == "" {
			continue
		}
		sizeBytes := firstPositive(entry.Filesize, entry.FilesizeApprox)
		if sizeBytes <= 0 && entry.Duration != nil && *entry.Duration > 0 {
			sizeBytes = estimateBytesFromDuration(*entry.Duration, quality)
		}
		if sizeBytes > 0 {
			byID[videoID] = sizeBytes
		}
	}

	var total int64
	for _, job := range mf.Jobs {
		if job.Status == model.StatusSkippedPrivate {
			continue
		}
		total += byID[strings.TrimSpace(job.VideoID)]
	}
	if total <= 0 {
		return runSizeEstimator{}
	}
	return runSizeEstimator{
		bytesByVideoID: byID,
		totalBytes:     total,
	}
}

func (e runSizeEstimator) hasEstimate() bool {
	return e.totalBytes > 0
}

func (e runSizeEstimator) completedBytes(jobs []model.Job) int64 {
	if e.totalBytes <= 0 {
		return 0
	}
	var done int64
	for _, job := range jobs {
		if job.Status != model.StatusCompleted {
			continue
		}
		done += e.bytesByVideoID[strings.TrimSpace(job.VideoID)]
	}
	if done < 0 {
		return 0
	}
	if done > e.totalBytes {
		return e.totalBytes
	}
	return done
}

func estimateBytesFromDuration(durationSec float64, quality string) int64 {
	if durationSec <= 0 {
		return 0
	}
	mbps := estimateMbpsForQuality(quality)
	if mbps <= 0 {
		return 0
	}
	bits := durationSec * mbps * 1_000_000
	bytes := bits / 8.0
	return int64(math.Round(bytes))
}

func estimateMbpsForQuality(rawQuality string) float64 {
	q := strings.ToLower(strings.TrimSpace(rawQuality))
	switch q {
	case "720p", "720", "sd", "small":
		return 3.5
	case "1080p", "1080", "hd":
		return 6.0
	default:
		return 8.0
	}
}

func firstPositive(values ...*float64) int64 {
	for _, v := range values {
		if v == nil || *v <= 0 {
			continue
		}
		return int64(math.Round(*v))
	}
	return 0
}

func formatBytesIEC(n int64) string {
	if n <= 0 {
		return "0 B"
	}
	const unit = 1024
	if n < unit {
		return fmtInt(n) + " B"
	}
	div, exp := int64(unit), 0
	for q := n / unit; q >= unit; q /= unit {
		div *= unit
		exp++
	}
	value := float64(n) / float64(div)
	suffix := "KMGTPE"[exp]
	return fmtFloat1(value) + " " + string(suffix) + "iB"
}

func fmtInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func fmtFloat1(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}
