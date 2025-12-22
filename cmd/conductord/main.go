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

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/daemon"
	"github.com/tombee/conductor/internal/log"
)

// Version information (injected via ldflags at build time)
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Parse command line flags
	var (
		backendType  = flag.String("backend", "", "Storage backend (memory, postgres)")
		postgresURL  = flag.String("postgres-url", "", "PostgreSQL connection URL")
		distributed  = flag.Bool("distributed", false, "Enable distributed mode")
		instanceID   = flag.String("instance-id", "", "Instance ID for distributed mode")
		socketPath   = flag.String("socket", "", "Unix socket path")
		tcpAddr      = flag.String("tcp", "", "TCP address to listen on")
		workflowsDir = flag.String("workflows-dir", "", "Directory for workflow files")
		tlsCert      = flag.String("tls-cert", "", "Path to TLS certificate file")
		tlsKey       = flag.String("tls-key", "", "Path to TLS private key file")
		allowRemote  = flag.Bool("allow-remote", false, "Allow binding to non-localhost addresses (SECURITY WARNING)")
		showVersion  = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("conductord %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	// Initialize structured logging from environment
	logger := log.New(log.FromEnv())
	slog.SetDefault(logger)

	// Load daemon configuration
	cfg, err := config.LoadDaemon("")
	if err != nil {
		logger.Error("Failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	// Apply CLI flag overrides
	if *backendType != "" {
		cfg.Daemon.Backend.Type = *backendType
	}
	if *postgresURL != "" {
		cfg.Daemon.Backend.Postgres.ConnectionString = *postgresURL
	}
	if *distributed {
		cfg.Daemon.Distributed.Enabled = true
	}
	if *instanceID != "" {
		cfg.Daemon.Distributed.InstanceID = *instanceID
	}
	if *socketPath != "" {
		cfg.Daemon.Listen.SocketPath = *socketPath
	}
	if *tcpAddr != "" {
		cfg.Daemon.Listen.TCPAddr = *tcpAddr
	}
	if *workflowsDir != "" {
		cfg.Daemon.WorkflowsDir = *workflowsDir
	}
	if *tlsCert != "" {
		cfg.Daemon.Listen.TLSCert = *tlsCert
	}
	if *tlsKey != "" {
		cfg.Daemon.Listen.TLSKey = *tlsKey
	}
	if *allowRemote {
		cfg.Daemon.Listen.AllowRemote = true
		logger.Warn("--allow-remote is enabled. The daemon will accept connections from any network address. Ensure you have proper authentication and TLS configured for production use.")
	}

	// Create daemon instance
	d, err := daemon.New(cfg, daemon.Options{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	})
	if err != nil {
		logger.Error("Failed to create daemon", slog.Any("error", err))
		os.Exit(1)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start daemon
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
		}
	case err := <-errCh:
		if err != nil {
			logger.Error("Daemon error", slog.Any("error", err))
			os.Exit(1)
		}
	}
}
