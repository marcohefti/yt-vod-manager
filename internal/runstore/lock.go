package runstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	runLockDirName   = ".run.lock"
	runLockOwnerFile = "owner.json"
)

type RunLock struct {
	lockDir string
}

type runLockOwner struct {
	PID       int    `json:"pid"`
	CreatedAt string `json:"created_at"`
	Hostname  string `json:"hostname,omitempty"`
}

func AcquireRunLock(runDir string) (RunLock, error) {
	target := strings.TrimSpace(runDir)
	if target == "" {
		return RunLock{}, fmt.Errorf("run directory is required")
	}

	lockDir := filepath.Join(target, runLockDirName)
	if err := os.Mkdir(lockDir, 0o755); err != nil {
		if os.IsExist(err) {
			ownerPath := filepath.Join(lockDir, runLockOwnerFile)
			var owner runLockOwner
			if readErr := ReadJSON(ownerPath, &owner); readErr == nil && owner.PID > 0 && owner.CreatedAt != "" {
				return RunLock{}, fmt.Errorf(
					"run directory is locked: %s (pid=%d created_at=%s host=%s)",
					target, owner.PID, owner.CreatedAt, owner.Hostname,
				)
			}
			return RunLock{}, fmt.Errorf("run directory is locked: %s", target)
		}
		return RunLock{}, fmt.Errorf("acquire run lock for %s: %w", target, err)
	}

	owner := runLockOwner{
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Hostname:  hostnameOrUnknown(),
	}
	ownerPath := filepath.Join(lockDir, runLockOwnerFile)
	if err := WriteJSON(ownerPath, owner); err != nil {
		_ = os.Remove(lockDir)
		return RunLock{}, fmt.Errorf("write run lock owner for %s: %w", target, err)
	}

	return RunLock{lockDir: lockDir}, nil
}

func (l RunLock) Release() error {
	if strings.TrimSpace(l.lockDir) == "" {
		return nil
	}
	_ = os.Remove(filepath.Join(l.lockDir, runLockOwnerFile))
	if err := os.Remove(l.lockDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release run lock %s: %w", l.lockDir, err)
	}
	return nil
}

func hostnameOrUnknown() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "unknown"
	}
	return host
}
