package archive

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type multiDashboard struct {
	mu sync.Mutex

	workers map[int]*liveProgress
	events  []string

	completed int
	target    int
	pending   int
	failR     int
	failP     int
	workersN  int
	sizeDone  int64
	sizeTotal int64

	stop chan struct{}
}

func newMultiDashboard(workers int) *multiDashboard {
	return &multiDashboard{
		workers:  make(map[int]*liveProgress),
		events:   make([]string, 0, 8),
		workersN: workers,
		stop:     make(chan struct{}),
	}
}

func (d *multiDashboard) Start() {
	go func() {
		t := time.NewTicker(700 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-d.stop:
				return
			case <-t.C:
				d.render()
			}
		}
	}()
}

func (d *multiDashboard) Stop() {
	close(d.stop)
	d.render()
}

func (d *multiDashboard) SetTotals(completed, target, pending, failR, failP int) {
	d.mu.Lock()
	d.completed = completed
	d.target = target
	d.pending = pending
	d.failR = failR
	d.failP = failP
	d.mu.Unlock()
}

func (d *multiDashboard) SetSizeEstimate(doneBytes, totalBytes int64) {
	d.mu.Lock()
	d.sizeDone = doneBytes
	d.sizeTotal = totalBytes
	d.mu.Unlock()
}

func (d *multiDashboard) SetWorker(workerID int, p *liveProgress) {
	d.mu.Lock()
	d.workers[workerID] = p
	d.mu.Unlock()
}

func (d *multiDashboard) RemoveWorker(workerID int, event string) {
	d.mu.Lock()
	delete(d.workers, workerID)
	if strings.TrimSpace(event) != "" {
		d.events = append([]string{event}, d.events...)
		if len(d.events) > 8 {
			d.events = d.events[:8]
		}
	}
	d.mu.Unlock()
}

func (d *multiDashboard) render() {
	d.mu.Lock()
	defer d.mu.Unlock()

	ids := make([]int, 0, len(d.workers))
	for id := range d.workers {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	totalMbp := 0.0
	for _, id := range ids {
		totalMbp += d.workers[id].rateMbp
	}
	totalMBps := totalMbp / 8.0

	var b strings.Builder
	b.WriteString("\033[H\033[2J")
	sizePart := ""
	etaPart := ""
	if d.sizeTotal > 0 {
		if eta := estimateTotalETA(d.sizeTotal, d.sizeDone, totalMbp); eta != "" {
			etaPart = fmt.Sprintf(" | eta ~ %s", eta)
		} else {
			etaPart = " | eta ~ calculating"
		}
		sizePart = fmt.Sprintf(" | size ~ %s/%s", formatBytesIEC(d.sizeDone), formatBytesIEC(d.sizeTotal))
	}

	b.WriteString(fmt.Sprintf("yt-vod-manager live | active %d/%d | downloaded %d/%d | pending %d | fail r:%d p:%d | total %.2f MB/s%s%s\n",
		len(ids), d.workersN, d.completed, d.target, d.pending, d.failR, d.failP, totalMBps, etaPart, sizePart))
	b.WriteString(strings.Repeat("-", 120) + "\n")

	if len(ids) == 0 {
		b.WriteString("(no active workers)\n")
	} else {
		for _, id := range ids {
			b.WriteString(fmt.Sprintf("w%d %s\n", id, d.workers[id].render()))
		}
	}

	if len(d.events) > 0 {
		b.WriteString(strings.Repeat("-", 120) + "\n")
		for _, e := range d.events {
			b.WriteString(e + "\n")
		}
	}

	fmt.Print(b.String())
}

func estimateTotalETA(totalBytes, doneBytes int64, totalMbp float64) string {
	if totalBytes <= 0 || totalMbp <= 0 {
		return ""
	}
	remainingBytes := totalBytes - doneBytes
	if remainingBytes <= 0 {
		return "0m"
	}
	remainingSeconds := (float64(remainingBytes) * 8.0) / (totalMbp * 1_000_000.0)
	if remainingSeconds <= 0 {
		return ""
	}
	return formatETASeconds(remainingSeconds)
}

func formatETASeconds(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	secs := int64(math.Round(seconds))
	if secs < 60 {
		return "<1m"
	}
	minutes := secs / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	remMinutes := minutes % 60
	if hours < 24 {
		if remMinutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, remMinutes)
	}
	days := hours / 24
	remHours := hours % 24
	if remHours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, remHours)
}
