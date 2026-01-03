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

package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/lifecycle"
)

// NewStopCommand creates the controller stop command.
func NewStopCommand() *cobra.Command {
	var (
		timeout time.Duration
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the conductor controller",
		Long: `Stop the conductor controller gracefully.

By default, sends SIGTERM and waits for graceful shutdown. If the timeout
is exceeded, sends SIGKILL to prevent orphaned processes.

Use --force to skip graceful shutdown and send SIGKILL immediately.

The stop command is idempotent: if the controller is not running,
it exits successfully after cleaning up stale PID files.`,
		Example: `  # Stop controller gracefully (SIGKILL if timeout exceeded)
  conductor controller stop

  # Stop with custom timeout before force kill
  conductor controller stop --timeout 60s

  # Skip graceful shutdown, kill immediately
  conductor controller stop --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop(cmd.Context(), stopOptions{
				timeout: timeout,
				force:   force,
			})
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Graceful shutdown timeout before SIGKILL")
	cmd.Flags().BoolVar(&force, "force", false, "Skip graceful shutdown, send SIGKILL immediately")

	return cmd
}

type stopOptions struct {
	timeout time.Duration
	force   bool
}

func runStop(ctx context.Context, opts stopOptions) error {
	// Load controller configuration
	cfg, err := config.LoadController("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine PID file path
	pidFilePath := cfg.Controller.PIDFile
	if pidFilePath == "" {
		// Default PID file location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		pidFilePath = filepath.Join(homeDir, ".conductor", "conductor.pid")
	}

	// Initialize lifecycle logger
	logPath := getLifecycleLogPath(cfg)
	lifecycleLog := lifecycle.NewLifecycleLogger(logPath)

	// Read PID file
	pidMgr := lifecycle.NewPIDFileManager(pidFilePath)
	pid, err := pidMgr.Read()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("Controller is not running (no PID file)")
			return nil
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	// Validate PID
	if pid <= 0 {
		return fmt.Errorf("invalid PID in file: %d", pid)
	}

	// Check if process is running
	if !lifecycle.IsProcessRunning(pid) {
		reason := "process not running"
		if err := lifecycleLog.LogStalePID(pid, reason); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", err)
		}

		fmt.Printf("Controller process %d is not running (removing stale PID file)\n", pid)

		if err := pidMgr.Remove(); err != nil {
			return fmt.Errorf("failed to remove stale PID file: %w", err)
		}

		return nil
	}

	// Validate it's a conductor process
	if !lifecycle.IsConductorProcess(pid) {
		return fmt.Errorf("PID %d is not a conductor process (refusing to stop)", pid)
	}

	// Log the stop operation
	if err := lifecycleLog.LogStop(pid, opts.force); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", err)
	}

	// Stop the controller
	startTime := time.Now()
	fmt.Printf("Stopping controller (PID %d)...\n", pid)

	if err := lifecycle.GracefulShutdown(pid, opts.timeout, opts.force); err != nil {
		if logErr := lifecycleLog.LogStopFailure(pid, err); logErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", logErr)
		}
		return fmt.Errorf("failed to stop controller: %w", err)
	}

	duration := time.Since(startTime)

	// Clean up PID file
	if err := pidMgr.Remove(); err != nil {
		// Process is stopped, so this is just a warning
		fmt.Fprintf(os.Stderr, "Warning: failed to remove PID file: %v\n", err)
	}

	if err := lifecycleLog.LogStopSuccess(pid, duration); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", err)
	}

	fmt.Println(shared.RenderOK("Controller stopped successfully"))
	return nil
}
