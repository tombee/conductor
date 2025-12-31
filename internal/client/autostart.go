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

package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// AutoStartConfig configures automatic daemon startup behavior.
type AutoStartConfig struct {
	// Enabled enables automatic daemon startup.
	Enabled bool

	// SocketPath is the socket path to use (empty for default).
	SocketPath string

	// StartTimeout is how long to wait for the daemon to start.
	StartTimeout time.Duration
}

// StartDaemon starts the conductor daemon in the background.
// Returns nil if the daemon starts successfully within the timeout.
func StartDaemon(cfg AutoStartConfig) error {
	if cfg.StartTimeout == 0 {
		cfg.StartTimeout = 10 * time.Second
	}

	// Find conductor binary (try conductord first for backwards compat, then conductor)
	var conductorPath string
	var err error
	conductorPath, err = exec.LookPath("conductord")
	if err != nil {
		conductorPath, err = exec.LookPath("conductor")
		if err != nil {
			return fmt.Errorf("conductor not found in PATH: %w", err)
		}
	}

	// Build command arguments
	// If we found "conductor", use "daemon start --foreground"
	// If we found "conductord", just pass socket args directly
	var args []string
	baseName := filepath.Base(conductorPath)
	if baseName == "conductor" || baseName == "conductor.exe" {
		args = []string{"daemon", "start", "--foreground"}
	}
	if cfg.SocketPath != "" {
		args = append(args, "--socket", cfg.SocketPath)
	}

	// Start daemon in background
	cmd := exec.Command(conductorPath, args...)
	cmd.Stdout = nil // Detach stdout
	cmd.Stderr = nil // Detach stderr
	cmd.Stdin = nil

	// Set environment variable to mark this as auto-started
	// Inherit parent environment and add CONDUCTOR_AUTO_STARTED=1
	cmd.Env = append(os.Environ(), "CONDUCTOR_AUTO_STARTED=1")

	// Set up process group for proper detachment
	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to become available
	ctx, cancel := context.WithTimeout(context.Background(), cfg.StartTimeout)
	defer cancel()

	client, err := FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Poll until daemon is ready
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for daemon to start")
		case <-ticker.C:
			if err := client.Ping(ctx); err == nil {
				return nil
			}
		}
	}
}

// EnsureDaemon ensures the daemon is running, starting it if needed and if auto-start is enabled.
// Returns a client connected to the daemon.
func EnsureDaemon(cfg AutoStartConfig) (*Client, error) {
	client, err := FromEnvironment()
	if err != nil {
		return nil, err
	}

	// Try to connect
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err = client.Ping(ctx)
	cancel()

	if err == nil {
		// Daemon is running
		return client, nil
	}

	// Check if daemon is not running
	if !IsDaemonNotRunning(err) {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	// Auto-start if enabled
	if !cfg.Enabled {
		dnr := &DaemonNotRunningError{}
		return nil, dnr
	}

	// Start the daemon
	if err := StartDaemon(cfg); err != nil {
		return nil, fmt.Errorf("auto-start failed: %w", err)
	}

	// Return fresh client
	return FromEnvironment()
}

// setSysProcAttr sets OS-specific process attributes for proper detachment.
// This is defined in separate files for Unix and Windows.
func setSysProcAttr(cmd *exec.Cmd) {
	setSysProcAttrPlatform(cmd)
}
