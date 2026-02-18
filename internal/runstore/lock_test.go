package runstore

import "testing"

func TestAcquireRunLock_BlocksConcurrentAcquire(t *testing.T) {
	runDir := t.TempDir()

	lock, err := AcquireRunLock(runDir)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	defer func() {
		_ = lock.Release()
	}()

	if _, err := AcquireRunLock(runDir); err == nil {
		t.Fatalf("expected second acquire to fail")
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("release lock: %v", err)
	}

	lock2, err := AcquireRunLock(runDir)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := lock2.Release(); err != nil {
		t.Fatalf("release second lock: %v", err)
	}
}
