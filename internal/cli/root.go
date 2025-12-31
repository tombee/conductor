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

package cli

import (
	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// SetVersion sets the version information (called from main)
func SetVersion(v, c, b string) {
	shared.SetVersion(v, c, b)
}

// NewRootCommand creates the root Cobra command for Conductor
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conductor",
		Short: "Conductor - LLM workflow orchestration",
		Long: `Conductor is a command-line tool for orchestrating complex workflows
with Large Language Models. It provides a simple, declarative way to
define multi-step processes and execute them across different LLM providers.

Run 'conductor setup' to get started with interactive configuration.
Run 'conductor examples' to see available workflow examples.`,
		SilenceUsage:  true, // Don't show usage on errors
		SilenceErrors: true, // We handle errors ourselves for proper exit codes
	}

	// Get flag pointers from shared package
	verbose, quiet, json, config := shared.RegisterFlagPointers()

	// Add global flags
	cmd.PersistentFlags().BoolVarP(verbose, "verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().BoolVarP(quiet, "quiet", "q", false, "Suppress non-error output")
	cmd.PersistentFlags().BoolVar(json, "json", false, "Output in JSON format")
	cmd.PersistentFlags().StringVar(config, "config", "", "Path to config file (default: ~/.config/conductor/config.yaml)")

	return cmd
}

// GetVersion returns version information
func GetVersion() (string, string, string) {
	return shared.GetVersion()
}

// HandleExitError handles exit errors with proper exit codes
func HandleExitError(err error) {
	shared.HandleExitError(err)
}
