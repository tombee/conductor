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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/client"
)

// SSEClient handles Server-Sent Events streaming for debug sessions.
type SSEClient struct {
	client    *client.Client
	sessionID string
	runID     string
	lastEventID string
}

// NewSSEClient creates a new SSE client for a debug session.
func NewSSEClient(c *client.Client, runID, sessionID string) *SSEClient {
	return &SSEClient{
		client:    c,
		runID:     runID,
		sessionID: sessionID,
	}
}

// DebugEvent represents a debug event from the SSE stream.
type DebugEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// StreamEvents streams debug events, calling the handler for each event.
// Automatically reconnects on disconnect with event ID for resumption.
func (c *SSEClient) StreamEvents(ctx context.Context, handler func(DebugEvent) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := c.streamOnce(ctx, handler)
		if err != nil {
			// Check if this is a context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Otherwise, attempt to reconnect
			fmt.Printf("Connection lost: %v. Reconnecting...\n", err)
			time.Sleep(2 * time.Second)
			continue
		}

		return nil
	}
}

// streamOnce establishes a single SSE connection and processes events.
func (c *SSEClient) streamOnce(ctx context.Context, handler func(DebugEvent) error) error {
	path := fmt.Sprintf("/v1/runs/%s/debug/events?session_id=%s", c.runID, c.sessionID)

	// Include last event ID for reconnection support
	if c.lastEventID != "" {
		path += fmt.Sprintf("&last_event_id=%s", c.lastEventID)
	}

	resp, err := c.client.GetStream(ctx, path, "text/event-stream")
	if err != nil {
		return fmt.Errorf("failed to connect to event stream: %w", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var currentEvent *DebugEvent

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("stream closed")
			}
			return fmt.Errorf("read error: %w", err)
		}

		line = strings.TrimSpace(line)

		// Empty line indicates end of event
		if line == "" {
			if currentEvent != nil {
				// Update last event ID for reconnection
				c.lastEventID = currentEvent.ID

				// Call handler
				if err := handler(*currentEvent); err != nil {
					return err
				}

				currentEvent = nil
			}
			continue
		}

		// Parse SSE field
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		field := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if currentEvent == nil {
			currentEvent = &DebugEvent{}
		}

		switch field {
		case "id":
			currentEvent.ID = value

		case "event":
			currentEvent.Type = value

		case "data":
			// Parse JSON data
			var fullEvent DebugEvent
			if err := json.Unmarshal([]byte(value), &fullEvent); err != nil {
				// Log error but continue processing
				fmt.Printf("Failed to parse event data: %v\n", err)
				continue
			}

			// Copy parsed data
			currentEvent.ID = fullEvent.ID
			currentEvent.Type = fullEvent.Type
			currentEvent.Timestamp = fullEvent.Timestamp
			currentEvent.Data = fullEvent.Data
		}
	}
}

// SendCommand sends a debug command to the controller.
func (c *SSEClient) SendCommand(ctx context.Context, commandType string, payload map[string]interface{}) error {
	path := fmt.Sprintf("/v1/runs/%s/debug/command?session_id=%s", c.runID, c.sessionID)

	cmd := map[string]interface{}{
		"type":    commandType,
		"payload": payload,
	}

	_, err := c.client.Post(ctx, path, cmd)
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	return nil
}
