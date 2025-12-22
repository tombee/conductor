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

package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// FallbackFactory creates fallback process-level sandboxes.
//
// Fallback sandboxes provide degraded isolation when containers are unavailable:
//   - Restricted environment variables
//   - Separate process group for cleanup
//   - No resource limits (would require cgroups)
//   - No filesystem isolation (relies on SecurityInterceptor enforcement)
//
// This is NOT equivalent to container isolation but provides basic process separation.
type FallbackFactory struct{}

// NewFallbackFactory creates a factory for fallback sandboxes.
func NewFallbackFactory() *FallbackFactory {
	return &FallbackFactory{}
}

// Type returns TypeFallback.
func (f *FallbackFactory) Type() Type {
	return TypeFallback
}

// Available always returns true (fallback is always available).
func (f *FallbackFactory) Available(ctx context.Context) bool {
	return true
}

// Create creates a new fallback sandbox.
func (f *FallbackFactory) Create(ctx context.Context, cfg Config) (Sandbox, error) {
	// Create a temporary directory for sandbox isolation
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("conductor-sandbox-%s-*", cfg.WorkflowID))
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox temp directory: %w", err)
	}

	s := &fallbackSandbox{
		config:  cfg,
		tmpDir:  tmpDir,
		workDir: cfg.WorkDir,
	}

	return s, nil
}

// fallbackSandbox implements Sandbox using process-level isolation.
//
// Limitations compared to container sandboxes:
//   - No memory/CPU limits (no cgroups)
//   - No network isolation (relies on application-level filtering)
//   - No filesystem isolation (relies on SecurityInterceptor)
//   - No seccomp filtering
//
// Security relies on:
//   - Restricted environment (no credential env vars)
//   - Process group isolation (cleanup)
//   - SecurityInterceptor enforcement in parent process
type fallbackSandbox struct {
	config  Config
	tmpDir  string
	workDir string
}

// Execute runs a command in a subprocess with restricted environment.
func (s *fallbackSandbox) Execute(ctx context.Context, cmd string, args []string) ([]byte, error) {
	// Create command
	execCmd := exec.CommandContext(ctx, cmd, args...)

	// Set working directory
	if s.workDir != "" {
		execCmd.Dir = s.workDir
	} else {
		execCmd.Dir = s.tmpDir
	}

	// Build restricted environment
	execCmd.Env = s.buildRestrictedEnv()

	// Set process group (for cleanup)
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}

	// Execute
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("command failed: %w (stderr: %s)", err, stderr.String())
		}
		return nil, fmt.Errorf("command failed: %w", err)
	}

	return stdout.Bytes(), nil
}

// WriteFile writes a file to the working directory.
func (s *fallbackSandbox) WriteFile(path string, content []byte) error {
	// Resolve path relative to work directory
	targetPath := filepath.Join(s.getBasePath(), path)

	// Ensure directory exists
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ReadFile reads a file from the working directory.
func (s *fallbackSandbox) ReadFile(path string) ([]byte, error) {
	// Resolve path relative to work directory
	targetPath := filepath.Join(s.getBasePath(), path)

	// Read file
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return content, nil
}

// Cleanup removes the temporary directory.
func (s *fallbackSandbox) Cleanup() error {
	if s.tmpDir != "" {
		if err := os.RemoveAll(s.tmpDir); err != nil {
			return fmt.Errorf("failed to cleanup sandbox: %w", err)
		}
		s.tmpDir = ""
	}
	return nil
}

// getBasePath returns the base path for file operations.
func (s *fallbackSandbox) getBasePath() string {
	if s.workDir != "" {
		return s.workDir
	}
	return s.tmpDir
}

// buildRestrictedEnv creates a minimal environment without credentials.
//
// Per spec FR7.4:
//   - Filter out AWS_*, API_KEY*, *_TOKEN, etc.
//   - Include only safe environment variables
//   - Never pass credentials via environment
func (s *fallbackSandbox) buildRestrictedEnv() []string {
	env := []string{
		// Minimal safe environment
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=" + s.tmpDir,
		"TMPDIR=" + s.tmpDir,
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
	}

	// Add configured environment variables (already filtered)
	for k, v := range s.config.Env {
		// Double-check: skip anything that looks like credentials
		if isCredentialEnvVar(k) {
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// Verify fallbackSandbox implements Sandbox interface
var _ Sandbox = (*fallbackSandbox)(nil)
