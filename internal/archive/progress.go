package archive

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"yt-vod-manager/internal/ytdlp"
)

var (
	rePct   = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)%`)
	reSpeed = regexp.MustCompile(`\bat\s+([^\s]+)`) // yt-dlp [download] ... at X
	reETA   = regexp.MustCompile(`\bETA\s+([0-9:]+)`)
	reOf    = regexp.MustCompile(`\bof\s+([^\s]+)`)
	reFF    = regexp.MustCompile(`\bspeed=\s*([^\s]+)`) // ffmpeg speed=x
	reFFBr  = regexp.MustCompile(`\bbitrate=\s*([0-9.]+)\s*([kKmMgG])bits/s`)
	reRes   = regexp.MustCompile(`\b([0-9]{3,5})x([0-9]{3,5})\b`)
)

type liveProgress struct {
	enabled bool

	index   int
	total   int
	done    int
	target  int
	failR   int
	failP   int
	videoID string
	title   string

	mu      sync.Mutex
	phase   string
	quality string
	pct     string
	speed   string
	rate    string
	rateMbp float64
	eta     string
	totalSz string
	last    string

	stop chan struct{}
}

func newLiveProgress(enabled bool, index, total, done, target, failR, failP int, videoID, title string) *liveProgress {
	return &liveProgress{
		enabled: enabled,
		index:   index,
		total:   total,
		done:    done,
		target:  target,
		failR:   failR,
		failP:   failP,
		videoID: videoID,
		title:   title,
		phase:   "starting",
		stop:    make(chan struct{}),
	}
}

func (p *liveProgress) Start() {
	if !p.enabled {
		return
	}
	go func() {
		t := time.NewTicker(700 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-t.C:
				fmt.Printf("\r\033[2K%s", p.render())
			}
		}
	}()
}

func (p *liveProgress) Stop(final string) {
	if !p.enabled {
		return
	}
	close(p.stop)
	fmt.Printf("\r\033[2K%s\n", final)
}

func (p *liveProgress) SetPhase(phase string) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	p.phase = phase
	p.mu.Unlock()
}

func (p *liveProgress) Handle(stream ytdlp.OutputStream, line string) {
	if !p.enabled {
		return
	}
	l := strings.TrimSpace(line)
	if l == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.last = l
	if strings.HasPrefix(l, "[youtube]") {
		p.phase = "metadata"
	}
	if strings.HasPrefix(l, "[info]") {
		if strings.Contains(strings.ToLower(l), "downloading subtitles") {
			p.phase = "subtitles"
		} else {
			p.phase = "preparing"
		}
	}
	if strings.HasPrefix(l, "[download]") {
		p.phase = "downloading"
		if m := rePct.FindStringSubmatch(l); len(m) > 1 {
			p.pct = m[1] + "%"
		}
		if m := reSpeed.FindStringSubmatch(l); len(m) > 1 {
			p.speed = m[1]
			if r, mbp := parseRateToDisplay(m[1]); r != "" {
				p.rate = r
				p.rateMbp = mbp
			}
		}
		if m := reETA.FindStringSubmatch(l); len(m) > 1 {
			p.eta = m[1]
		}
		if m := reOf.FindStringSubmatch(l); len(m) > 1 {
			p.totalSz = m[1]
		}
	}
	if stream == ytdlp.StreamStderr {
		if p.quality == "" {
			if m := reRes.FindStringSubmatch(l); len(m) > 2 {
				if w, errW := strconv.Atoi(m[1]); errW == nil {
					if h, errH := strconv.Atoi(m[2]); errH == nil {
						p.quality = resolutionToQuality(w, h)
					}
				}
			}
		}
		if m := reFFBr.FindStringSubmatch(l); len(m) > 2 {
			p.rate, p.rateMbp = ffmpegBitrateToDisplay(m[1], m[2])
		}
		if m := reFF.FindStringSubmatch(l); len(m) > 1 {
			p.speed = m[1]
			if p.phase == "starting" || p.phase == "metadata" || p.phase == "preparing" {
				p.phase = "downloading"
			}
		}
	}
}

func (p *liveProgress) render() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	title := p.title
	if len(title) > 52 {
		title = title[:52] + "..."
	}

	parts := []string{fmt.Sprintf("[%d/%d] %s", p.index, p.total, p.videoID), p.phase}
	if p.quality != "" {
		parts = append(parts, p.quality)
	}
	if p.target > 0 {
		parts = append(parts, fmt.Sprintf("downloaded %d/%d", p.done, p.target))
	}
	if p.failR > 0 || p.failP > 0 {
		parts = append(parts, fmt.Sprintf("failed r:%d p:%d", p.failR, p.failP))
	}
	if p.pct != "" {
		parts = append(parts, p.pct)
	}
	if p.speed != "" {
		parts = append(parts, p.speed)
	}
	if p.rate != "" {
		parts = append(parts, p.rate)
	}
	if p.eta != "" {
		parts = append(parts, "ETA "+p.eta)
	}
	if p.totalSz != "" {
		parts = append(parts, p.totalSz)
	}
	parts = append(parts, "| "+title)
	return strings.Join(parts, "  ")
}

func parseRateToDisplay(v string) (string, float64) {
	x := strings.TrimSpace(v)
	if x == "" {
		return "", 0
	}
	l := strings.ToLower(x)
	if strings.Contains(l, "ib/s") || strings.Contains(l, "b/s") {
		// yt-dlp unit, keep as-is because it's already throughput.
		return x, parseRateToMbp(x)
	}
	return "", 0
}

func ffmpegBitrateToDisplay(num, unit string) (string, float64) {
	f, err := strconv.ParseFloat(num, 64)
	if err != nil || f <= 0 {
		return "", 0
	}
	switch strings.ToUpper(strings.TrimSpace(unit)) {
	case "K":
		mbps := f / 1000.0
		MBps := mbps / 8.0
		return fmt.Sprintf("%.2f Mb/s (%.2f MB/s)", mbps, MBps), mbps
	case "M":
		mbps := f
		MBps := mbps / 8.0
		return fmt.Sprintf("%.2f Mb/s (%.2f MB/s)", mbps, MBps), mbps
	case "G":
		mbps := f * 1000.0
		MBps := mbps / 8.0
		return fmt.Sprintf("%.2f Mb/s (%.2f MB/s)", mbps, MBps), mbps
	default:
		return "", 0
	}
}

func resolutionToQuality(width, height int) string {
	_ = width
	switch {
	case height >= 4320:
		return "8K"
	case height >= 2160:
		return "4K"
	default:
		return fmt.Sprintf("%dp", height)
	}
}

func parseRateToMbp(s string) float64 {
	x := strings.TrimSpace(strings.ToLower(s))
	// Examples: 12.3MiB/s, 700KiB/s, 5.1MB/s, 40.2KB/s
	re := regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*([kmgt]?i?b)/s$`)
	m := re.FindStringSubmatch(x)
	if len(m) < 3 {
		return 0
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil || val <= 0 {
		return 0
	}
	unit := m[2]
	// Convert to MB/s
	var mbPerSec float64
	switch unit {
	case "kib":
		mbPerSec = val * 1024 / 1000_000
	case "kb":
		mbPerSec = val * 1000 / 1000_000
	case "mib":
		mbPerSec = val * 1024 * 1024 / 1000_000
	case "mb":
		mbPerSec = val
	case "gib":
		mbPerSec = val * 1024 * 1024 * 1024 / 1000_000
	case "gb":
		mbPerSec = val * 1000
	default:
		return 0
	}
	return mbPerSec * 8 // MB/s -> Mb/s
}
