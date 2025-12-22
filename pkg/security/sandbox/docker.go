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
	"strings"
	"time"
)

const (
	// DefaultImage is the default container image for sandboxes
	DefaultImage = "alpine:latest"

	// DefaultTimeout is the default sandbox lifetime
	DefaultTimeout = 10 * time.Minute
)

// DockerFactory creates Docker-based sandboxes.
type DockerFactory struct {
	runtime string // "docker" or "podman"
}

// NewDockerFactory creates a factory for Docker/Podman sandboxes.
// Automatically detects whether Docker or Podman is available.
func NewDockerFactory() *DockerFactory {
	return &DockerFactory{
		runtime: detectRuntime(),
	}
}

// detectRuntime checks which container runtime is available.
func detectRuntime() string {
	// Check Docker first
	if _, err := exec.LookPath("docker"); err == nil {
		// Verify Docker is actually running
		cmd := exec.Command("docker", "info")
		if err := cmd.Run(); err == nil {
			return "docker"
		}
	}

	// Check Podman
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}

	return ""
}

// Type returns TypeDocker.
func (f *DockerFactory) Type() Type {
	return TypeDocker
}

// Available returns true if Docker or Podman is available.
func (f *DockerFactory) Available(ctx context.Context) bool {
	return f.runtime != ""
}

// Create creates a new Docker sandbox.
func (f *DockerFactory) Create(ctx context.Context, cfg Config) (Sandbox, error) {
	if !f.Available(ctx) {
		return nil, fmt.Errorf("no container runtime available (tried docker, podman)")
	}

	// Set defaults
	if cfg.Image == "" {
		cfg.Image = DefaultImage
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	// Create container
	s := &dockerSandbox{
		runtime:    f.runtime,
		config:     cfg,
		ctx:        ctx,
		workingDir: "/workspace",
	}

	if err := s.createContainer(); err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	return s, nil
}

// dockerSandbox implements Sandbox using Docker/Podman containers.
type dockerSandbox struct {
	runtime     string
	config      Config
	containerID string
	ctx         context.Context
	workingDir  string
}

// createContainer creates and starts the container.
func (s *dockerSandbox) createContainer() error {
	args := []string{"run", "--detach"}

	// Add resource limits
	if s.config.ResourceLimits.MaxMemory > 0 {
		args = append(args, "--memory", fmt.Sprintf("%d", s.config.ResourceLimits.MaxMemory))
	}
	if s.config.ResourceLimits.MaxCPU > 0 {
		// Docker uses --cpus with decimal (e.g., --cpus=0.5 for 50%)
		cpus := float64(s.config.ResourceLimits.MaxCPU) / 100.0
		args = append(args, "--cpus", fmt.Sprintf("%.2f", cpus))
	}
	if s.config.ResourceLimits.MaxProcesses > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", s.config.ResourceLimits.MaxProcesses))
	}

	// Configure network
	switch s.config.NetworkMode {
	case NetworkNone:
		args = append(args, "--network", "none")
	case NetworkFiltered:
		// Create custom network with filtering (would require additional setup)
		// For now, use host network and rely on application-level filtering
		// TODO: Implement iptables-based filtering or custom Docker network
		args = append(args, "--network", "bridge")
	case NetworkFull:
		args = append(args, "--network", "bridge")
	default:
		args = append(args, "--network", "none") // Secure default
	}

	// Add environment variables (filtered, no credentials)
	for k, v := range s.config.Env {
		// Additional safety: skip environment variables that look like credentials
		if isCredentialEnvVar(k) {
			continue
		}
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	// Mount working directory
	if s.config.WorkDir != "" {
		absWorkDir, err := filepath.Abs(s.config.WorkDir)
		if err != nil {
			return fmt.Errorf("failed to resolve work directory: %w", err)
		}
		args = append(args, "--volume", fmt.Sprintf("%s:%s", absWorkDir, s.workingDir))
	}

	// Mount read-only paths
	for _, path := range s.config.ReadOnlyPaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		// Mount to same path inside container, read-only
		args = append(args, "--volume", fmt.Sprintf("%s:%s:ro", absPath, absPath))
	}

	// Mount writable paths
	for _, path := range s.config.WritablePaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		args = append(args, "--volume", fmt.Sprintf("%s:%s", absPath, absPath))
	}

	// Security options
	args = append(args,
		"--security-opt", "no-new-privileges", // Prevent privilege escalation
		"--read-only",                         // Read-only root filesystem
		"--tmpfs", "/tmp:rw,noexec,nosuid",    // Writable /tmp without exec
	)

	// Set working directory
	args = append(args, "--workdir", s.workingDir)

	// Add labels for identification
	args = append(args,
		"--label", fmt.Sprintf("conductor.workflow=%s", s.config.WorkflowID),
		"--label", "conductor.sandbox=true",
	)

	// Image and command (sleep to keep container running)
	args = append(args, s.config.Image, "sleep", "infinity")

	// Create container
	cmd := exec.CommandContext(s.ctx, s.runtime, args...)
	output, err := cmd.Output()
	if err != nil {
		var stderr string
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return fmt.Errorf("failed to create container: %w (stderr: %s)", err, stderr)
	}

	s.containerID = strings.TrimSpace(string(output))
	return nil
}

