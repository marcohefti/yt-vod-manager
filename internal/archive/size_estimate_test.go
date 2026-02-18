package archive

import (
	"path/filepath"
	"testing"

	"yt-vod-manager/internal/model"
	"yt-vod-manager/internal/runstore"
)

func TestLoadRunSizeEstimator(t *testing.T) {
	runDir := t.TempDir()
	raw := `{
  "entries": [
    {"id":"a","filesize":1000},
    {"id":"b","filesize_approx":2000},
    {"id":"c","duration":10},
    {"id":"d"},
    {"id":"e","filesize":9999}
  ]
}`
	if err := runstore.WriteBytes(filepath.Join(runDir, "manifest.raw.json"), []byte(raw)); err != nil {
		t.Fatal(err)
	}

	mf := model.JobsManifest{
		Jobs: []model.Job{
			{VideoID: "a", Status: model.StatusPending},
			{VideoID: "b", Status: model.StatusCompleted},
			{VideoID: "c", Status: model.StatusCompleted},
			{VideoID: "d", Status: model.StatusPending},
			{VideoID: "e", Status: model.StatusSkippedPrivate},
		},
	}

	est := loadRunSizeEstimator(runDir, mf, "720p")
	if !est.hasEstimate() {
		t.Fatal("expected size estimate")
	}
	const wantTotal int64 = 4378000 // a(1000) + b(2000) + c(10s * 3.5 Mb/s)
	if est.totalBytes != wantTotal {
		t.Fatalf("totalBytes = %d, want %d", est.totalBytes, wantTotal)
	}
	const wantDone int64 = 4377000 // b + c
	gotDone := est.completedBytes(mf.Jobs)
	if gotDone != wantDone {
		t.Fatalf("completedBytes = %d, want %d", gotDone, wantDone)
	}
}

func TestEstimateMbpsForQuality(t *testing.T) {
	if got := estimateMbpsForQuality("720p"); got != 3.5 {
		t.Fatalf("720p mbps = %v, want 3.5", got)
	}
	if got := estimateMbpsForQuality("1080p"); got != 6.0 {
		t.Fatalf("1080p mbps = %v, want 6.0", got)
	}
	if got := estimateMbpsForQuality("best"); got != 8.0 {
		t.Fatalf("best mbps = %v, want 8.0", got)
	}
}
