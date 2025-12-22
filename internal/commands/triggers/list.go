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
		Long: `List all configured triggers (webhooks, schedules, poll triggers, API triggers, and file watchers).

Shows triggers currently defined in the config file. Note that changes require
a controller restart to take effect.`,
		Example: `  # List all triggers in table format
  conductor triggers list

  # List only poll triggers
  conductor triggers list --type poll

  # Get triggers as JSON for scripting
  conductor triggers list --json`,
		RunE: runList,
	}

	cmd.Flags().String("type", "", "Filter by trigger type (webhook, schedule, poll, api)")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	typeFilter, _ := cmd.Flags().GetString("type")

	// If filtering for poll triggers only, delegate to list-poll logic
	if typeFilter == "poll" {
		return runListPollOnly(cmd)
	}

	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Determine which trigger types to fetch based on filter
	showWebhooks := typeFilter == "" || typeFilter == "webhook"
	showSchedules := typeFilter == "" || typeFilter == "schedule"
	showAPITriggers := typeFilter == "" || typeFilter == "api"

	// Validate filter if provided
	if typeFilter != "" && !showWebhooks && !showSchedules && !showAPITriggers {
		return fmt.Errorf("invalid trigger type %q (must be: webhook, schedule, poll, or api)", typeFilter)
	}

	// Get trigger lists
	webhooks, err := mgr.ListWebhooks(ctx)
	if err != nil && showWebhooks {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}

	schedules, err := mgr.ListSchedules(ctx)
	if err != nil && showSchedules {
		return fmt.Errorf("failed to list schedules: %w", err)
	}

	apiTriggers, err := mgr.ListEndpoints(ctx)
	if err != nil && showAPITriggers {
		return fmt.Errorf("failed to list API triggers: %w", err)
	}

	fileWatchers, err := mgr.ListFileWatchers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list file watchers: %w", err)
	}

	if shared.GetJSON() {
		output := map[string]any{}
		if showWebhooks {
			output["webhooks"] = webhooks
		}
		if showSchedules {
			output["schedules"] = schedules
		}
		if showAPITriggers {
			output["api_triggers"] = apiTriggers
		}
		output["file_watchers"] = fileWatchers
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	// Print in tabular format with colors
	out := cmd.OutOrStdout()

	if showWebhooks {
		fmt.Fprintln(out, shared.Bold.Render("Webhooks:"))
		if len(webhooks) == 0 {
			fmt.Fprintln(out, shared.Muted.Render("  (none)"))
		} else {
			for _, wh := range webhooks {
				events := shared.Muted.Render("(all)")
				if len(wh.Events) > 0 {
					events = strings.Join(wh.Events, ", ")
				}
				fmt.Fprintf(out, "  %s %s -> %s %s events=%s\n",
					shared.StatusOK.Render(shared.SymbolOK),
					shared.Bold.Render(wh.Path),
					wh.Workflow,
					shared.Muted.Render("["+wh.Source+"]"),
					events)
			}
		}
		fmt.Fprintln(out)
	}

	if showSchedules {
		fmt.Fprintln(out, shared.Bold.Render("Schedules:"))
		if len(schedules) == 0 {
			fmt.Fprintln(out, shared.Muted.Render("  (none)"))
		} else {
			for _, sch := range schedules {
				tz := sch.Timezone
				if tz == "" {
					tz = "UTC"
				}
				var statusStyled string
				if sch.Enabled {
					statusStyled = shared.StatusOK.Render("enabled")
				} else {
					statusStyled = shared.Muted.Render("disabled")
				}
				fmt.Fprintf(out, "  %s %s: %s %s -> %s [%s]\n",
					shared.StatusOK.Render(shared.SymbolOK),
					shared.Bold.Render(sch.Name),
					sch.Cron,
					shared.Muted.Render("("+tz+")"),
					sch.Workflow,
					statusStyled)
			}
		}
		fmt.Fprintln(out)
	}

	if showAPITriggers {
		fmt.Fprintln(out, shared.Bold.Render("API Triggers:"))
		if len(apiTriggers) == 0 {
			fmt.Fprintln(out, shared.Muted.Render("  (none)"))
		} else {
			for _, trigger := range apiTriggers {
				desc := trigger.Description
				if desc == "" {
					desc = shared.Muted.Render("(no description)")
				}
				fmt.Fprintf(out, "  %s %s -> %s: %s\n",
					shared.StatusOK.Render(shared.SymbolOK),
					shared.Bold.Render(trigger.Name),
					trigger.Workflow,
					desc)
			}
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, shared.Bold.Render("File Watchers:"))
	if len(fileWatchers) == 0 {
		fmt.Fprintln(out, shared.Muted.Render("  (none)"))
	} else {
		for _, fw := range fileWatchers {
			events := shared.Muted.Render("(all)")
			if len(fw.Events) > 0 {
				events = strings.Join(fw.Events, ", ")
			}
			var statusStyled string
			if fw.Enabled {
				statusStyled = shared.StatusOK.Render("enabled")
			} else {
				statusStyled = shared.Muted.Render("disabled")
			}
			path := fw.Path
			if len(fw.IncludePatterns) > 0 {
				path = fmt.Sprintf("%s %s", path, shared.Muted.Render("[include: "+strings.Join(fw.IncludePatterns, ", ")+"]"))
			}
			fmt.Fprintf(out, "  %s %s: %s -> %s events=%s [%s]\n",
				shared.StatusOK.Render(shared.SymbolOK),
				shared.Bold.Render(fw.Name),
				path,
				fw.Workflow,
				events,
				statusStyled)
		}
	}

	return nil
}

// runListPollOnly is a helper that runs the poll trigger listing logic.
func runListPollOnly(cmd *cobra.Command) error {
	return runListPoll(cmd, nil)
}
