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
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tombee/conductor/internal/client"
)

// NewAttachCmd creates the debug attach command.
func NewAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach <session-id>",
		Short: "Attach to an existing debug session",
		Long: `Attach to an existing debug session for a running workflow.

This command connects to a debug session running on the controller,
allowing you to observe events and send debug commands.

Examples:
  # Attach to a debug session
  conductor debug attach debug-abc123-1234567890

  # Attach and reconnect automatically
  conductor debug attach debug-abc123-1234567890`,
		Args: cobra.ExactArgs(1),
		RunE: runAttach,
	}

	return cmd
}

// runAttach implements the debug attach command.
func runAttach(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	ctx := cmd.Context()

	// Create API client
	c, err := client.New()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// First, get the session details to find the run ID
	session, err := getSession(ctx, c, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	fmt.Printf("Attaching to debug session %s (run: %s)\n", sessionID, session.RunID)
	fmt.Printf("State: %s, Current Step: %s\n", session.State, session.CurrentStepID)

	// Create SSE client
	sseClient := NewSSEClient(c, session.RunID, sessionID)

	// Stream events
	fmt.Println("Connected to debug session. Streaming events...")

	return sseClient.StreamEvents(ctx, func(event DebugEvent) error {
		return handleDebugEvent(event)
	})
}

// Session represents a debug session.
type Session struct {
	SessionID     string `json:"session_id"`
	RunID         string `json:"run_id"`
	CurrentStepID string `json:"current_step_id"`
	State         string `json:"state"`
	LastActivity  string `json:"last_activity"`
	CreatedAt     string `json:"created_at"`
	ExpiresAt     string `json:"expires_at"`
}

// getSession retrieves session details from the controller.
func getSession(ctx context.Context, c *client.Client, sessionID string) (*Session, error) {
	resp, err := c.Get(ctx, "/v1/debug/sessions")
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	// Parse sessions array
	sessionsData, ok := resp["sessions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	for _, item := range sessionsData {
		sessionMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		sid, _ := sessionMap["session_id"].(string)
		if sid == sessionID {
			// Convert to Session struct
			jsonData, _ := json.Marshal(sessionMap)
			var session Session
			if err := json.Unmarshal(jsonData, &session); err != nil {
				return nil, fmt.Errorf("failed to parse session: %w", err)
			}
			return &session, nil
		}
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// handleDebugEvent handles a single debug event.
func handleDebugEvent(event DebugEvent) error {
	switch event.Type {
	case "heartbeat":
		// Silent heartbeat
		return nil

	case "step_start":
		stepID, _ := event.Data["step_id"].(string)
		stepIndex, _ := event.Data["step_index"].(float64)
		fmt.Printf("✓ Step Started: %s (index: %.0f)\n", stepID, stepIndex)

	case "paused":
		stepID, _ := event.Data["step_id"].(string)
		fmt.Printf("⏸  Workflow Paused at Step: %s\n", stepID)
		fmt.Println("Available commands: continue, next, skip, abort, inspect, context")

	case "resumed":
		stepID, _ := event.Data["step_id"].(string)
		fmt.Printf("▶  Workflow Resumed from Step: %s\n", stepID)

	case "completed":
		stepID, _ := event.Data["step_id"].(string)
		duration, _ := event.Data["duration_str"].(string)
		fmt.Printf("✓ Step Completed: %s (duration: %s)\n", stepID, duration)

	case "command_error":
		command, _ := event.Data["command"].(string)
		errorMsg, _ := event.Data["error"].(string)
		fmt.Fprintf(os.Stderr, "✗ Command Error (%s): %s\n", command, errorMsg)

	default:
		fmt.Printf("Event: %s\n", event.Type)
	}

	return nil
}

func init() {
	// This will be called from the root command to register the debug attach command
}
