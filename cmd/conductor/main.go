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
	"github.com/tombee/conductor/internal/commands/controller"
	"github.com/tombee/conductor/internal/commands/debug"
	"github.com/tombee/conductor/internal/commands/diagnostics"
	"github.com/tombee/conductor/internal/commands/docs"
	"github.com/tombee/conductor/internal/commands/integrations"
	"github.com/tombee/conductor/internal/commands/management"
	"github.com/tombee/conductor/internal/commands/mcp"
	"github.com/tombee/conductor/internal/commands/mcpserver"
	"github.com/tombee/conductor/internal/commands/model"
	"github.com/tombee/conductor/internal/commands/provider"
	"github.com/tombee/conductor/internal/commands/run"
	"github.com/tombee/conductor/internal/commands/secrets"
	"github.com/tombee/conductor/internal/commands/test"
	"github.com/tombee/conductor/internal/commands/triggers"
	"github.com/tombee/conductor/internal/commands/validate"
	versioncmd "github.com/tombee/conductor/internal/commands/version"
	"github.com/tombee/conductor/internal/commands/workflow"
	workspacecmd "github.com/tombee/conductor/internal/commands/workspace"
	controllerpkg "github.com/tombee/conductor/internal/controller"
)

// Version information (injected via ldflags at build time)
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Check for --controller-child flag before any cobra processing
	// This flag indicates the binary was spawned by controller start in background mode
	controllerChild := false
	var controllerFlags controllerChildFlags

	for i, arg := range os.Args[1:] {
		if arg == "--controller-child" {
			controllerChild = true
			// Parse controller flags
			controllerFlags = parseControllerChildFlags(os.Args[i+1:])
			break
		}
	}

	if controllerChild {
		// Run as controller child (background mode)
		runOpts := controllerpkg.RunOptions{
			Version:      version,
			Commit:       commit,
			BuildDate:    buildDate,
			BackendType:  controllerFlags.backend,
			PostgresURL:  controllerFlags.postgresURL,
			Distributed:  controllerFlags.distributed,
			InstanceID:   controllerFlags.instanceID,
			SocketPath:   controllerFlags.socket,
			TCPAddr:      controllerFlags.tcp,
			WorkflowsDir: controllerFlags.workflowsDir,
			TLSCert:      controllerFlags.tlsCert,
			TLSKey:       controllerFlags.tlsKey,
			AllowRemote:  controllerFlags.allowRemote,
		}

		if err := controllerpkg.Run(runOpts); err != nil {
			fmt.Fprintf(os.Stderr, "Controller error: %v\n", err)
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
	rootCmd.AddCommand(test.NewCommand())

	// Workflow management commands
	rootCmd.AddCommand(workflow.NewExamplesCommand())
	rootCmd.AddCommand(workflow.NewSchemaCommand())
	rootCmd.AddCommand(workflow.NewUsageCommand())

	// Controller commands
	rootCmd.AddCommand(controller.NewCommand())

	// MCP commands
	rootCmd.AddCommand(mcp.NewMCPCommand())
	rootCmd.AddCommand(mcpserver.NewCommand())

	// Management commands
	rootCmd.AddCommand(management.NewHistoryCommand())
	rootCmd.AddCommand(triggers.NewTriggersCommand())

	// Debug commands
	rootCmd.AddCommand(debug.NewDebugCommand())

	// Configuration and security
	rootCmd.AddCommand(config.NewConfigCommand())
	rootCmd.AddCommand(integrations.NewCommand())
	rootCmd.AddCommand(workspacecmd.NewCommand())
	rootCmd.AddCommand(secrets.NewCommand())
	rootCmd.AddCommand(provider.NewCommand())
	rootCmd.AddCommand(model.NewCommand())

	// Diagnostics commands
	rootCmd.AddCommand(diagnostics.NewHealthCommand())
	rootCmd.AddCommand(diagnostics.NewPingCommand())
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

// controllerChildFlags holds flags for controller child process.
type controllerChildFlags struct {
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

// parseControllerChildFlags parses controller flags from command line.
// This uses standard flag package to parse flags after --controller-child.
func parseControllerChildFlags(args []string) controllerChildFlags {
	var flags controllerChildFlags

	fs := flag.NewFlagSet("controller-child", flag.ContinueOnError)
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
