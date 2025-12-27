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

package diagnostics

import (
	"os"

	"github.com/spf13/cobra"
)

// NewCompletionCommand creates the completion command with subcommands
func NewCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Annotations: map[string]string{
			"group": "diagnostics",
		},
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for Conductor.

To load completions:

Bash:
  $ source <(conductor completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ conductor completion bash > /etc/bash_completion.d/conductor
  # macOS:
  $ conductor completion bash > $(brew --prefix)/etc/bash_completion.d/conductor

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ conductor completion zsh > "${fpath[1]}/_conductor"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ conductor completion fish | source

  # To load completions for each session, execute once:
  $ conductor completion fish > ~/.config/fish/completions/conductor.fish
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE:                  runCompletion,
	}

	return cmd
}

func runCompletion(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "bash":
		return cmd.Root().GenBashCompletion(os.Stdout)
	case "zsh":
		return cmd.Root().GenZshCompletion(os.Stdout)
	case "fish":
		return cmd.Root().GenFishCompletion(os.Stdout, true)
	}
	return nil
}
