// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var (
	// ErrPIDFileExists is returned when trying to create a PID file that already exists.
	ErrPIDFileExists = errors.New("PID file already exists")

	// ErrPIDFileLocked is returned when another process holds the PID file lock.
	ErrPIDFileLocked = errors.New("PID file is locked by another process")

	// ErrInvalidPID is returned when the PID file contains invalid data.
	ErrInvalidPID = errors.New("invalid PID in file")

	// ErrUnsafeDirectory is returned when the PID file parent is world-writable.
	ErrUnsafeDirectory = errors.New("PID file directory is world-writable")
)

// PIDFileManager manages secure PID file operations.
// It uses exclusive file locking (flock) and atomic creation (O_EXCL)
// to prevent race conditions and symlink attacks.
type PIDFileManager struct {
	path     string
	lockFile *os.File
}

// NewPIDFileManager creates a new PID file manager for the given path.
func NewPIDFileManager(path string) *PIDFileManager {
	return &PIDFileManager{
		path: path,
	}
}

// Create writes the given PID to the file with exclusive locking.
// It creates the parent directory if needed and sets restrictive permissions.
// Returns ErrPIDFileExists if the file already exists and is locked.
func (m *PIDFileManager) Create(pid int) error {
	// Verify parent directory is safe
	parentDir := filepath.Dir(m.path)
	if err := m.verifyDirectorySafety(parentDir); err != nil {
		return fmt.Errorf("unsafe PID file location: %w", err)
	}

	// Create parent directory if needed with restrictive permissions
	if err := os.MkdirAll(parentDir, 0700); err != nil {
		return fmt.Errorf("failed to create PID file directory: %w", err)
	}

	// Open file with O_EXCL to prevent symlink attacks and race conditions
	// O_RDWR is needed for flock
	f, err := os.OpenFile(m.path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return ErrPIDFileExists
		}
		return fmt.Errorf("failed to create PID file: %w", err)
	}

	// Acquire exclusive lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		os.Remove(m.path) // Clean up the file we created
		if err == syscall.EWOULDBLOCK {
			return ErrPIDFileLocked
		}
		return fmt.Errorf("failed to lock PID file: %w", err)
	}

	// Write PID
	if _, err := f.WriteString(fmt.Sprintf("%d\n", pid)); err != nil {
		f.Close()
		os.Remove(m.path)
		return fmt.Errorf("failed to write PID: %w", err)
	}

	// Sync to disk
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(m.path)
		return fmt.Errorf("failed to sync PID file: %w", err)
	}

	// Keep file open to maintain lock
	m.lockFile = f
	return nil
}

// Read reads the PID from the file.
// Returns ErrInvalidPID if the file contains non-numeric data.
func (m *PIDFileManager) Read() (int, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, err
		}
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	// Parse PID (trim whitespace and newlines)
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidPID, pidStr)
	}

	if pid <= 0 {
		return 0, fmt.Errorf("%w: PID must be positive, got %d", ErrInvalidPID, pid)
	}

	return pid, nil
}

// Remove deletes the PID file and releases the lock.
func (m *PIDFileManager) Remove() error {
	// Release lock if held
	if m.lockFile != nil {
		syscall.Flock(int(m.lockFile.Fd()), syscall.LOCK_UN)
		m.lockFile.Close()
		m.lockFile = nil
	}

	// Remove file (ignore errors if already removed)
	if err := os.Remove(m.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}

	return nil
}

// Exists returns true if the PID file exists.
func (m *PIDFileManager) Exists() bool {
	_, err := os.Stat(m.path)
	return err == nil
}

// verifyDirectorySafety checks that the directory is not world-writable.
// This prevents attacks where an attacker creates a symlink in a world-writable
// directory pointing to a file they want us to overwrite.
func (m *PIDFileManager) verifyDirectorySafety(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		// Directory doesn't exist yet - that's fine, we'll create it
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat directory: %w", err)
	}

	// Check if directory is world-writable (other write bit set)
	mode := info.Mode()
	if mode&0002 != 0 {
		return fmt.Errorf("%w: %s has mode %04o", ErrUnsafeDirectory, dir, mode&os.ModePerm)
	}

	return nil
}
