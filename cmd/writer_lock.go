package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const writerLockFile = "writer.lock"

// writerLockMetadata describes the active writer lock owner.
type writerLockMetadata struct {
	PID       int    `json:"pid"`
	Command   string `json:"command"`
	StartedAt int64  `json:"started_at_unix"`
}

// acquireWriterLock acquires the single-writer lock for index/update/serve.
func acquireWriterLock(root, command string) (func(), error) {
	tlDir := filepath.Join(root, ".treelines")
	if err := os.MkdirAll(tlDir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure .treelines directory: %w", err)
	}

	lockPath := filepath.Join(tlDir, writerLockFile)
	release, err := tryCreateWriterLock(lockPath, command)
	if err == nil {
		return release, nil
	}

	if !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	meta, readErr := readWriterLockMetadata(lockPath)
	if readErr == nil && meta.PID > 0 && !processExists(meta.PID) {
		if rmErr := os.Remove(lockPath); rmErr == nil || errors.Is(rmErr, os.ErrNotExist) {
			return tryCreateWriterLock(lockPath, command)
		}
	}

	holder := "another writer command"
	if readErr == nil {
		holder = fmt.Sprintf("%s (pid=%d)", meta.Command, meta.PID)
	}
	return nil, fmt.Errorf("%s is active; try again in 10 seconds", holder)
}

// tryCreateWriterLock creates the lock file atomically if it does not exist.
func tryCreateWriterLock(lockPath, command string) (func(), error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	meta := writerLockMetadata{
		PID:       os.Getpid(),
		Command:   command,
		StartedAt: time.Now().Unix(),
	}
	data, err := json.Marshal(meta)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(lockPath)
		return nil, fmt.Errorf("marshal writer lock metadata: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(lockPath)
		return nil, fmt.Errorf("write writer lock metadata: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(lockPath)
		return nil, fmt.Errorf("close writer lock metadata file: %w", err)
	}

	released := false
	release := func() {
		if released {
			return
		}
		released = true
		_ = os.Remove(lockPath)
	}
	return release, nil
}

// readWriterLockMetadata loads lock metadata from the lock file.
func readWriterLockMetadata(lockPath string) (writerLockMetadata, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return writerLockMetadata{}, err
	}
	var meta writerLockMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return writerLockMetadata{}, err
	}
	return meta, nil
}

// processExists reports whether a process id is currently alive.
func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}
