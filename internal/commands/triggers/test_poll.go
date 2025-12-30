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
	"github.com/tombee/conductor/internal/controller/polltrigger"
)

// newTestPollCommand creates the command to test poll triggers.
func newTestPollCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <name>",
		Short: "Test a poll trigger (dry-run)",
		Long: `Test a poll trigger by executing a dry-run poll.

This command polls the integration API and shows what events would trigger
workflows, without actually updating the trigger state or firing workflows.
Useful for debugging trigger configuration and seeing what events would match.`,
		Example: `  # Test a poll trigger named "pagerduty-oncall"
  conductor triggers test pagerduty-oncall

  # Get test results as JSON
  conductor triggers test pagerduty-oncall --json`,
		Args: cobra.ExactArgs(1),
		RunE: runTestPoll,
	}

	return cmd
}

func runTestPoll(cmd *cobra.Command, args []string) error {
	triggerName := args[0]

	// Get state manager (read-only for dry-run)
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

	// Get current state (or create empty state for new triggers)
	state, err := stateManager.GetState(ctx, pollTrigger.TriggerID)
	if err != nil {
		return fmt.Errorf("failed to get trigger state: %w", err)
	}

	if state == nil {
		// Create a temporary state for testing
		state = &polltrigger.PollState{
			TriggerID:    pollTrigger.TriggerID,
			WorkflowPath: pollTrigger.WorkflowPath,
			Integration:  pollTrigger.Integration,
			SeenEvents:   make(map[string]int64),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		// For new triggers, use a reasonable default for LastPollTime
		// (e.g., last hour to avoid flooding with historical events)
		state.LastPollTime = time.Now().Add(-1 * time.Hour)
	}

	// Get the poller for this integration
	poller, err := getPoller(pollTrigger.Integration)
	if err != nil {
		return fmt.Errorf("failed to get poller: %w", err)
	}

	// Execute the poll (dry-run - don't update state)
	if !shared.GetJSON() {
		fmt.Fprintf(cmd.OutOrStdout(), "Testing poll trigger: %s\n", triggerName)
		fmt.Fprintf(cmd.OutOrStdout(), "Integration: %s\n", pollTrigger.Integration)
		fmt.Fprintf(cmd.OutOrStdout(), "Polling since: %s\n\n", state.LastPollTime.Format(time.RFC3339))
	}

	events, cursor, err := poller.Poll(ctx, state, pollTrigger.Query)
	if err != nil {
		return fmt.Errorf("poll failed: %w", err)
	}

	// Filter out events we've already seen
	var newEvents []map[string]interface{}
	for _, event := range events {
		eventID, ok := event["id"].(string)
		if !ok {
			continue
		}

		if _, seen := state.SeenEvents[eventID]; !seen {
			newEvents = append(newEvents, event)
		}
	}

	// Output results
	if shared.GetJSON() {
		result := map[string]interface{}{
			"trigger":     triggerName,
			"integration": pollTrigger.Integration,
			"poll_time":   time.Now(),
			"total_events": len(events),
			"new_events":   len(newEvents),
			"cursor":       cursor,
			"events":       newEvents,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Print human-readable output
	fmt.Fprintf(cmd.OutOrStdout(), "Poll Results:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Total events returned: %d\n", len(events))
	fmt.Fprintf(cmd.OutOrStdout(), "  New events (not seen): %d\n", len(newEvents))
	if cursor != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Pagination cursor: %s\n", cursor)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	if len(newEvents) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No new events would trigger workflows.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Events that would trigger workflows:")
	for i, event := range newEvents {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Event %d:\n", i+1)

		// Show common fields
		if id, ok := event["id"]; ok {
			fmt.Fprintf(cmd.OutOrStdout(), "    ID: %v\n", id)
		}
		if ts, ok := event["timestamp"]; ok {
			fmt.Fprintf(cmd.OutOrStdout(), "    Timestamp: %v\n", ts)
		}
		if title, ok := event["title"]; ok {
			fmt.Fprintf(cmd.OutOrStdout(), "    Title: %v\n", title)
		}
		if summary, ok := event["summary"]; ok {
			fmt.Fprintf(cmd.OutOrStdout(), "    Summary: %v\n", summary)
		}

		// Show full event in verbose mode
		if shared.GetVerbose() {
			eventJSON, _ := json.MarshalIndent(event, "    ", "  ")
			fmt.Fprintf(cmd.OutOrStdout(), "    Full event:\n%s\n", string(eventJSON))
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Note: This is a dry-run. No state was updated and no workflows were fired.")

	return nil
}

// findPollTrigger finds a poll trigger by name.
func findPollTrigger(name string) (*polltrigger.PollTriggerRegistration, error) {
	triggers, err := listPollTriggers()
	if err != nil {
		return nil, err
	}

	for _, t := range triggers {
		if t.TriggerID == name {
			return t, nil
		}
	}

	return nil, nil
}

// getPoller gets the appropriate poller for an integration.
func getPoller(integration string) (polltrigger.IntegrationPoller, error) {
	// This will be implemented when we integrate with the actual pollers
	// For now, return an error
	return nil, fmt.Errorf("poller for integration %q not yet available in CLI context", integration)
}
