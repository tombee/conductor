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

package workflow

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewInitCommand creates the init command
func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [name]",
		Annotations: map[string]string{
			"group": "workflow",
		},
		Short: "Initialize Conductor or create a new workflow",
		Long: `Initialize Conductor or create a new workflow from a template.

Without arguments: Runs the setup wizard to configure Conductor providers.
With a name argument: Creates a new workflow file from a template.

Examples:
  conductor init                       # Run setup wizard
  conductor init my-workflow           # Create my-workflow/workflow.yaml
  conductor init --file review.yaml    # Create single file in current directory
  conductor init --template code-review my-review  # Use code-review template
  conductor init --list                # List available templates`,
		RunE: runInit,
		Args: cobra.MaximumNArgs(1),
	}

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(os.Stderr, "Error: 'conductor init' has been replaced")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Use these commands instead:")
	fmt.Fprintln(os.Stderr, "  conductor setup              # Configure Conductor (providers, integrations, secrets)")
	fmt.Fprintln(os.Stderr, "")

	return &shared.ExitError{Code: 1, Message: ""}
}
