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
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// newShowPollCommand creates the command to show poll trigger details.
func newShowPollCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show detailed poll trigger information",
		Long: `Show detailed information about a poll trigger.

Displays the trigger configuration, current state, last poll time, error count,
and other diagnostic information. Useful for debugging poll trigger issues.`,
		Example: `  # Show details for a poll trigger
  conductor triggers show pagerduty-oncall

  # Get details as JSON
  conductor triggers show pagerduty-oncall --json`,
		Args: cobra.ExactArgs(1),
		RunE: runShowPoll,
	}

	return cmd
}

func runShowPoll(cmd *cobra.Command, args []string) error {
	triggerName := args[0]

	// Get state manager
	stateManager, err := getPollStateManager()
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}
	defer stateManager.Close()

	ctx := context.Background()

	// Find the poll trigger configuration
	pollTrigger, err := findPollTrigger(triggerName)
	if err != nil {
		return fmt.Errorf("failed to find poll trigger: %w", err)
	}
	if pollTrigger == nil {
		return fmt.Errorf("poll trigger %q not found", triggerName)
	}

	// Get current state
	state, err := stateManager.GetState(ctx, pollTrigger.TriggerID)
	if err != nil {
		return fmt.Errorf("failed to get trigger state: %w", err)
	}

	// Build output structure
	output := map[string]interface{}{
		"name":        pollTrigger.TriggerID,
		"workflow":    pollTrigger.WorkflowPath,
		"integration": pollTrigger.Integration,
		"interval":    fmt.Sprintf("%ds", pollTrigger.Interval),
		"query":       pollTrigger.Query,
		"startup":     pollTrigger.Startup,
	}

	if pollTrigger.Backfill > 0 {
		output["backfill"] = fmt.Sprintf("%ds", pollTrigger.Backfill)
	}

	if pollTrigger.InputMapping != nil && len(pollTrigger.InputMapping) > 0 {
		output["input_mapping"] = pollTrigger.InputMapping
	}

	// Add state information
	if state != nil {
		output["state"] = map[string]interface{}{
			"last_poll_time":  formatTime(state.LastPollTime),
			"high_water_mark": formatTime(state.HighWaterMark),
			"seen_events":     len(state.SeenEvents),
			"cursor":          state.Cursor,
			"error_count":     state.ErrorCount,
			"last_error":      state.LastError,
			"created_at":      state.CreatedAt.Format(time.RFC3339),
			"updated_at":      state.UpdatedAt.Format(time.RFC3339),
		}

		// Determine status
		if state.ErrorCount >= 10 {
			output["status"] = "paused"
		} else if state.ErrorCount > 0 {
			output["status"] = "degraded"
		} else {
			output["status"] = "healthy"
		}
	} else {
		output["state"] = nil
		output["status"] = "new"
	}

	// Output results
	if shared.GetJSON() {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	// Print human-readable output
	fmt.Fprintf(cmd.OutOrStdout(), "Poll Trigger: %s\n\n", pollTrigger.TriggerID)

	fmt.Fprintln(cmd.OutOrStdout(), "Configuration:")
	fmt.Fprintf(cmd.OutOrStdout(), "  Workflow:    %s\n", pollTrigger.WorkflowPath)
	fmt.Fprintf(cmd.OutOrStdout(), "  Integration: %s\n", pollTrigger.Integration)
	fmt.Fprintf(cmd.OutOrStdout(), "  Interval:    %ds\n", pollTrigger.Interval)
	fmt.Fprintf(cmd.OutOrStdout(), "  Startup:     %s\n", pollTrigger.Startup)
	if pollTrigger.Backfill > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Backfill:    %ds\n", pollTrigger.Backfill)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Query Parameters:")
	if len(pollTrigger.Query) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for k, v := range pollTrigger.Query {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v\n", k, v)
		}
	}

	if pollTrigger.InputMapping != nil && len(pollTrigger.InputMapping) > 0 {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Input Mapping:")
		for k, v := range pollTrigger.InputMapping {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", k, v)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	if state != nil {
		status := output["status"].(string)
		var statusDisplay string
		switch status {
		case "healthy":
			statusDisplay = shared.RenderOK("healthy")
		case "degraded":
			statusDisplay = shared.RenderWarn("degraded")
		case "paused":
			statusDisplay = shared.RenderError("paused")
		default:
			statusDisplay = status
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", shared.Muted.Render("Status:"), statusDisplay)
		fmt.Fprintln(cmd.OutOrStdout())

		fmt.Fprintln(cmd.OutOrStdout(), "State:")
		fmt.Fprintf(cmd.OutOrStdout(), "  Last poll:       %s\n", formatTimeWithRelative(state.LastPollTime))
		fmt.Fprintf(cmd.OutOrStdout(), "  High water mark: %s\n", formatTimeWithRelative(state.HighWaterMark))
		fmt.Fprintf(cmd.OutOrStdout(), "  Seen events:     %d\n", len(state.SeenEvents))
		if state.Cursor != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Cursor:          %s\n", state.Cursor)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Error count:     %d\n", state.ErrorCount)
		if state.LastError != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Last error:      %s\n", state.LastError)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:         %s\n", state.CreatedAt.Format(time.RFC3339))
		fmt.Fprintf(cmd.OutOrStdout(), "  Updated:         %s\n", formatTimeWithRelative(state.UpdatedAt))

		// Show recommendations based on state
		fmt.Fprintln(cmd.OutOrStdout())
		if state.ErrorCount >= 10 {
			fmt.Fprintln(cmd.OutOrStdout(), shared.RenderWarn("Trigger is PAUSED due to repeated failures."))
			fmt.Fprintln(cmd.OutOrStdout(), shared.Muted.Render("  Fix the underlying issue and run:"))
			fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", shared.StatusInfo.Render(fmt.Sprintf("conductor triggers reset %s", triggerName)))
		} else if state.ErrorCount > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), shared.RenderWarn(fmt.Sprintf("Trigger has %d consecutive error(s). Will pause after 10.", state.ErrorCount)))
			if state.LastError != "" {
				fmt.Fprintln(cmd.OutOrStdout(), shared.Muted.Render("  Check the error message above and verify integration credentials."))
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), shared.RenderOK("Trigger is operating normally."))
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Status: new (no state yet)")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "This trigger has not executed yet. Wait for the controller to start")
		fmt.Fprintln(cmd.OutOrStdout(), "and execute the first poll, or run:")
		fmt.Fprintf(cmd.OutOrStdout(), "  conductor triggers test %s\n", triggerName)
	}

	return nil
}

// formatTimeWithRelative formats a time with both absolute and relative values.
func formatTimeWithRelative(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	abs := t.Format(time.RFC3339)
	rel := formatRelativeTime(t)

	if rel == "just now" {
		return abs + " (just now)"
	}

	return fmt.Sprintf("%s (%s)", abs, rel)
}
