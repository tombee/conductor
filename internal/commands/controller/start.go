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
	"github.com/tombee/conductor/internal/client"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	controllerpkg "github.com/tombee/conductor/internal/controller"
	"github.com/tombee/conductor/internal/lifecycle"
)

// NewStartCommand creates the controller start command.
func NewStartCommand() *cobra.Command {
	var (
		foreground    bool
		timeout       time.Duration
		socket        string
		tcpAddr       string
		allowRemote   bool
		forceInsecure bool
		workflowsDir  string
		backend       string
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the conductor controller",
		Long: `Start the conductor controller in the background.

By default, the controller runs in the background and writes a PID file.
Use --foreground to run in the current terminal (no PID file, logs to stdout).

The start command is idempotent: if the controller is already running and healthy,
it exits successfully without starting a new instance.`,
		Example: `  # Start controller in background
  conductor controller start

  # Start in foreground (for systemd/docker)
  conductor controller start --foreground

  # Start with custom socket path
  conductor controller start --socket /tmp/conductor.sock

  # Start with TCP listener
  conductor controller start --tcp localhost:8080

  # Override health check timeout
  conductor controller start --timeout 60s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(cmd.Context(), startOptions{
				foreground:    foreground,
				timeout:       timeout,
				socket:        socket,
				tcpAddr:       tcpAddr,
				allowRemote:   allowRemote,
				forceInsecure: forceInsecure,
				workflowsDir:  workflowsDir,
				backend:       backend,
			})
		},
	}

	cmd.Flags().BoolVar(&foreground, "foreground", false, "Run in foreground (no PID file)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Health check timeout")
	cmd.Flags().StringVar(&socket, "socket", "", "Unix socket path")
	cmd.Flags().StringVar(&tcpAddr, "tcp", "", "TCP address to listen on")
	cmd.Flags().BoolVar(&allowRemote, "allow-remote", false, "Allow non-localhost TCP connections")
	cmd.Flags().BoolVar(&forceInsecure, "force-insecure", false, "Acknowledge insecure config (dev only)")
	cmd.Flags().StringVar(&workflowsDir, "workflows-dir", "", "Workflows directory")
	cmd.Flags().StringVar(&backend, "backend", "", "Storage backend (memory, postgres)")

	return cmd
}

type startOptions struct {
	foreground    bool
	timeout       time.Duration
	socket        string
	tcpAddr       string
	allowRemote   bool
	forceInsecure bool
	workflowsDir  string
	backend       string
}

func runStart(ctx context.Context, opts startOptions) error {
	// Load controller configuration
	cfg, err := config.LoadController("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply flag overrides
	if opts.socket != "" {
		cfg.Controller.Listen.SocketPath = opts.socket
	}
	if opts.tcpAddr != "" {
		cfg.Controller.Listen.TCPAddr = opts.tcpAddr
	}
	if opts.allowRemote {
		cfg.Controller.Listen.AllowRemote = true
	}
	if opts.workflowsDir != "" {
		cfg.Controller.WorkflowsDir = opts.workflowsDir
	}
	if opts.backend != "" {
		cfg.Controller.Backend.Type = opts.backend
	}
	if opts.forceInsecure {
		cfg.Controller.ForceInsecure = true
	}

	// Determine PID file path (unless foreground mode)
	var pidFilePath string
	if !opts.foreground {
		pidFilePath = cfg.Controller.PIDFile
		if pidFilePath == "" {
			// Default PID file location
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			pidFilePath = filepath.Join(homeDir, ".conductor", "conductor.pid")
		}
	}

	// Initialize lifecycle logger
	logPath := getLifecycleLogPath(cfg)
	lifecycleLog := lifecycle.NewLifecycleLogger(logPath)

	// Build args for logging
	args := buildControllerArgs(cfg, opts)
	if err := lifecycleLog.LogStart("", args, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", err)
	}

	// Foreground mode: run controller inline
	if opts.foreground {
		fmt.Println("Starting controller in foreground mode...")

		runOpts := controllerpkg.RunOptions{
			Version:      "", // Will be set from build ldflags in main
			Commit:       "",
			BuildDate:    "",
			BackendType:  opts.backend,
			SocketPath:   cfg.Controller.Listen.SocketPath,
			TCPAddr:      cfg.Controller.Listen.TCPAddr,
			AllowRemote:  cfg.Controller.Listen.AllowRemote,
			WorkflowsDir: cfg.Controller.WorkflowsDir,
		}

		if err := controllerpkg.Run(runOpts); err != nil {
			if logErr := lifecycleLog.LogStartFailure(err); logErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", logErr)
			}
			return err
		}

		return nil
	}

	// Background mode: check if already running
	if pidFilePath != "" {
		pidMgr := lifecycle.NewPIDFileManager(pidFilePath)

		// Try to read existing PID
		existingPID, err := pidMgr.Read()
		if err == nil {
			// PID file exists, check if process is running
			if lifecycle.IsProcessRunning(existingPID) && lifecycle.IsConductorProcess(existingPID) {
				// Process is running, check if healthy using client
				if err := waitForHealthy(cfg, 5*time.Second); err == nil {
					if err := lifecycleLog.LogAlreadyRunning(existingPID); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", err)
					}
					fmt.Printf("Controller is already running (PID %d)\n", existingPID)
					return nil
				}

				// Process exists but unhealthy - log warning
				fmt.Fprintf(os.Stderr, "Warning: controller process exists (PID %d) but is unhealthy, starting new instance\n", existingPID)
			} else {
				// Stale PID file
				reason := "process not running"
				if err := lifecycleLog.LogStalePID(existingPID, reason); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", err)
				}
				fmt.Fprintf(os.Stderr, "Warning: removing stale PID file (process %d not running)\n", existingPID)
				if err := pidMgr.Remove(); err != nil {
					return fmt.Errorf("failed to remove stale PID file: %w", err)
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			// Error reading PID file (not just "doesn't exist")
			return fmt.Errorf("failed to check existing controller: %w", err)
		}
	}

	// Spawn detached controller process
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build controller arguments
	controllerArgs := buildControllerArgs(cfg, opts)

	// Determine controller log path
	controllerLogPath := getControllerLogPath(cfg)

	// Spawn the process
	spawner := lifecycle.NewSpawner()
	pid, err := spawner.SpawnDetached(binaryPath, controllerArgs, controllerLogPath)
	if err != nil {
		if logErr := lifecycleLog.LogStartFailure(err); logErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", logErr)
		}
		return fmt.Errorf("failed to spawn controller: %w", err)
	}

	// Wait for controller to become healthy
	startTime := time.Now()
	fmt.Printf("Starting controller (PID %d)...\n", pid)

	if err := waitForHealthy(cfg, opts.timeout); err != nil {
		// Controller failed to become healthy - try to clean up
		_ = lifecycle.SendSignal(pid, 15) // SIGTERM

		if logErr := lifecycleLog.LogStartFailure(err); logErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write lifecycle log: %v\n", logErr)
		}
		return fmt.Errorf("controller failed to become healthy within %v: %w", opts.timeout, err)
	}

	duration := time.Since(startTime)

	// Write PID file
	if pidFilePath != "" {
		pidMgr := lifecycle.NewPIDFileManager(pidFilePath)
		if err := pidMgr.Create(pid); err != nil {
			// Controller is running but we couldn't write PID file - warn but don't fail
			fmt.Fprintf(os.Stderr, "%s\n", shared.RenderWarn(fmt.Sprintf("Controller started but failed to write PID file: %v", err)))
			fmt.Println(shared.RenderOK(fmt.Sprintf("Controller started successfully (PID %d)", pid)))
			return nil
		}
	}

	if err := lifecycleLog.LogStartSuccess(pid, 0, duration); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", shared.RenderWarn(fmt.Sprintf("Failed to write lifecycle log: %v", err)))
	}

	fmt.Println(shared.RenderOK(fmt.Sprintf("Controller started successfully (PID %d)", pid)))
	return nil
}

// buildControllerArgs constructs the arguments to pass to the spawned controller process.
func buildControllerArgs(cfg *config.Config, opts startOptions) []string {
	args := []string{"--controller-child"}

	if cfg.Controller.Listen.SocketPath != "" {
		args = append(args, "--socket", cfg.Controller.Listen.SocketPath)
	}
	if cfg.Controller.Listen.TCPAddr != "" {
		args = append(args, "--tcp", cfg.Controller.Listen.TCPAddr)
	}
	if cfg.Controller.Listen.AllowRemote {
		args = append(args, "--allow-remote")
	}
	if cfg.Controller.WorkflowsDir != "" {
		args = append(args, "--workflows-dir", cfg.Controller.WorkflowsDir)
	}
	if cfg.Controller.Backend.Type != "" {
		args = append(args, "--backend", cfg.Controller.Backend.Type)
	}
	if cfg.Controller.ForceInsecure {
		args = append(args, "--force-insecure")
	}

	return args
}

// waitForHealthy polls the controller until it becomes healthy or times out.
// Uses the internal client which handles Unix socket connections properly.
func waitForHealthy(cfg *config.Config, timeout time.Duration) error {
	// Create a client that connects to the socket
	socketPath := cfg.Controller.Listen.SocketPath
	if socketPath == "" {
		homeDir, _ := os.UserHomeDir()
		socketPath = filepath.Join(homeDir, ".conductor", "conductor.sock")
	}

	transport := client.NewUnixTransport(socketPath)
	c, err := client.New(client.WithTransport(transport))
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Poll with exponential backoff
	interval := 100 * time.Millisecond
	maxInterval := time.Second
	deadline := time.Now().Add(timeout)
	attempts := 0

	for time.Now().Before(deadline) {
		attempts++
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := c.Ping(ctx)
		cancel()

		if err == nil {
			return nil // Healthy!
		}

		// Wait before retrying
		time.Sleep(interval)
		interval = time.Duration(float64(interval) * 1.5)
		if interval > maxInterval {
			interval = maxInterval
		}
	}

	return fmt.Errorf("health check timeout after %d attempts", attempts)
}

// getLifecycleLogPath returns the lifecycle log file path.
func getLifecycleLogPath(cfg *config.Config) string {
	// Default: ~/.local/share/conductor/lifecycle.log
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/conductor-lifecycle.log"
	}

	return filepath.Join(homeDir, ".local", "share", "conductor", "lifecycle.log")
}

// getControllerLogPath returns the controller output log file path.
func getControllerLogPath(cfg *config.Config) string {
	// Default: ~/.local/share/conductor/controller.log
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/conductor-controller.log"
	}

	return filepath.Join(homeDir, ".local", "share", "conductor", "controller.log")
}
