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
	"syscall"
	"time"
)

var (
	// ErrProcessNotRunning is returned when the process does not exist.
	ErrProcessNotRunning = errors.New("process not running")

	// ErrNotConductorProcess is returned when the process is not a conductor controller.
	ErrNotConductorProcess = errors.New("process is not a conductor controller")

	// ErrShutdownTimeout is returned when the process doesn't exit within the timeout.
	ErrShutdownTimeout = errors.New("shutdown timeout exceeded")
)

// ProcessInfo contains information about a running process.
type ProcessInfo struct {
	PID     int
	Running bool
	Command string
}

// IsProcessRunning checks if a process with the given PID exists.
func IsProcessRunning(pid int) bool {
	// Send signal 0 to check if process exists
	// This doesn't actually send a signal, just checks permissions
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// IsConductorProcess checks if the given PID is a conductor controller process.
// This prevents sending signals to unrelated processes if the PID file is stale.
func IsConductorProcess(pid int) bool {
	return isConductorProcess(pid)
}

// SendSignal sends a signal to the given process.
func SendSignal(pid int, sig syscall.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if err := proc.Signal(sig); err != nil {
		return fmt.Errorf("failed to send signal %v to process %d: %w", sig, pid, err)
	}

	return nil
}

// WaitForExit waits for the process to exit, checking every interval.
// Returns ErrShutdownTimeout if the process is still running after timeout.
func WaitForExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := 100 * time.Millisecond

	for time.Now().Before(deadline) {
		if !IsProcessRunning(pid) {
			return nil
		}
		time.Sleep(interval)
	}

	return ErrShutdownTimeout
}

// GracefulShutdown sends SIGTERM to a process and waits for it to exit.
// If force is true and the timeout is exceeded, sends SIGKILL.
func GracefulShutdown(pid int, timeout time.Duration, force bool) error {
	// Verify process is running
	if !IsProcessRunning(pid) {
		return ErrProcessNotRunning
	}

	// Send SIGTERM
	if err := SendSignal(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for process to exit
	err := WaitForExit(pid, timeout)
	if err == nil {
		return nil // Process exited gracefully
	}

	if !force {
		return err // Timeout but force not requested
	}

	// Force kill with SIGKILL
	if err := SendSignal(pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to send SIGKILL: %w", err)
	}

	// Wait a short time for SIGKILL to take effect
	if err := WaitForExit(pid, 5*time.Second); err != nil {
		return fmt.Errorf("process did not die after SIGKILL: %w", err)
	}

	return nil
}

// GetProcessInfo returns information about the process with the given PID.
func GetProcessInfo(pid int) (*ProcessInfo, error) {
	info := &ProcessInfo{
		PID:     pid,
		Running: IsProcessRunning(pid),
	}

	if info.Running {
		cmd, err := getProcessCommand(pid)
		if err != nil {
			// Process exists but we can't read command - that's ok
			info.Command = "<unknown>"
		} else {
			info.Command = cmd
		}
	}

	return info, nil
}
