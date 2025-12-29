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
	"flag"
	"fmt"
	"os"

	"github.com/tombee/conductor/internal/cli"
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/commands/config"
	"github.com/tombee/conductor/internal/commands/daemon"
	"github.com/tombee/conductor/internal/commands/diagnostics"
	"github.com/tombee/conductor/internal/commands/docs"
	"github.com/tombee/conductor/internal/commands/endpoint"
	"github.com/tombee/conductor/internal/commands/management"
	"github.com/tombee/conductor/internal/commands/mcp"
	"github.com/tombee/conductor/internal/commands/mcpserver"
	"github.com/tombee/conductor/internal/commands/override"
	"github.com/tombee/conductor/internal/commands/run"
	"github.com/tombee/conductor/internal/commands/secrets"
	"github.com/tombee/conductor/internal/commands/security"
	"github.com/tombee/conductor/internal/commands/validate"
	versioncmd "github.com/tombee/conductor/internal/commands/version"
	"github.com/tombee/conductor/internal/commands/workflow"
	daemonpkg "github.com/tombee/conductor/internal/daemon"
)

// Version information (injected via ldflags at build time)
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Check for --daemon-child flag before any cobra processing
	// This flag indicates the binary was spawned by daemon start in background mode
	daemonChild := false
	var daemonFlags daemonChildFlags

	for i, arg := range os.Args[1:] {
		if arg == "--daemon-child" {
			daemonChild = true
			// Parse daemon flags
			daemonFlags = parseDaemonChildFlags(os.Args[i+1:])
			break
		}
	}

	if daemonChild {
		// Run as daemon child (background mode)
		runOpts := daemonpkg.RunOptions{
			Version:      version,
			Commit:       commit,
			BuildDate:    buildDate,
			BackendType:  daemonFlags.backend,
			PostgresURL:  daemonFlags.postgresURL,
			Distributed:  daemonFlags.distributed,
			InstanceID:   daemonFlags.instanceID,
			SocketPath:   daemonFlags.socket,
			TCPAddr:      daemonFlags.tcp,
			WorkflowsDir: daemonFlags.workflowsDir,
			TLSCert:      daemonFlags.tlsCert,
			TLSKey:       daemonFlags.tlsKey,
			AllowRemote:  daemonFlags.allowRemote,
		}

		if err := daemonpkg.Run(runOpts); err != nil {
			fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Normal CLI mode
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

	// MCP commands
	rootCmd.AddCommand(mcp.NewMCPCommand())
	rootCmd.AddCommand(mcpserver.NewCommand())

	// Management commands
	rootCmd.AddCommand(management.NewRunsCommand())
	rootCmd.AddCommand(management.NewEventsCommand())
	rootCmd.AddCommand(management.NewTracesCommand())
	rootCmd.AddCommand(management.NewCacheCommand())
	rootCmd.AddCommand(endpoint.NewCommand())

	// Configuration and security
	rootCmd.AddCommand(config.NewConfigCommand())
	rootCmd.AddCommand(secrets.NewCommand())
	rootCmd.AddCommand(security.NewCommand())
	rootCmd.AddCommand(override.NewCommand())

	// Diagnostics commands
	rootCmd.AddCommand(diagnostics.NewDoctorCommand())
	rootCmd.AddCommand(diagnostics.NewPingCommand())
	rootCmd.AddCommand(diagnostics.NewProvidersCommand())
	rootCmd.AddCommand(completion.NewCommand())

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

// daemonChildFlags holds flags for daemon child process.
type daemonChildFlags struct {
	backend      string
	postgresURL  string
	distributed  bool
	instanceID   string
	socket       string
	tcp          string
	workflowsDir string
	tlsCert      string
	tlsKey       string
	allowRemote  bool
}

// parseDaemonChildFlags parses daemon flags from command line.
// This uses standard flag package to parse flags after --daemon-child.
func parseDaemonChildFlags(args []string) daemonChildFlags {
	var flags daemonChildFlags

	fs := flag.NewFlagSet("daemon-child", flag.ContinueOnError)
	fs.StringVar(&flags.backend, "backend", "", "Storage backend")
	fs.StringVar(&flags.postgresURL, "postgres-url", "", "PostgreSQL URL")
	fs.BoolVar(&flags.distributed, "distributed", false, "Distributed mode")
	fs.StringVar(&flags.instanceID, "instance-id", "", "Instance ID")
	fs.StringVar(&flags.socket, "socket", "", "Unix socket path")
	fs.StringVar(&flags.tcp, "tcp", "", "TCP address")
	fs.StringVar(&flags.workflowsDir, "workflows-dir", "", "Workflows directory")
	fs.StringVar(&flags.tlsCert, "tls-cert", "", "TLS certificate")
	fs.StringVar(&flags.tlsKey, "tls-key", "", "TLS key")
	fs.BoolVar(&flags.allowRemote, "allow-remote", false, "Allow remote")

	// Ignore parse errors - just extract what we can
	_ = fs.Parse(args)

	return flags
}
