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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

var (
	tracesWorkflow string
	tracesSince    string
	tracesStatus   string
	tracesJSON     bool
)

// NewTracesCommand creates the traces command
func NewTracesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "traces",
		Short: "Manage and view workflow traces",
		Long:  `View and filter workflow execution traces for debugging and monitoring.`,
		RunE:  runTracesList,
	}

	// Add flags to traces list command
	cmd.Flags().StringVar(&tracesWorkflow, "workflow", "", "Filter by workflow ID")
	cmd.Flags().StringVar(&tracesSince, "since", "24h", "Show traces since duration (e.g., 1h, 30m, 7d)")
	cmd.Flags().StringVar(&tracesStatus, "status", "", "Filter by status (ok, error)")
	cmd.Flags().BoolVar(&tracesJSON, "json", false, "Output as JSON")

	// Add show subcommand
	showCmd := &cobra.Command{
		Use:   "show <trace-id>",
		Short: "Show details of a specific trace",
		Long:  `Display detailed information about a workflow trace including all spans.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runTracesShow,
	}
	showCmd.Flags().BoolVar(&tracesJSON, "json", false, "Output as JSON")

	cmd.AddCommand(showCmd)

	return cmd
}

func runTracesList(cmd *cobra.Command, args []string) error {
	// Parse since duration
	since, err := parseDuration(tracesSince)
	if err != nil {
		return fmt.Errorf("invalid --since duration: %w", err)
	}

	// Build query parameters
	params := make(map[string]string)
	if tracesWorkflow != "" {
		params["workflow"] = tracesWorkflow
	}
	if tracesStatus != "" {
		params["status"] = tracesStatus
	}
	params["since"] = since.Format(time.RFC3339)

	// Make API request to daemon
	url := shared.BuildAPIURL("/v1/traces", params)
	resp, err := shared.MakeAPIRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch traces: %w", err)
	}

	var traces []TraceListItem
	if err := json.Unmarshal(resp, &traces); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if tracesJSON {
		output, err := json.MarshalIndent(traces, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Display as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TRACE ID\tWORKFLOW\tSTATUS\tDURATION\tSTARTED\tCOST")
	for _, trace := range traces {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t$%.4f\n",
			truncateID(trace.TraceID),
			trace.WorkflowID,
			trace.Status,
			shared.FormatDuration(trace.Duration),
			formatTime(trace.StartTime),
			trace.TotalCost,
		)
	}
	w.Flush()

	return nil
}

func runTracesShow(cmd *cobra.Command, args []string) error {
	traceID := args[0]

	// Make API request to daemon
	url := shared.BuildAPIURL(fmt.Sprintf("/v1/traces/%s", traceID), nil)
	resp, err := shared.MakeAPIRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch trace: %w", err)
	}

	var trace TraceDetail
	if err := json.Unmarshal(resp, &trace); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if tracesJSON {
		output, err := json.MarshalIndent(trace, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Display trace summary
	fmt.Printf("Trace: %s\n", trace.TraceID)
	fmt.Printf("Workflow: %s\n", trace.WorkflowID)
	fmt.Printf("Status: %s\n", trace.Status)
	fmt.Printf("Started: %s\n", formatTime(trace.StartTime))
	fmt.Printf("Duration: %s\n", shared.FormatDuration(trace.Duration))
	fmt.Printf("Total Cost: $%.4f\n\n", trace.TotalCost)

	// Display spans
	fmt.Println("Spans:")
	displaySpan(trace.RootSpan, 0)

	return nil
}

func displaySpan(span Span, indent int) {
	prefix := strings.Repeat("  ", indent)
	statusIcon := "✓"
	if span.Status == "error" {
		statusIcon = "✗"
	}

	fmt.Printf("%s%s %s (%s)\n", prefix, statusIcon, span.Name, shared.FormatDuration(span.Duration))

	// Display key attributes
	if len(span.Attributes) > 0 {
		for k, v := range span.Attributes {
			// Only show important attributes
			if strings.HasPrefix(k, "conductor.") || strings.HasPrefix(k, "llm.") {
				fmt.Printf("%s  %s: %v\n", prefix, k, v)
			}
		}
	}

	// Display events
	if len(span.Events) > 0 {
		for _, event := range span.Events {
			fmt.Printf("%s  [%s] %s\n", prefix, formatTime(event.Timestamp), event.Name)
		}
	}

	// Display children
	for _, child := range span.Children {
		displaySpan(child, indent+1)
	}
}

// TraceListItem represents a trace in the list view
type TraceListItem struct {
	TraceID    string        `json:"trace_id"`
	WorkflowID string        `json:"workflow_id"`
	Status     string        `json:"status"`
	StartTime  time.Time     `json:"start_time"`
	Duration   time.Duration `json:"duration"`
	TotalCost  float64       `json:"total_cost"`
}

// TraceDetail represents full trace details
type TraceDetail struct {
	TraceID    string        `json:"trace_id"`
	WorkflowID string        `json:"workflow_id"`
	Status     string        `json:"status"`
	StartTime  time.Time     `json:"start_time"`
	Duration   time.Duration `json:"duration"`
	TotalCost  float64       `json:"total_cost"`
	RootSpan   Span          `json:"root_span"`
}

// Span represents a trace span
type Span struct {
	SpanID     string            `json:"span_id"`
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	StartTime  time.Time         `json:"start_time"`
	Duration   time.Duration     `json:"duration"`
	Attributes map[string]any    `json:"attributes"`
	Events     []SpanEvent       `json:"events"`
	Children   []Span            `json:"children"`
}

// SpanEvent represents an event within a span
type SpanEvent struct {
	Name       string         `json:"name"`
	Timestamp  time.Time      `json:"timestamp"`
	Attributes map[string]any `json:"attributes"`
}

// Helper functions

func parseDuration(s string) (time.Time, error) {
	dur, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-dur), nil
}

func truncateID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func formatTime(t time.Time) string {
	if time.Since(t) < 24*time.Hour {
		return t.Format("15:04:05")
	}
	return t.Format("2006-01-02 15:04")
}