// Execute runs a command in the sandbox.
func (s *dockerSandbox) Execute(ctx context.Context, cmd string, args []string) ([]byte, error) {
	if s.containerID == "" {
		return nil, fmt.Errorf("sandbox not initialized")
	}

	// Build exec command
	execArgs := []string{"exec", s.containerID}

	// Combine command and args
	fullCmd := append([]string{cmd}, args...)
	execArgs = append(execArgs, fullCmd...)

	// Execute
	execCmd := exec.CommandContext(ctx, s.runtime, execArgs...)
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()
	if err != nil {
		// Include stderr in error for debugging
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("command failed: %w (stderr: %s)", err, stderr.String())
		}
		return nil, fmt.Errorf("command failed: %w", err)
	}

	return stdout.Bytes(), nil
}

// WriteFile writes a file into the sandbox.
func (s *dockerSandbox) WriteFile(path string, content []byte) error {
	if s.containerID == "" {
		return fmt.Errorf("sandbox not initialized")
	}

	// Create a temporary file with the content
	tmpFile, err := os.CreateTemp("", "sandbox-write-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Use docker cp to copy into container
	targetPath := filepath.Join(s.workingDir, path)
	cpArgs := []string{"cp", tmpFile.Name(), fmt.Sprintf("%s:%s", s.containerID, targetPath)}

	cmd := exec.CommandContext(s.ctx, s.runtime, cpArgs...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file to container: %w", err)
	}

	return nil
}

// ReadFile reads a file from the sandbox.
func (s *dockerSandbox) ReadFile(path string) ([]byte, error) {
	if s.containerID == "" {
		return nil, fmt.Errorf("sandbox not initialized")
	}

	// Create a temporary directory for extraction
	tmpDir, err := os.MkdirTemp("", "sandbox-read-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use docker cp to copy from container
	sourcePath := filepath.Join(s.workingDir, path)
	cpArgs := []string{"cp", fmt.Sprintf("%s:%s", s.containerID, sourcePath), tmpDir}

	cmd := exec.CommandContext(s.ctx, s.runtime, cpArgs...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to copy file from container: %w", err)
	}

	// Read the copied file
	copiedPath := filepath.Join(tmpDir, filepath.Base(path))
	content, err := os.ReadFile(copiedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read copied file: %w", err)
	}

	return content, nil
}

// Cleanup destroys the container.
func (s *dockerSandbox) Cleanup() error {
	if s.containerID == "" {
		return nil
	}

	// Stop and remove container
	stopCmd := exec.Command(s.runtime, "rm", "--force", s.containerID)
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	s.containerID = ""
	return nil
}

// Stats returns container resource usage statistics.
func (s *dockerSandbox) Stats(ctx context.Context) (Stats, error) {
	if s.containerID == "" {
		return Stats{}, fmt.Errorf("sandbox not initialized")
	}

	// Get stats in JSON format
	cmd := exec.CommandContext(ctx, s.runtime, "stats", "--no-stream", "--format", "{{json .}}", s.containerID)
	_, err := cmd.Output()
	if err != nil {
		return Stats{}, fmt.Errorf("failed to get container stats: %w", err)
	}

	// Parse output (simplified - would need JSON parsing in production)
	// For now, return empty stats
	// TODO: Parse JSON output to populate Stats struct
	return Stats{}, nil
}

// StreamExecute runs a command with streaming I/O.
func (s *dockerSandbox) StreamExecute(ctx context.Context, cmd string, args []string, opts StreamExecuteOptions) error {
	if s.containerID == "" {
		return fmt.Errorf("sandbox not initialized")
	}

	execArgs := []string{"exec"}

	// Add interactive flags if stdin is provided
	if opts.Stdin != nil {
		execArgs = append(execArgs, "-i")
	}

	execArgs = append(execArgs, s.containerID)

	// Combine command and args
	fullCmd := append([]string{cmd}, args...)
	execArgs = append(execArgs, fullCmd...)

	// Execute
	execCmd := exec.CommandContext(ctx, s.runtime, execArgs...)
	execCmd.Stdout = opts.Stdout
	execCmd.Stderr = opts.Stderr
	execCmd.Stdin = opts.Stdin

	return execCmd.Run()
}

// isCredentialEnvVar checks if an environment variable name indicates credentials.
// This prevents accidentally passing credentials via environment variables.
func isCredentialEnvVar(name string) bool {
	upperName := strings.ToUpper(name)

	// Patterns that indicate credentials (per spec FR7.4)
	patterns := []string{
		"AWS_",
		"API_KEY",
		"APIKEY",
		"_TOKEN",
		"_SECRET",
		"_PASSWORD",
		"_PASS",
		"_PWD",
		"GITHUB_TOKEN",
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
	}

	for _, pattern := range patterns {
		if strings.Contains(upperName, pattern) {
			return true
		}
	}

	return false
}

// Verify dockerSandbox implements AdvancedSandbox interface
var _ AdvancedSandbox = (*dockerSandbox)(nil)
