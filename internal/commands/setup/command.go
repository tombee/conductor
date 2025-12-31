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

package setup

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCommand creates the setup command
func NewCommand() *cobra.Command {
	var accessible bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive wizard to configure Conductor",
		Long: `Launch the interactive setup wizard to configure:
  - LLM providers (Claude Code, Ollama, Anthropic, OpenAI-compatible)
  - Secrets management (keychain, environment variables, file)
  - Integrations (GitHub, Slack, Jira, Discord, Jenkins)

The wizard provides a TUI (Terminal User Interface) for guided configuration.`,
		Annotations: map[string]string{
			"group": "config",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd, accessible)
		},
	}

	cmd.Flags().BoolVar(&accessible, "accessible", false, "Use accessible mode (simple text prompts instead of TUI)")

	return cmd
}

// runSetup executes the setup wizard
func runSetup(cmd *cobra.Command, accessible bool) error {
	// TODO: Implement setup wizard
	return fmt.Errorf("setup wizard not yet implemented")
}
