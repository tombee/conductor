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

func newAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new trigger",
		Long: `Add a new trigger configuration.

Subcommands:
  webhook   - Add a webhook trigger
  schedule  - Add a schedule trigger
  api       - Add an API trigger
  file      - Add a file watcher trigger`,
	}

	cmd.AddCommand(newAddWebhookCommand())
	cmd.AddCommand(newAddScheduleCommand())
	cmd.AddCommand(newAddAPICommand())
	cmd.AddCommand(newAddFileCommand())

	return cmd
}
