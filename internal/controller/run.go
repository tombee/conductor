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
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/log"
)

// RunOptions configures controller execution.
type RunOptions struct {
	Version   string
	Commit    string
	BuildDate string

	// Config overrides
	BackendType  string
	PostgresURL  string
	Distributed  bool
	InstanceID   string
	SocketPath   string
	TCPAddr      string
	WorkflowsDir string
	TLSCert      string
	TLSKey       string
	AllowRemote  bool
}

// Run starts the controller and blocks until shutdown.
// This is the main entry point for controller execution, used by both
// foreground mode (conductor controller start --foreground) and background
// mode (conductor --controller-child).
func Run(opts RunOptions) error {
	// Initialize structured logging from environment
	logger := log.New(log.FromEnv())
	slog.SetDefault(logger)

	// Load controller configuration
	cfg, err := config.LoadController("")
	if err != nil {
		logger.Error("Failed to load config", slog.Any("error", err))
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply overrides from options
	if opts.BackendType != "" {
		cfg.Controller.Backend.Type = opts.BackendType
	}
	if opts.PostgresURL != "" {
		cfg.Controller.Backend.Postgres.ConnectionString = opts.PostgresURL
	}
	if opts.Distributed {
		cfg.Controller.Distributed.Enabled = true
	}
	if opts.InstanceID != "" {
		cfg.Controller.Distributed.InstanceID = opts.InstanceID
	}
	if opts.SocketPath != "" {
		cfg.Controller.Listen.SocketPath = opts.SocketPath
	}
	if opts.TCPAddr != "" {
		cfg.Controller.Listen.TCPAddr = opts.TCPAddr
	}
	if opts.WorkflowsDir != "" {
		cfg.Controller.WorkflowsDir = opts.WorkflowsDir
	}
	if opts.TLSCert != "" {
		cfg.Controller.Listen.TLSCert = opts.TLSCert
	}
	if opts.TLSKey != "" {
		cfg.Controller.Listen.TLSKey = opts.TLSKey
	}
	if opts.AllowRemote {
		cfg.Controller.Listen.AllowRemote = true
		logger.Warn("--allow-remote is enabled. The controller will accept connections from any network address. Ensure you have proper authentication and TLS configured for production use.")
	}

	// Create controller instance
	d, err := New(cfg, Options{
		Version:   opts.Version,
		Commit:    opts.Commit,
		BuildDate: opts.BuildDate,
	})
	if err != nil {
		logger.Error("Failed to create controller", slog.Any("error", err))
		return fmt.Errorf("failed to create controller: %w", err)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start controller
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(ctx)
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
		cancel()
		if err := d.Shutdown(context.Background()); err != nil {
			logger.Error("Error during shutdown", slog.Any("error", err))
			return fmt.Errorf("shutdown error: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			logger.Error("Controller error", slog.Any("error", err))
			return fmt.Errorf("controller error: %w", err)
		}
		return nil
	}
}
