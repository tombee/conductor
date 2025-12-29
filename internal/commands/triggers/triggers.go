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

package triggers

import (
	"github.com/spf13/cobra"
)

// NewTriggersCommand creates the triggers command with subcommands.
func NewTriggersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "triggers",
		Short: "Manage workflow triggers",
		Long: `Manage workflow triggers (webhooks, schedules, and API endpoints).

Triggers determine how workflows are invoked. This command allows you to add,
list, and remove triggers without manually editing config.yaml.

Note: Changes to triggers require restarting the controller to take effect.

Subcommands:
  add      - Add a new trigger
  list     - List all configured triggers
  remove   - Remove a trigger`,
		Annotations: map[string]string{
			"group": "controller",
		},
	}

	cmd.AddCommand(newAddCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newRemoveCommand())

	return cmd
}
