package triggers

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
)

const lockTimeout = 5 * time.Second

// FileLock represents a file lock.
type FileLock struct {
	file *os.File
}

// AcquireLock acquires an exclusive lock on the config file.
// Returns an error if the lock cannot be acquired within the timeout.
func AcquireLock(ctx context.Context, path string) (*FileLock, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}

	lockCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
	}()

	select {
	case err := <-done:
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
		return &FileLock{file: file}, nil
	case <-lockCtx.Done():
		file.Close()
		return nil, fmt.Errorf("config file locked by another process (timeout after %v)", lockTimeout)
	}
}

// Release releases the file lock.
func (l *FileLock) Release() error {
	if l.file == nil {
		return nil
	}
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		l.file.Close()
		return fmt.Errorf("failed to release lock: %w", err)
	}
	return l.file.Close()
}
