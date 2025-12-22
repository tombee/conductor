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
	"time"
)

// AutoStartConfig configures automatic controller startup behavior.
type AutoStartConfig struct {
	// Enabled enables automatic controller startup.
	Enabled bool

	// SocketPath is the socket path to use (empty for default).
	SocketPath string

	// StartTimeout is how long to wait for the controller to start.
	StartTimeout time.Duration
}

// StartController starts the conductor controller in the background.
// Returns nil if the controller starts successfully within the timeout.
func StartController(cfg AutoStartConfig) error {
	if cfg.StartTimeout == 0 {
		cfg.StartTimeout = 10 * time.Second
	}

	// Find conductor binary
	conductorPath, err := exec.LookPath("conductor")
	if err != nil {
		return fmt.Errorf("conductor not found in PATH: %w", err)
	}

	// Build command arguments
	args := []string{"controller", "start", "--foreground"}
	if cfg.SocketPath != "" {
		args = append(args, "--socket", cfg.SocketPath)
	}

	// Start controller in background
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
		return fmt.Errorf("failed to start controller: %w", err)
	}

	// Wait for controller to become available
	ctx, cancel := context.WithTimeout(context.Background(), cfg.StartTimeout)
	defer cancel()

	client, err := FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Poll until controller is ready
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for controller to start")
		case <-ticker.C:
			if err := client.Ping(ctx); err == nil {
				return nil
			}
		}
	}
}

// EnsureController ensures the controller is running, starting it if needed and if auto-start is enabled.
// Returns a client connected to the controller.
func EnsureController(cfg AutoStartConfig) (*Client, error) {
	client, err := FromEnvironment()
	if err != nil {
		return nil, err
	}

	// Try to connect
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err = client.Ping(ctx)
	cancel()

	if err == nil {
		// Controller is running
		return client, nil
	}

	// Check if controller is not running
	if !IsControllerNotRunning(err) {
		return nil, fmt.Errorf("failed to connect to controller: %w", err)
	}

	// Auto-start if enabled
	if !cfg.Enabled {
		cnr := &ControllerNotRunningError{}
		return nil, cnr
	}

	// Start the controller
	if err := StartController(cfg); err != nil {
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
