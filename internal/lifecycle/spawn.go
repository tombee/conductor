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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Spawner handles detached process spawning for daemon background mode.
type Spawner struct {
	// Additional environment variables to pass to the child process
	Env []string
}

// NewSpawner creates a new process spawner.
func NewSpawner() *Spawner {
	return &Spawner{
		Env: os.Environ(),
	}
}

// WithEnv sets additional environment variables for the spawned process.
func (s *Spawner) WithEnv(env []string) *Spawner {
	s.Env = env
	return s
}

// SpawnDetached spawns a detached background process.
// The process:
// - Runs in its own process group (not killed when parent exits)
// - Has stdin closed, stdout/stderr redirected to logPath
// - Has a new session ID (fully detached)
//
// Returns the PID of the spawned process.
func (s *Spawner) SpawnDetached(binary string, args []string, logPath string) (int, error) {
	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return 0, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file for output redirection
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	// Create command
	cmd := exec.Command(binary, args...)
	cmd.Env = s.Env

	// Redirect output to log file
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil // Close stdin

	// Configure process attributes for detachment
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Create new process group
		Setpgid: true,
		// Create new session (fully detach from terminal)
		Setsid: true,
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start process: %w", err)
	}

	// Get PID before releasing
	pid := cmd.Process.Pid

	// Release the process (don't wait for it)
	// This is safe because we configured it to be detached
	if err := cmd.Process.Release(); err != nil {
		// Process is already running, this is not fatal
		// but we should log it
		return pid, fmt.Errorf("process started but failed to release: %w", err)
	}

	return pid, nil
}

// SpawnDetachedWithFiles is like SpawnDetached but allows specifying separate stdout/stderr files.
func (s *Spawner) SpawnDetachedWithFiles(binary string, args []string, stdoutPath, stderrPath string) (int, error) {
	// Ensure log directories exist
	stdoutDir := filepath.Dir(stdoutPath)
	stderrDir := filepath.Dir(stderrPath)

	if err := os.MkdirAll(stdoutDir, 0700); err != nil {
		return 0, fmt.Errorf("failed to create stdout directory: %w", err)
	}
	if stdoutDir != stderrDir {
		if err := os.MkdirAll(stderrDir, 0700); err != nil {
			return 0, fmt.Errorf("failed to create stderr directory: %w", err)
		}
	}

	// Open output files
	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to open stdout file: %w", err)
	}
	defer stdoutFile.Close()

	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to open stderr file: %w", err)
	}
	defer stderrFile.Close()

	// Create command
	cmd := exec.Command(binary, args...)
	cmd.Env = s.Env

	// Redirect outputs
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	cmd.Stdin = nil

	// Configure process attributes for detachment
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Setsid:  true,
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start process: %w", err)
	}

	pid := cmd.Process.Pid

	// Release the process
	if err := cmd.Process.Release(); err != nil {
		return pid, fmt.Errorf("process started but failed to release: %w", err)
	}

	return pid, nil
}
