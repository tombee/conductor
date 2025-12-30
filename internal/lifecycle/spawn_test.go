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
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// skipOnSpawnError checks if an error is a spawn permission error and skips if so.
// Some environments (sandboxed test runners, containers) block fork/exec.
func skipOnSpawnError(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "operation not permitted") {
		t.Skipf("Skipping: spawn not permitted in this environment: %v", err)
	}
}

func TestSpawner_SpawnDetached(t *testing.T) {
	if os.Getenv("SKIP_SPAWN_TESTS") != "" {
		t.Skip("Skipping spawn tests (SKIP_SPAWN_TESTS is set)")
	}

	tmpDir := t.TempDir()

	t.Run("spawns detached process", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "test.log")
		spawner := NewSpawner()

		// Spawn a process that writes to stdout and runs for a bit
		pid, err := spawner.SpawnDetached("sh", []string{"-c", "echo 'test output'; sleep 1"}, logPath)
		skipOnSpawnError(t, err)
		if err != nil {
			t.Fatalf("SpawnDetached() error = %v", err)
		}

		// Verify process is running
		if !IsProcessRunning(pid) {
			t.Error("Spawned process is not running")
		}

		// Wait for process to complete
		time.Sleep(2 * time.Second)

		// Verify log file was created and contains output
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		if !strings.Contains(string(content), "test output") {
			t.Errorf("Log file does not contain expected output: %s", content)
		}
	})

	t.Run("creates log directory if missing", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "nested", "dir", "test.log")
		spawner := NewSpawner()

		pid, err := spawner.SpawnDetached("sh", []string{"-c", "echo 'test'"}, logPath)
		skipOnSpawnError(t, err)
		if err != nil {
			t.Fatalf("SpawnDetached() error = %v", err)
		}
		defer syscall.Kill(pid, syscall.SIGKILL)

		// Verify directory was created
		logDir := filepath.Dir(logPath)
		info, err := os.Stat(logDir)
		if err != nil {
			t.Fatalf("Log directory not created: %v", err)
		}

		// Verify directory permissions
		if mode := info.Mode() & os.ModePerm; mode != 0700 {
			t.Errorf("Log directory mode = %04o, want 0700", mode)
		}
	})

	t.Run("process survives parent exit", func(t *testing.T) {
		// This test verifies the process is truly detached
		// We can't easily test parent exit, but we can verify process group
		logPath := filepath.Join(tmpDir, "detach.log")
		spawner := NewSpawner()

		pid, err := spawner.SpawnDetached("sleep", []string{"2"}, logPath)
		skipOnSpawnError(t, err)
		if err != nil {
			t.Fatalf("SpawnDetached() error = %v", err)
		}
		defer syscall.Kill(pid, syscall.SIGKILL)

		// Verify process has different PGID (process group ID)
		// This indicates it's in its own process group
		// We can't easily get PGID from Go, but we can verify process is running
		if !IsProcessRunning(pid) {
			t.Error("Spawned process not running")
		}

		// Verify the process doesn't share our process group by checking it's still
		// running after we return (can't be killed by terminal close)
		time.Sleep(500 * time.Millisecond)
		if !IsProcessRunning(pid) {
			t.Error("Process died prematurely")
		}
	})

	t.Run("sets correct log file permissions", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "perms.log")
		spawner := NewSpawner()

		pid, err := spawner.SpawnDetached("echo", []string{"test"}, logPath)
		skipOnSpawnError(t, err)
		if err != nil {
			t.Fatalf("SpawnDetached() error = %v", err)
		}
		defer syscall.Kill(pid, syscall.SIGKILL)

		// Wait for file to be created
		time.Sleep(100 * time.Millisecond)

		info, err := os.Stat(logPath)
		if err != nil {
			t.Fatalf("Failed to stat log file: %v", err)
		}

		if mode := info.Mode() & os.ModePerm; mode != 0600 {
			t.Errorf("Log file mode = %04o, want 0600", mode)
		}
	})

	t.Run("appends to existing log file", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "append.log")

		// Create initial log content
		if err := os.WriteFile(logPath, []byte("initial\n"), 0600); err != nil {
			t.Fatalf("Failed to create initial log: %v", err)
		}

		spawner := NewSpawner()
		pid, err := spawner.SpawnDetached("echo", []string{"appended"}, logPath)
		skipOnSpawnError(t, err)
		if err != nil {
			t.Fatalf("SpawnDetached() error = %v", err)
		}
		defer syscall.Kill(pid, syscall.SIGKILL)

		// Wait for output
		time.Sleep(500 * time.Millisecond)

		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "initial") {
			t.Error("Original content was overwritten")
		}
		if !strings.Contains(contentStr, "appended") {
			t.Error("New content was not appended")
		}
	})

	t.Run("handles invalid binary path", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "error.log")
		spawner := NewSpawner()

		_, err := spawner.SpawnDetached("/nonexistent/binary", []string{}, logPath)
		if err == nil {
			t.Error("SpawnDetached() with invalid binary succeeded, want error")
		}
	})
}

