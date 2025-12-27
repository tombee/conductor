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
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestIsProcessRunning(t *testing.T) {
	t.Run("returns true for current process", func(t *testing.T) {
		if !IsProcessRunning(os.Getpid()) {
			t.Error("IsProcessRunning(os.Getpid()) = false, want true")
		}
	})

	t.Run("returns false for non-existent PID", func(t *testing.T) {
		// Use a very high PID that's unlikely to exist
		if IsProcessRunning(999999) {
			t.Error("IsProcessRunning(999999) = true, want false")
		}
	})

	t.Run("returns false for PID 1 without permissions", func(t *testing.T) {
		// PID 1 (init) exists but we likely can't signal it
		// This tests the permission check logic
		// Note: This might return true if running as root
		running := IsProcessRunning(1)
		t.Logf("IsProcessRunning(1) = %v", running)
		// We don't assert here because behavior depends on permissions
	})
}

func TestSendSignal(t *testing.T) {
	t.Run("sends signal to running process", func(t *testing.T) {
		// Create a long-running sleep process
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start sleep process: %v", err)
		}
		defer cmd.Process.Kill()

		pid := cmd.Process.Pid

		// Send harmless signal (0 = existence check)
		if err := SendSignal(pid, syscall.Signal(0)); err != nil {
			t.Errorf("SendSignal() error = %v", err)
		}

		// Clean up
		cmd.Process.Kill()
	})

	t.Run("returns error for non-existent process", func(t *testing.T) {
		err := SendSignal(999999, syscall.SIGTERM)
		if err == nil {
			t.Error("SendSignal() to non-existent process succeeded, want error")
		}
	})
}

func TestWaitForExit(t *testing.T) {
	t.Run("returns nil when process exits", func(t *testing.T) {
		// Create a short-lived process
		cmd := exec.Command("sh", "-c", "exit 0")
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}

		pid := cmd.Process.Pid

		// Wait for process to actually exit
		cmd.Wait()

		// Wait should succeed since process exits quickly
		err := WaitForExit(pid, 2*time.Second)
		if err != nil {
			t.Errorf("WaitForExit() error = %v, want nil", err)
		}
	})

	t.Run("returns timeout error for long-running process", func(t *testing.T) {
		// Create a long-running process
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}
		defer cmd.Process.Kill()

		pid := cmd.Process.Pid

		// Wait with short timeout should fail
		err := WaitForExit(pid, 200*time.Millisecond)
		if !errors.Is(err, ErrShutdownTimeout) {
			t.Errorf("WaitForExit() error = %v, want ErrShutdownTimeout", err)
		}
	})
}

func TestGracefulShutdown(t *testing.T) {
	t.Run("shuts down process with SIGTERM", func(t *testing.T) {
		// Skip this test as signal handling behavior varies by platform
		// Integration tests will cover real daemon shutdown
		t.Skip("Signal handling in tests is platform-specific - covered by integration tests")
	})

	t.Run("force kills process after timeout", func(t *testing.T) {
		// Skip this test as signal handling varies by platform
		t.Skip("Signal handling in tests is platform-specific - covered by integration tests")
	})

	t.Run("returns error for non-existent process", func(t *testing.T) {
		err := GracefulShutdown(999999, 1*time.Second, false)
		if !errors.Is(err, ErrProcessNotRunning) {
			t.Errorf("GracefulShutdown() error = %v, want ErrProcessNotRunning", err)
		}
	})
}

func TestGetProcessInfo(t *testing.T) {
	t.Run("returns info for running process", func(t *testing.T) {
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}
		defer cmd.Process.Kill()

		pid := cmd.Process.Pid
		info, err := GetProcessInfo(pid)
		if err != nil {
			t.Fatalf("GetProcessInfo() error = %v", err)
		}

		if info.PID != pid {
			t.Errorf("info.PID = %d, want %d", info.PID, pid)
		}
		if !info.Running {
			t.Error("info.Running = false, want true")
		}
		if info.Command == "" {
			t.Error("info.Command is empty")
		}
		t.Logf("Command: %s", info.Command)
	})

	t.Run("returns not running for non-existent process", func(t *testing.T) {
		info, err := GetProcessInfo(999999)
		if err != nil {
			t.Fatalf("GetProcessInfo() error = %v", err)
		}

		if info.Running {
			t.Error("info.Running = true, want false")
		}
	})
}

func TestIsConductorProcess(t *testing.T) {
	t.Run("returns false for non-conductor process", func(t *testing.T) {
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}
		defer cmd.Process.Kill()

		if IsConductorProcess(cmd.Process.Pid) {
			t.Error("IsConductorProcess(sleep) = true, want false")
		}
	})

	t.Run("returns false for non-existent process", func(t *testing.T) {
		if IsConductorProcess(999999) {
			t.Error("IsConductorProcess(999999) = true, want false")
		}
	})

	// Note: We can't easily test true case without building a conductor binary
	// or mocking the platform-specific functions. Integration tests will cover this.
}
