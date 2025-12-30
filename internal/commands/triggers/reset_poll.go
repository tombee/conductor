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
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// newResetPollCommand creates the command to reset poll trigger state.
func newResetPollCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset <name>",
		Short: "Reset poll trigger state",
		Long: `Reset the state of a poll trigger.

This clears the last poll time, seen events, and error count for the trigger,
allowing it to re-process historical events on the next poll. This is useful
for recovering from errors or re-processing events after fixing trigger
configuration.

Warning: Resetting state may cause duplicate workflow executions for events
that were previously processed.`,
		Example: `  # Reset state for a poll trigger
  conductor triggers reset pagerduty-oncall

  # Reset and confirm
  conductor triggers reset slack-mentions --yes`,
		Args: cobra.ExactArgs(1),
		RunE: runResetPoll,
	}

	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}

func runResetPoll(cmd *cobra.Command, args []string) error {
	triggerName := args[0]

	// Get confirmation flag
	yes, _ := cmd.Flags().GetBool("yes")

	// Get state manager
	stateManager, err := getPollStateManager()
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}
	defer stateManager.Close()

	ctx := context.Background()

	// Verify trigger exists
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

	if state == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Trigger %q has no state to reset.\n", triggerName)
		return nil
	}

	// Show current state and confirm
	if !yes {
		fmt.Fprintf(cmd.OutOrStdout(), "Current state for %q:\n", triggerName)
		fmt.Fprintf(cmd.OutOrStdout(), "  Last poll: %s\n", formatTime(state.LastPollTime))
		fmt.Fprintf(cmd.OutOrStdout(), "  Seen events: %d\n", len(state.SeenEvents))
		fmt.Fprintf(cmd.OutOrStdout(), "  Error count: %d\n", state.ErrorCount)
		if state.LastError != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Last error: %s\n", state.LastError)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "Reset will clear all state and allow re-processing of historical events.\n")
		fmt.Fprintf(cmd.OutOrStdout(), "This may cause duplicate workflow executions.\n\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Continue? [y/N]: ")

		var response string
		fmt.Fscanln(cmd.InOrStdin(), &response)
		if response != "y" && response != "Y" && response != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Reset cancelled.")
			return nil
		}
	}

	// Reset the state
	state.LastPollTime = time.Time{}
	state.HighWaterMark = time.Time{}
	state.SeenEvents = make(map[string]int64)
	state.Cursor = ""
	state.LastError = ""
	state.ErrorCount = 0
	state.UpdatedAt = time.Now()

	if err := stateManager.SaveState(ctx, state); err != nil {
		return fmt.Errorf("failed to save reset state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Successfully reset state for poll trigger %q\n", triggerName)
	fmt.Fprintln(cmd.OutOrStdout(), "The trigger will re-process events on the next poll cycle.")

	return nil
}

// formatTime formats a time value for display.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(time.RFC3339)
}
