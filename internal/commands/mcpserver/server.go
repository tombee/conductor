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

package mcpserver

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/mcp/server"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewCommand creates the mcp-server command
func NewCommand() *cobra.Command {
	var (
		logLevel string
	)

	cmd := &cobra.Command{
		Use:   "mcp-server",
		Short: "Start the Conductor MCP server",
		Long: `Start the Conductor MCP (Model Context Protocol) server.

The MCP server exposes Conductor functionality as tools that AI coding assistants
(Claude Code, Cursor, Gemini CLI) can use to validate workflows, run them, list
templates, and check configuration health.

The server runs in stdio mode by default, which is suitable for integration with
AI assistants via their MCP configuration.

Configuration example for Claude Code (~/.config/claude/config.json):
  {
    "mcpServers": {
      "conductor": {
        "command": "conductor",
        "args": ["mcp-server"]
      }
    }
  }

The server exposes these tools:
  - conductor_validate: Validate workflow YAML
  - conductor_schema: Get JSON Schema
  - conductor_list_templates: List available templates
  - conductor_scaffold: Generate workflow from template
  - conductor_run: Execute workflow (dry-run default)
  - conductor_doctor: Check installation health

For safety, the conductor_run tool defaults to dry_run=true. AI assistants must
explicitly set dry_run=false to execute workflows.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServer(cmd, logLevel)
		},
	}

	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Logging verbosity (debug, info, warn, error)")

	return cmd
}

func runMCPServer(cmd *cobra.Command, logLevel string) error {
	// Get version info
	versionStr, _, _ := shared.GetVersion()

	// Create server config
	config := server.ServerConfig{
		Name:     "conductor",
		Version:  versionStr,
		LogLevel: logLevel,
	}

	// Create the MCP server
	srv, err := server.NewServer(config)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start shutdown handler in background
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nReceived shutdown signal, shutting down gracefully...")

		// Create shutdown context with 5-second timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
		}

		cancel()
	}()

	// Run the server (blocks until shutdown)
	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("MCP server error: %w", err)
	}

	return nil
}
