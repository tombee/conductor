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

package completion

import (
	"os"

	"github.com/spf13/cobra"
)

// NewCommand creates the completion command for generating shell completion scripts.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "completion [bash|zsh|fish|powershell]",
		Annotations: map[string]string{
			"group": "diagnostics",
		},
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for Conductor.

To load completions:

Bash:
  # To load completions for the current session:
  $ source <(conductor completion bash)

  # To load completions for each session, save to a completions directory:
  # Linux (system-wide, requires root):
  $ conductor completion bash > /etc/bash_completion.d/conductor
  # Linux (user-local):
  $ mkdir -p ~/.local/share/bash-completion/completions
  $ conductor completion bash > ~/.local/share/bash-completion/completions/conductor
  # macOS (with Homebrew):
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

PowerShell:
  # To load completions for the current session:
  conductor completion powershell | Out-String | Invoke-Expression

  # To load completions for each session, save to a file and source it:
  # Create completions directory if it doesn't exist:
  New-Item -ItemType Directory -Force -Path "$HOME/.config/powershell/completions"
  conductor completion powershell > "$HOME/.config/powershell/completions/conductor.ps1"

  # Then add this line to your $PROFILE (once):
  Get-ChildItem "$HOME/.config/powershell/completions/*.ps1" | ForEach-Object { . $_ }
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
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
	case "powershell":
		return cmd.Root().GenPowerShellCompletion(os.Stdout)
	}
	return nil
}
