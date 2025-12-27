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
	"github.com/tombee/conductor/internal/cli"
	"github.com/tombee/conductor/internal/commands/config"
	"github.com/tombee/conductor/internal/commands/daemon"
	"github.com/tombee/conductor/internal/commands/diagnostics"
	"github.com/tombee/conductor/internal/commands/docs"
	"github.com/tombee/conductor/internal/commands/endpoint"
	"github.com/tombee/conductor/internal/commands/management"
	"github.com/tombee/conductor/internal/commands/mcp"
	"github.com/tombee/conductor/internal/commands/mcpserver"
	"github.com/tombee/conductor/internal/commands/run"
	"github.com/tombee/conductor/internal/commands/secrets"
	"github.com/tombee/conductor/internal/commands/security"
	"github.com/tombee/conductor/internal/commands/validate"
	versioncmd "github.com/tombee/conductor/internal/commands/version"
	"github.com/tombee/conductor/internal/commands/workflow"
)

// Version information (injected via ldflags at build time)
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Set version information from build-time ldflags
	cli.SetVersion(version, commit, buildDate)

	// Create root command and add subcommands
	rootCmd := cli.NewRootCommand()

	// Core workflow commands
	rootCmd.AddCommand(run.NewCommand())
	rootCmd.AddCommand(validate.NewCommand())

	// Workflow management commands
	rootCmd.AddCommand(workflow.NewInitCommand())
	rootCmd.AddCommand(workflow.NewQuickstartCommand())
	rootCmd.AddCommand(workflow.NewExamplesCommand())
	rootCmd.AddCommand(workflow.NewSchemaCommand())
	rootCmd.AddCommand(workflow.NewCostsCommand())

	// Daemon commands
	rootCmd.AddCommand(daemon.NewCommand())
	rootCmd.AddCommand(daemon.NewServeCommand())

	// MCP commands
	rootCmd.AddCommand(mcp.NewMCPCommand())
	rootCmd.AddCommand(mcpserver.NewCommand())

	// Management commands
	rootCmd.AddCommand(management.NewRunsCommand())
	rootCmd.AddCommand(management.NewEventsCommand())
	rootCmd.AddCommand(management.NewTracesCommand())
	rootCmd.AddCommand(management.NewCacheCommand())
	rootCmd.AddCommand(management.NewConnectorCommand())
	rootCmd.AddCommand(management.NewConnectorsCommand())
	rootCmd.AddCommand(endpoint.NewCommand())

	// Configuration and security
	rootCmd.AddCommand(config.NewConfigCommand())
	rootCmd.AddCommand(secrets.NewCommand())
	rootCmd.AddCommand(security.NewCommand())

	// Diagnostics commands
	rootCmd.AddCommand(diagnostics.NewDoctorCommand())
	rootCmd.AddCommand(diagnostics.NewPingCommand())
	rootCmd.AddCommand(diagnostics.NewProvidersCommand())
	rootCmd.AddCommand(diagnostics.NewCompletionCommand())

	// Documentation command
	rootCmd.AddCommand(docs.NewDocsCommand())

	// Version command
	rootCmd.AddCommand(versioncmd.NewVersionCommand())

	// Custom help command with JSON support
	rootCmd.SetHelpCommand(cli.NewHelpCommand(rootCmd))

	// Execute root command
	if err := rootCmd.Execute(); err != nil {
		cli.HandleExitError(err)
	}
}
