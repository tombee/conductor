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

package debug

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/tombee/conductor/internal/client"
)

// NewSessionsCmd creates the debug sessions command.
func NewSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage debug sessions",
		Long: `List and manage active debug sessions on the controller.

Debug sessions are created when a workflow run is started with breakpoints.
Each session maintains connection state and event history for debugging.`,
	}

	cmd.AddCommand(NewSessionsListCmd())
	cmd.AddCommand(NewSessionsKillCmd())

	return cmd
}

// NewSessionsListCmd creates the debug sessions list command.
func NewSessionsListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active debug sessions",
		Long: `List all active debug sessions on the controller.

Shows session ID, run ID, current step, state, and last activity time.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Create API client
			c, err := client.New()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get sessions list
			resp, err := c.Get(ctx, "/v1/debug/sessions")
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			// Parse sessions array
			var sessions []Session
			if sessionsData, ok := resp["sessions"].([]interface{}); ok {
				for _, item := range sessionsData {
					if sessionMap, ok := item.(map[string]interface{}); ok {
						jsonData, _ := json.Marshal(sessionMap)
						var session Session
						if err := json.Unmarshal(jsonData, &session); err == nil {
							sessions = append(sessions, session)
						}
					}
				}
			}

			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(sessions)
			}

			if len(sessions) == 0 {
				fmt.Println("No active debug sessions")
				return nil
			}

			// Print table
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SESSION ID\tRUN ID\tSTATE\tCURRENT STEP\tLAST ACTIVITY")

			for _, s := range sessions {
				lastActivity := "N/A"
				if s.LastActivity != "" {
					if t, err := time.Parse(time.RFC3339, s.LastActivity); err == nil {
						lastActivity = formatDuration(time.Since(t)) + " ago"
					}
				}

				currentStep := s.CurrentStepID
				if currentStep == "" {
					currentStep = "-"
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					s.SessionID,
					s.RunID,
					s.State,
					currentStep,
					lastActivity,
				)
			}

			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

// NewSessionsKillCmd creates the debug sessions kill command.
func NewSessionsKillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kill <session-id>",
		Short: "Kill a debug session",
		Long: `Terminate a debug session and its associated run.

This command forcefully terminates a debug session, updating its state to KILLED
and canceling the associated workflow run if it's still executing.

Examples:
  # Kill a debug session
  conductor debug sessions kill debug-abc123-1234567890`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			ctx := cmd.Context()

			// Create API client
			c, err := client.New()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get session to find run ID
			session, err := getSession(ctx, c, sessionID)
			if err != nil {
				return fmt.Errorf("failed to get session: %w", err)
			}

			// Send kill command
			path := fmt.Sprintf("/v1/debug/sessions/%s", sessionID)
			if err := c.Delete(ctx, path); err != nil {
				return fmt.Errorf("failed to kill session: %w", err)
			}

			fmt.Printf("✓ Debug session %s killed\n", sessionID)
			fmt.Printf("Run %s will be terminated\n", session.RunID)

			return nil
		},
	}

	return cmd
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
