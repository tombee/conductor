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

// Package sandbox provides process isolation for Conductor tool execution.
//
// The sandbox package implements multiple isolation strategies:
//   - Docker/Podman containers (primary, cross-platform)
//   - Process-level fallback with restricted environment
//
// Sandboxes provide filesystem, network, and resource isolation for tools
// running under strict and air-gapped security profiles.
package sandbox

import (
	"context"
	"io"
	"time"
)

// Sandbox represents an isolated execution environment for tools.
//
// Sandboxes provide:
//   - Filesystem isolation (bind mounts, read-only roots)
//   - Network isolation (disabled or filtered)
//   - Resource limits (CPU, memory, process count)
//   - Security policies (seccomp, capabilities)
//
// A sandbox lifecycle:
//  1. Create sandbox with Config
//  2. Execute commands, read/write files
//  3. Cleanup to release resources
//
// Sandboxes are NOT thread-safe. Each workflow should use its own sandbox.
type Sandbox interface {
	// Execute runs a command in the sandbox.
	// Returns command output and error.
	Execute(ctx context.Context, cmd string, args []string) ([]byte, error)

	// WriteFile writes a file into the sandbox filesystem.
	// Path is relative to the sandbox root.
	WriteFile(path string, content []byte) error

	// ReadFile reads a file from the sandbox filesystem.
	// Path is relative to the sandbox root.
	ReadFile(path string) ([]byte, error)

	// Cleanup destroys the sandbox and releases all resources.
	// Must be called to prevent resource leaks.
	Cleanup() error
}

// Config defines sandbox configuration.
type Config struct {
	// WorkflowID identifies the workflow using this sandbox
	WorkflowID string

	// WorkDir is the working directory path to mount in the sandbox
	WorkDir string

	// ReadOnlyPaths are paths to mount read-only in the sandbox
	ReadOnlyPaths []string

	// WritablePaths are paths to mount writable in the sandbox
	WritablePaths []string

	// NetworkMode controls network access
	NetworkMode NetworkMode

	// AllowedHosts lists hosts accessible when NetworkMode is NetworkFiltered
	AllowedHosts []string

	// ResourceLimits defines resource constraints
	ResourceLimits ResourceLimits

	// Env is a filtered environment variable map
	// Credentials should NOT be passed here - use tmpfs-mounted secrets instead
	Env map[string]string

	// Image is the container image to use (defaults to a minimal base image)
	Image string

	// Timeout is the maximum lifetime for this sandbox
	Timeout time.Duration
}

// NetworkMode defines sandbox network isolation level.
type NetworkMode string

const (
	// NetworkNone disables all network access (air-gapped)
	NetworkNone NetworkMode = "none"

	// NetworkFiltered allows access only to AllowedHosts
	NetworkFiltered NetworkMode = "filtered"

	// NetworkFull allows unrestricted network access
	NetworkFull NetworkMode = "full"
)

// ResourceLimits defines resource constraints for sandbox execution.
type ResourceLimits struct {
	// MaxMemory is the maximum memory in bytes (0 = no limit)
	MaxMemory int64

	// MaxCPU is CPU quota as percentage (100 = 1 core, 0 = no limit)
	MaxCPU int

	// MaxProcesses is the maximum number of processes (0 = no limit)
	MaxProcesses int

	// MaxFileSize is the maximum file size in bytes (0 = no limit)
	MaxFileSize int64
}

// Type represents the sandbox implementation type.
type Type string

const (
	// TypeDocker uses Docker/Podman containers
	TypeDocker Type = "docker"

	// TypeFallback uses process-level isolation
	TypeFallback Type = "fallback"

	// TypeNone indicates no sandbox available
	TypeNone Type = "none"
)

// Factory creates sandbox instances.
type Factory interface {
	// Create creates a new sandbox with the given configuration.
	Create(ctx context.Context, cfg Config) (Sandbox, error)

	// Type returns the sandbox type this factory creates.
	Type() Type

	// Available returns true if this sandbox type is available on this system.
	Available(ctx context.Context) bool
}

// Stats provides sandbox resource usage statistics.
type Stats struct {
	// MemoryUsage in bytes
	MemoryUsage int64

	// CPUUsage as percentage
	CPUUsage float64

	// ProcessCount is the number of running processes
	ProcessCount int

	// NetworkRX bytes received
	NetworkRX int64

	// NetworkTX bytes transmitted
	NetworkTX int64
}

// StreamExecuteOptions configures streaming command execution.
type StreamExecuteOptions struct {
	// Stdout receives command stdout
	Stdout io.Writer

	// Stderr receives command stderr
	Stderr io.Writer

	// Stdin provides command stdin
	Stdin io.Reader
}

// AdvancedSandbox extends Sandbox with optional advanced features.
//
// Not all sandbox implementations support these features.
// Check via type assertion before using.
type AdvancedSandbox interface {
	Sandbox

	// Stats returns current resource usage statistics.
	Stats(ctx context.Context) (Stats, error)

	// StreamExecute runs a command with streaming I/O.
	StreamExecute(ctx context.Context, cmd string, args []string, opts StreamExecuteOptions) error
}
