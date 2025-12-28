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

package run

import (
	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewCommand creates the run command
func NewCommand() *cobra.Command {
	var (
		inputs                        []string
		inputFile                     string
		outputFile                    string
		noStats                       bool
		provider                      string
		model                         string
		timeout                       string
		dryRun                        bool
		quiet                         bool
		verbose                       bool
		daemon                        bool
		background                    bool
		mcpDev                        bool
		noCache                       bool
		noInteractive                 bool
		helpInputs                    bool
		securityMode                  string
		allowHosts                    []string
		allowPaths                    []string
		workspace                     string
		profile                       string
		acceptUnenforceablePermissions bool
	)

	cmd := &cobra.Command{
		Use:   "run <workflow>",
		Short: "Execute a workflow",
		Annotations: map[string]string{
			"group": "execution",
		},
		Long: `Run executes a Conductor workflow with provider resolution.

Provider Resolution Order:
  1. Agent mapping lookup (if step specifies 'agent')
  2. CONDUCTOR_PROVIDER environment variable
  3. default_provider from config
  4. Auto-detection fallback

Execution Modes:
  --daemon       Submit to conductord daemon for execution
  --background   Run asynchronously (implies --daemon), return run ID immediately

Profile Selection (SPEC-130):
  --workspace, -w <name>   Workspace for profile resolution (env: CONDUCTOR_WORKSPACE)
  --profile, -p <name>     Profile for binding resolution (env: CONDUCTOR_PROFILE)

  Selection Precedence: CLI flag > environment variable > default

Remote Workflows:
  conductor run github:user/repo              Run from GitHub repo
  conductor run github:user/repo@v1.0         Pin to specific tag
  conductor run github:user/repo@main         Pin to branch
  conductor run github:user/repo/path         Run from subdirectory
  --no-cache                                  Force fresh download (skip cache)

Verbosity levels:
  --verbose  Show full provider/model info for each step
  (default)  Show minimal progress updates
  --quiet    Suppress non-error output`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --json implies --no-interactive
			if shared.GetJSON() {
				noInteractive = true
			}

			if background {
				daemon = true // --background implies --daemon
			}
			if mcpDev {
				daemon = true // --mcp-dev requires daemon mode
			}
			if daemon {
				return runWorkflowViaDaemon(args[0], inputs, inputFile, outputFile, noStats, background, mcpDev, noCache, quiet, verbose, noInteractive, helpInputs, provider, model, timeout, workspace, profile)
			}
			return runWorkflowLocal(args[0], inputs, inputFile, dryRun, quiet, verbose, noInteractive, helpInputs, acceptUnenforceablePermissions)
		},
	}

	cmd.Flags().StringSliceVarP(&inputs, "input", "i", nil, "Workflow input in key=value format")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "JSON file with inputs (use '-' for stdin)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write output to file")
	cmd.Flags().BoolVar(&noStats, "no-stats", false, "Don't show cost/token statistics")
	cmd.Flags().StringVar(&provider, "provider", "", "Override default provider")
	cmd.Flags().StringVar(&model, "model", "", "Override model tier")
	cmd.Flags().StringVar(&timeout, "timeout", "", "Override step timeout")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show execution plan without running")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress all warnings")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed execution logs")
	cmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "Submit to daemon for execution")
	cmd.Flags().BoolVar(&background, "background", false, "Run asynchronously, return run ID immediately")
	cmd.Flags().BoolVar(&mcpDev, "mcp-dev", false, "Enable MCP development mode (auto-restart servers, debug output)")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Force fresh download of remote workflows (skip cache)")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "Disable interactive prompts for missing inputs")
	cmd.Flags().BoolVar(&helpInputs, "help-inputs", false, "List all workflow inputs without running")
	cmd.Flags().StringVar(&securityMode, "security", "", "Security profile to use (unrestricted, standard, strict, air-gapped)")
	cmd.Flags().StringSliceVar(&allowHosts, "allow-hosts", nil, "Additional allowed network hosts")
	cmd.Flags().StringSliceVar(&allowPaths, "allow-paths", nil, "Additional allowed filesystem paths")
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace for profile resolution (env: CONDUCTOR_WORKSPACE)")
	cmd.Flags().StringVarP(&profile, "profile", "p", "", "Profile for binding resolution (env: CONDUCTOR_PROFILE)")
	cmd.Flags().BoolVar(&acceptUnenforceablePermissions, "accept-unenforceable-permissions", false, "Allow workflow execution even if some permissions cannot be enforced by the provider (SPEC-141)")

	return cmd
}
