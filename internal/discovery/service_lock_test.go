package discovery

import (
	"path/filepath"
	"strings"
	"testing"

	"yt-vod-manager/internal/runstore"
)

func TestRefresh_FailsWhenRunDirectoryIsLocked(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "run")
	if err := runstore.Mkdir(runDir); err != nil {
		t.Fatal(err)
	}

	lock, err := runstore.AcquireRunLock(runDir)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	defer func() {
		_ = lock.Release()
	}()

	_, err = Refresh(RefreshOptions{
		RunDir: runDir,
	})
	if err == nil {
		t.Fatalf("expected lock error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "locked") {
		t.Fatalf("expected locked error, got %v", err)
	}
}
