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

package management

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

var (
	eventsFollow   bool
	eventsRun      string
	eventsWorkflow string
	eventsSince    string
	eventsJSON     bool
)

// NewEventsCommand creates the events command
func NewEventsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Annotations: map[string]string{
			"group": "management",
		},
		Short: "View workflow events",
		Long:  `View and stream workflow execution events for real-time monitoring.`,
		RunE:  runEventsList,
	}

	cmd.Flags().BoolVarP(&eventsFollow, "follow", "f", false, "Stream events in real-time")
	cmd.Flags().StringVar(&eventsRun, "run", "", "Filter by run ID")
	cmd.Flags().StringVar(&eventsWorkflow, "workflow", "", "Filter by workflow ID")
	cmd.Flags().StringVar(&eventsSince, "since", "1h", "Show events since duration (e.g., 1h, 30m)")
	cmd.Flags().BoolVar(&eventsJSON, "json", false, "Output as JSON")

	return cmd
}

func runEventsList(cmd *cobra.Command, args []string) error {
	if eventsFollow {
		return streamEvents()
	}

	// Parse since duration
	since, err := parseDuration(eventsSince)
	if err != nil {
		return fmt.Errorf("invalid --since duration: %w", err)
	}

	// Build query parameters
	params := make(map[string]string)
	if eventsRun != "" {
		params["run"] = eventsRun
	}
	if eventsWorkflow != "" {
		params["workflow"] = eventsWorkflow
	}
	params["since"] = since.Format(time.RFC3339)

	// Make API request to daemon
	url := shared.BuildAPIURL("/v1/events", params)
	resp, err := shared.MakeAPIRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch events: %w", err)
	}

	var events []Event
	if err := json.Unmarshal(resp, &events); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if eventsJSON {
		output, err := json.MarshalIndent(events, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Display as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tEVENT\tWORKFLOW\tRUN\tDETAILS")
	for _, event := range events {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			formatTime(event.Timestamp),
			event.Name,
			event.WorkflowID,
			truncateID(event.RunID),
			formatEventDetails(event),
		)
	}
	w.Flush()

	return nil
}

func streamEvents() error {
	// Build query parameters
	params := make(map[string]string)
	if eventsRun != "" {
		params["run"] = eventsRun
	}
	if eventsWorkflow != "" {
		params["workflow"] = eventsWorkflow
	}

	// Make streaming request to daemon
	url := shared.BuildAPIURL("/v1/events/stream", params)

	client := &http.Client{
		Timeout: 0, // No timeout for streaming
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to event stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Stream events
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {json}"
		if len(line) > 6 && line[:6] == "data: " {
			eventJSON := line[6:]

			var event Event
			if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to parse event: %v\n", err)
				continue
			}

			if eventsJSON {
				output, _ := json.Marshal(event)
				fmt.Println(string(output))
			} else {
				fmt.Printf("[%s] %s - %s (run: %s) - %s\n",
					formatTime(event.Timestamp),
					event.Name,
					event.WorkflowID,
					truncateID(event.RunID),
					formatEventDetails(event),
				)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}

// Event represents a workflow event
type Event struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Timestamp  time.Time      `json:"timestamp"`
	TraceID    string         `json:"trace_id"`
	SpanID     string         `json:"span_id"`
	RunID      string         `json:"run_id"`
	WorkflowID string         `json:"workflow_id"`
	Attributes map[string]any `json:"attributes"`
}

func formatEventDetails(event Event) string {
	// Extract key details from attributes
	details := ""

	switch event.Name {
	case "run.started":
		if trigger, ok := event.Attributes["trigger"].(string); ok {
			details = fmt.Sprintf("trigger: %s", trigger)
		}
	case "run.completed":
		if duration, ok := event.Attributes["duration_ms"].(float64); ok {
			details = fmt.Sprintf("duration: %dms", int(duration))
		}
	case "run.failed":
		if errMsg, ok := event.Attributes["error"].(string); ok {
			// Truncate error message
			if len(errMsg) > 50 {
				errMsg = errMsg[:50] + "..."
			}
			details = fmt.Sprintf("error: %s", errMsg)
		}
	case "step.completed":
		if stepName, ok := event.Attributes["conductor.step.name"].(string); ok {
			details = fmt.Sprintf("step: %s", stepName)
		}
	case "llm.response":
		if tokens, ok := event.Attributes["llm.tokens.completion"].(float64); ok {
			details = fmt.Sprintf("tokens: %d", int(tokens))
		}
	}

	return details
}
