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
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPIDFileManager_Create(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	t.Run("creates PID file with correct content", func(t *testing.T) {
		m := NewPIDFileManager(pidPath)
		defer m.Remove()

		if err := m.Create(1234); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		// Verify file exists
		if !m.Exists() {
			t.Error("PID file does not exist after Create()")
		}

		// Verify content
		pid, err := m.Read()
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if pid != 1234 {
			t.Errorf("Read() = %d, want 1234", pid)
		}

		// Verify permissions
		info, err := os.Stat(pidPath)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if mode := info.Mode() & os.ModePerm; mode != 0600 {
			t.Errorf("PID file mode = %04o, want 0600", mode)
		}
	})

	t.Run("returns error if file already exists", func(t *testing.T) {
		pidPath := filepath.Join(tmpDir, "duplicate.pid")
		m1 := NewPIDFileManager(pidPath)
		m2 := NewPIDFileManager(pidPath)

		defer m1.Remove()

		// First creation should succeed
		if err := m1.Create(1234); err != nil {
			t.Fatalf("First Create() error = %v", err)
		}

		// Second creation should fail
		err := m2.Create(5678)
		if !errors.Is(err, ErrPIDFileExists) {
			t.Errorf("Second Create() error = %v, want ErrPIDFileExists", err)
		}
	})

	t.Run("creates parent directory if missing", func(t *testing.T) {
		deepPath := filepath.Join(tmpDir, "nested", "dir", "test.pid")
		m := NewPIDFileManager(deepPath)
		defer m.Remove()

		if err := m.Create(1234); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		// Verify parent directory was created
		parentDir := filepath.Dir(deepPath)
		info, err := os.Stat(parentDir)
		if err != nil {
			t.Fatalf("Parent directory not created: %v", err)
		}

		// Verify parent directory permissions
		if mode := info.Mode() & os.ModePerm; mode != 0700 {
			t.Errorf("Parent directory mode = %04o, want 0700", mode)
		}
	})
}

func TestPIDFileManager_Read(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("reads valid PID", func(t *testing.T) {
		pidPath := filepath.Join(tmpDir, "valid.pid")
		if err := os.WriteFile(pidPath, []byte("9999\n"), 0600); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		m := NewPIDFileManager(pidPath)
		pid, err := m.Read()
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if pid != 9999 {
			t.Errorf("Read() = %d, want 9999", pid)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		pidPath := filepath.Join(tmpDir, "nonexistent.pid")
		m := NewPIDFileManager(pidPath)

		_, err := m.Read()
		if !os.IsNotExist(err) {
			t.Errorf("Read() error = %v, want os.IsNotExist", err)
		}
	})

	t.Run("returns error for invalid PID", func(t *testing.T) {
		tests := []struct {
			name    string
			content string
		}{
			{"non-numeric", "not-a-number\n"},
			{"negative", "-123\n"},
			{"zero", "0\n"},
			{"float", "123.45\n"},
			{"empty", ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pidPath := filepath.Join(tmpDir, tt.name+".pid")
				if err := os.WriteFile(pidPath, []byte(tt.content), 0600); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}

				m := NewPIDFileManager(pidPath)
				_, err := m.Read()
				if !errors.Is(err, ErrInvalidPID) {
					t.Errorf("Read() error = %v, want ErrInvalidPID", err)
				}
			})
		}
	})

	t.Run("handles whitespace", func(t *testing.T) {
		pidPath := filepath.Join(tmpDir, "whitespace.pid")
		if err := os.WriteFile(pidPath, []byte("  1234  \n"), 0600); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		m := NewPIDFileManager(pidPath)
		pid, err := m.Read()
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if pid != 1234 {
			t.Errorf("Read() = %d, want 1234", pid)
		}
	})
}

