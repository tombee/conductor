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
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured triggers",
		Long: `List all configured triggers (webhooks, schedules, and API endpoints).

Shows triggers currently defined in the config file. Note that changes require
a controller restart to take effect.`,
		Example: `  # List all triggers in table format
  conductor triggers list

  # Get triggers as JSON for scripting
  conductor triggers list --json`,
		RunE: runList,
	}

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Get all triggers
	webhooks, err := mgr.ListWebhooks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}

	schedules, err := mgr.ListSchedules(ctx)
	if err != nil {
		return fmt.Errorf("failed to list schedules: %w", err)
	}

	endpoints, err := mgr.ListEndpoints(ctx)
	if err != nil {
		return fmt.Errorf("failed to list endpoints: %w", err)
	}

	if shared.GetJSON() {
		output := map[string]any{
			"webhooks":  webhooks,
			"schedules": schedules,
			"endpoints": endpoints,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	// Print in tabular format
	fmt.Fprintln(cmd.OutOrStdout(), "Webhooks:")
	if len(webhooks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, wh := range webhooks {
			events := "(all)"
			if len(wh.Events) > 0 {
				events = strings.Join(wh.Events, ", ")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s -> %s [%s] events=%s\n",
				wh.Path, wh.Workflow, wh.Source, events)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Schedules:")
	if len(schedules) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, sch := range schedules {
			tz := sch.Timezone
			if tz == "" {
				tz = "UTC"
			}
			status := "enabled"
			if !sch.Enabled {
				status = "disabled"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s (%s) -> %s [%s]\n",
				sch.Name, sch.Cron, tz, sch.Workflow, status)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Endpoints:")
	if len(endpoints) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, ep := range endpoints {
			desc := ep.Description
			if desc == "" {
				desc = "(no description)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s -> %s: %s\n",
				ep.Name, ep.Workflow, desc)
		}
	}

	return nil
}