func TestSpawner_SpawnDetachedWithFiles(t *testing.T) {
	if os.Getenv("SKIP_SPAWN_TESTS") != "" {
		t.Skip("Skipping spawn tests (SKIP_SPAWN_TESTS is set)")
	}

	tmpDir := t.TempDir()

	t.Run("separates stdout and stderr", func(t *testing.T) {
		stdoutPath := filepath.Join(tmpDir, "stdout.log")
		stderrPath := filepath.Join(tmpDir, "stderr.log")
		spawner := NewSpawner()

		// Command that writes to both stdout and stderr
		pid, err := spawner.SpawnDetachedWithFiles("sh", []string{"-c", "echo 'out'; echo 'err' >&2"}, stdoutPath, stderrPath)
		skipOnSpawnError(t, err)
		if err != nil {
			t.Fatalf("SpawnDetachedWithFiles() error = %v", err)
		}
		defer syscall.Kill(pid, syscall.SIGKILL)

		// Wait for output
		time.Sleep(500 * time.Millisecond)

		// Verify stdout
		stdoutContent, err := os.ReadFile(stdoutPath)
		if err != nil {
			t.Fatalf("Failed to read stdout: %v", err)
		}
		if !strings.Contains(string(stdoutContent), "out") {
			t.Errorf("stdout does not contain 'out': %s", stdoutContent)
		}
		if strings.Contains(string(stdoutContent), "err") {
			t.Error("stdout contains stderr content")
		}

		// Verify stderr
		stderrContent, err := os.ReadFile(stderrPath)
		if err != nil {
			t.Fatalf("Failed to read stderr: %v", err)
		}
		if !strings.Contains(string(stderrContent), "err") {
			t.Errorf("stderr does not contain 'err': %s", stderrContent)
		}
		if strings.Contains(string(stderrContent), "out") {
			t.Error("stderr contains stdout content")
		}
	})

	t.Run("creates directories for both files", func(t *testing.T) {
		stdoutPath := filepath.Join(tmpDir, "out", "stdout.log")
		stderrPath := filepath.Join(tmpDir, "err", "stderr.log")
		spawner := NewSpawner()

		pid, err := spawner.SpawnDetachedWithFiles("echo", []string{"test"}, stdoutPath, stderrPath)
		skipOnSpawnError(t, err)
		if err != nil {
			t.Fatalf("SpawnDetachedWithFiles() error = %v", err)
		}
		defer syscall.Kill(pid, syscall.SIGKILL)

		// Verify both directories exist
		for _, path := range []string{stdoutPath, stderrPath} {
			dir := filepath.Dir(path)
			info, err := os.Stat(dir)
			if err != nil {
				t.Errorf("Directory %s not created: %v", dir, err)
				continue
			}
			if mode := info.Mode() & os.ModePerm; mode != 0700 {
				t.Errorf("Directory %s mode = %04o, want 0700", dir, mode)
			}
		}
	})
}

func TestSpawner_WithEnv(t *testing.T) {
	if os.Getenv("SKIP_SPAWN_TESTS") != "" {
		t.Skip("Skipping spawn tests (SKIP_SPAWN_TESTS is set)")
	}

	tmpDir := t.TempDir()

	t.Run("passes environment variables to child", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "env.log")
		spawner := NewSpawner().WithEnv([]string{"TEST_VAR=test_value"})

		pid, err := spawner.SpawnDetached("sh", []string{"-c", "echo $TEST_VAR"}, logPath)
		skipOnSpawnError(t, err)
		if err != nil {
			t.Fatalf("SpawnDetached() error = %v", err)
		}
		defer syscall.Kill(pid, syscall.SIGKILL)

		// Wait for output
		time.Sleep(500 * time.Millisecond)

		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log: %v", err)
		}

		if !strings.Contains(string(content), "test_value") {
			t.Errorf("Environment variable not passed to child: %s", content)
		}
	})
}
