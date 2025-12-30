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

// newListPollCommand creates the command to list poll triggers.
func newListPollCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-poll",
		Short: "List poll triggers and their status",
		Long: `List all configured poll triggers with their current state.

Shows the name, integration, polling interval, status, and error information
for each poll trigger. This is useful for monitoring the health of poll-based
triggers.`,
		Example: `  # List all poll triggers in table format
  conductor triggers list-poll

  # Get poll triggers as JSON for scripting
  conductor triggers list-poll --json`,
		RunE: runListPoll,
	}

	return cmd
}

// PollTriggerInfo contains display information for a poll trigger.
type PollTriggerInfo struct {
	Name        string    `json:"name"`
	Integration string    `json:"integration"`
	Interval    string    `json:"interval"`
	Status      string    `json:"status"`
	LastPoll    time.Time `json:"last_poll,omitempty"`
	ErrorCount  int       `json:"error_count"`
	LastError   string    `json:"last_error,omitempty"`
}

func runListPoll(cmd *cobra.Command, args []string) error {
	// Get state manager to read poll trigger state
	stateManager, err := getPollStateManager()
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}
	defer stateManager.Close()

	ctx := context.Background()

	// Get all poll triggers from config
	pollTriggers, err := listPollTriggers()
	if err != nil {
		return fmt.Errorf("failed to list poll triggers: %w", err)
	}

	// Collect status information for each trigger
	var infos []PollTriggerInfo
	for _, pt := range pollTriggers {
		state, err := stateManager.GetState(ctx, pt.TriggerID)
		if err != nil {
			return fmt.Errorf("failed to get state for trigger %s: %w", pt.TriggerID, err)
		}

		info := PollTriggerInfo{
			Name:        pt.TriggerID,
			Integration: pt.Integration,
			Interval:    fmt.Sprintf("%ds", pt.Interval),
			ErrorCount:  0,
		}

		if state != nil {
			info.LastPoll = state.LastPollTime
			info.ErrorCount = state.ErrorCount
			info.LastError = state.LastError

			// Determine status
			if state.ErrorCount >= 10 {
				info.Status = "paused"
			} else if state.ErrorCount > 0 {
				info.Status = "degraded"
			} else {
				info.Status = "healthy"
			}
		} else {
			info.Status = "new"
		}

		infos = append(infos, info)
	}

	// Output results
	if shared.GetJSON() {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"poll_triggers": infos,
		})
	}

	// Print in tabular format
	if len(infos) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No poll triggers configured.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Poll Triggers:")
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-10s %-10s %-12s %s\n",
		"NAME", "INTEGRATION", "INTERVAL", "STATUS", "ERROR COUNT", "LAST POLL")
	fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-10s %-10s %-12s %s\n",
		"----", "-----------", "--------", "------", "-----------", "---------")

	for _, info := range infos {
		lastPoll := "-"
		if !info.LastPoll.IsZero() {
			lastPoll = formatRelativeTime(info.LastPoll)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-10s %-10s %-12d %s\n",
			truncate(info.Name, 30),
			truncate(info.Integration, 15),
			info.Interval,
			info.Status,
			info.ErrorCount,
			lastPoll)
	}

	// Show errors if any
	hasErrors := false
	for _, info := range infos {
		if info.LastError != "" {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Recent Errors:")
		for _, info := range infos {
			if info.LastError != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", info.Name, info.LastError)
			}
		}
	}

	return nil
}

// formatRelativeTime formats a timestamp relative to now.
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	duration := time.Since(t)
	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// truncate truncates a string to the given length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// listPollTriggers finds all poll triggers from workflow files.
// This scans the workflows directory for poll trigger configurations.
func listPollTriggers() ([]*polltrigger.PollTriggerRegistration, error) {
	mgr, err := getManager()
	if err != nil {
		return nil, err
	}

	// This will be implemented when we add poll triggers to the manager
	// For now, return empty list
	_ = mgr
	return nil, fmt.Errorf("poll trigger listing not yet implemented in workflow manager")
}

// getPollStateManager creates a state manager for reading poll state.
func getPollStateManager() (*polltrigger.StateManager, error) {
	// Use default state path
	return polltrigger.NewStateManager(polltrigger.StateConfig{})
}
