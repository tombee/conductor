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

package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/log"
	"github.com/tombee/conductor/internal/rpc"
)

// Serve command flags
var (
	servePort int
)

// NewServeCommand creates the serve command
func NewServeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Conductor RPC server",
		Long: `Start the Conductor RPC server to accept workflow execution requests.

The server listens for WebSocket connections and processes workflow
execution requests from clients.`,
		Example: `  # Start server with default settings
  conductor serve

  # Start server with specific port
  conductor serve --port 8080`,
		RunE: runServe,
	}

	cmd.Flags().IntVar(&servePort, "port", 0, "Port to bind to (default: 9876)")

	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(shared.GetConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override port if specified
	if servePort != 0 {
		cfg.Server.Port = servePort
	}

	// Create logger from configuration
	logConfig := &log.Config{
		Level:     cfg.Log.Level,
		Format:    log.Format(cfg.Log.Format),
		Output:    os.Stderr,
		AddSource: cfg.Log.AddSource,
	}
	logger := log.New(logConfig)

	v, _, _ := shared.GetVersion()
	logger.Info("conductor starting", "version", v)

	// Create RPC server with configuration
	serverConfig := rpc.DefaultConfig()
	serverConfig.Logger = logger
	serverConfig.Port = cfg.Server.Port
	serverConfig.ShutdownTimeout = cfg.Server.ShutdownTimeout

	server := rpc.NewServer(serverConfig)

	// Start server
	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	logger.Info("conductor ready",
		"port", port,
		"log_level", cfg.Log.Level,
		"log_format", cfg.Log.Format)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	logger.Info("shutting down")

	// Graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	logger.Info("shutdown complete")
	fmt.Println("Goodbye!")

	return nil
}