func TestPIDFileManager_Remove(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("removes PID file and releases lock", func(t *testing.T) {
		pidPath := filepath.Join(tmpDir, "remove.pid")
		m := NewPIDFileManager(pidPath)

		if err := m.Create(1234); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := m.Remove(); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		// Verify file is gone
		if m.Exists() {
			t.Error("PID file still exists after Remove()")
		}

		// Verify we can create a new one (lock was released)
		m2 := NewPIDFileManager(pidPath)
		defer m2.Remove()
		if err := m2.Create(5678); err != nil {
			t.Errorf("Failed to create new PID file after Remove(): %v", err)
		}
	})

	t.Run("succeeds if file already removed", func(t *testing.T) {
		pidPath := filepath.Join(tmpDir, "already-removed.pid")
		m := NewPIDFileManager(pidPath)

		// Remove non-existent file should not error
		if err := m.Remove(); err != nil {
			t.Errorf("Remove() error = %v, want nil", err)
		}
	})
}

func TestPIDFileManager_Locking(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "lock.pid")

	t.Run("prevents concurrent creates", func(t *testing.T) {
		m1 := NewPIDFileManager(pidPath)
		m2 := NewPIDFileManager(pidPath)

		defer m1.Remove()

		// First manager creates and locks
		if err := m1.Create(1111); err != nil {
			t.Fatalf("First Create() error = %v", err)
		}

		// Second manager should fail to acquire lock
		// Note: Since we use O_EXCL, this will fail at file creation, not locking
		err := m2.Create(2222)
		if err == nil {
			t.Error("Second Create() succeeded, want error")
			m2.Remove()
		}
	})
}

func TestPIDFileManager_DirectorySafety(t *testing.T) {
	t.Run("rejects world-writable directory", func(t *testing.T) {
		// This test may behave differently on different platforms
		// On macOS, temp dirs have sticky bit set which provides protection
		// even with 0777 permissions
		tmpDir := t.TempDir()
		unsafeDir := filepath.Join(tmpDir, "unsafe")
		if err := os.Mkdir(unsafeDir, 0777); err != nil {
			t.Fatalf("Failed to create unsafe directory: %v", err)
		}

		// Verify the directory is actually world-writable
		info, err := os.Stat(unsafeDir)
		if err != nil {
			t.Fatalf("Failed to stat unsafe directory: %v", err)
		}

		// Check if world-writable bit is actually set
		if info.Mode()&0002 == 0 {
			t.Skip("Platform doesn't support world-writable directories in this context")
		}

		pidPath := filepath.Join(unsafeDir, "test.pid")
		m := NewPIDFileManager(pidPath)

		err = m.Create(1234)
		if err == nil {
			m.Remove()
			t.Error("Create() in world-writable directory succeeded, want error")
			return
		}

		if !errors.Is(err, ErrUnsafeDirectory) {
			t.Errorf("Create() error = %v, want ErrUnsafeDirectory", err)
		}
	})
}

func TestPIDFileManager_FileLocking(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "flock.pid")

	t.Run("holds exclusive lock while file is open", func(t *testing.T) {
		m := NewPIDFileManager(pidPath)
		defer m.Remove()

		if err := m.Create(1234); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		// Try to acquire lock from another file descriptor
		f, err := os.OpenFile(pidPath, os.O_RDWR, 0600)
		if err != nil {
			t.Fatalf("Failed to open PID file: %v", err)
		}
		defer f.Close()

		// Non-blocking lock attempt should fail
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			t.Error("Acquired lock on already-locked file")
			syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		}
		if err != syscall.EWOULDBLOCK {
			t.Errorf("Flock error = %v, want EWOULDBLOCK", err)
		}
	})

	t.Run("releases lock on Remove", func(t *testing.T) {
		pidPath := filepath.Join(tmpDir, "flock-release.pid")
		m := NewPIDFileManager(pidPath)

		if err := m.Create(1234); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := m.Remove(); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		// Should be able to create new PID file now
		m2 := NewPIDFileManager(pidPath)
		defer m2.Remove()

		if err := m2.Create(5678); err != nil {
			t.Errorf("Second Create() after Remove() error = %v", err)
		}
	})
}
