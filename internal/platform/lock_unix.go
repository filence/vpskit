//go:build unix

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type ProcessLock struct {
	file *os.File
}

func AcquireProcessLock(path string) (*ProcessLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open process lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("another VPSKit process holds the lock: %w", err)
	}
	return &ProcessLock{file: file}, nil
}

func (lock *ProcessLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	unlockError := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	closeError := lock.file.Close()
	if unlockError != nil {
		return unlockError
	}
	return closeError
}
