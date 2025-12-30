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
	"github.com/tombee/conductor/internal/cli/timeline"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/pkg/observability"
)

var (
	tracesWorkflow string
	tracesSince    string
	tracesStatus   string
	tracesJSON     bool
	tracesLLM      bool
	tracesHTTP     bool
	tracesFailed   bool
	exportFormat   string
)

// NewTracesCommand creates the traces command
func NewTracesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "traces",
		Annotations: map[string]string{
			"group": "management",
		},
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
	showCmd.Flags().BoolVar(&tracesLLM, "llm", false, "Filter to show only LLM spans")
	showCmd.Flags().BoolVar(&tracesHTTP, "http", false, "Filter to show only HTTP spans")
	showCmd.Flags().BoolVar(&tracesFailed, "failed", false, "Filter to show only failed spans")

	// Add timeline subcommand
	timelineCmd := &cobra.Command{
		Use:   "timeline <trace-id>",
		Short: "Show ASCII timeline visualization of trace execution",
		Long:  `Display a waterfall timeline view showing span execution over time.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runTracesTimeline,
	}

	// Add export subcommand
	exportCmd := &cobra.Command{
		Use:   "export <trace-id>",
		Short: "Export trace visualization to a file",
		Long:  `Export trace data in various formats for external analysis.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runTracesExport,
	}
	exportCmd.Flags().StringVar(&exportFormat, "format", "html", "Export format (html, json)")

	// Add diff subcommand
	diffCmd := &cobra.Command{
		Use:   "diff <trace-id-1> <trace-id-2>",
		Short: "Compare two workflow trace executions",
		Long:  `Show differences between two trace executions including status, duration, and output changes.`,
		Args:  cobra.ExactArgs(2),
		RunE:  runTracesDiff,
	}

	cmd.AddCommand(showCmd, timelineCmd, exportCmd, diffCmd)

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

	// Display spans with filtering
	fmt.Println("Spans:")
	displaySpan(trace.RootSpan, 0, &spanFilter{
		llmOnly:    tracesLLM,
		httpOnly:   tracesHTTP,
		failedOnly: tracesFailed,
	})

	return nil
}

type spanFilter struct {
	llmOnly    bool
	httpOnly   bool
	failedOnly bool
}

func (f *spanFilter) shouldDisplay(span Span) bool {
	if f == nil {
		return true
	}

	// Failed filter
	if f.failedOnly && span.Status != "error" {
		return false
	}

	// LLM filter - check for llm.* attributes
	if f.llmOnly {
		hasLLM := false
		for k := range span.Attributes {
			if strings.HasPrefix(k, "llm.") {
				hasLLM = true
				break
			}
		}
		if !hasLLM {
			return false
		}
	}

	// HTTP filter - check for http.* attributes
	if f.httpOnly {
		hasHTTP := false
		for k := range span.Attributes {
			if strings.HasPrefix(k, "http.") {
				hasHTTP = true
				break
			}
		}
		if !hasHTTP {
			return false
		}
	}

	return true
}

func displaySpan(span Span, indent int, filter *spanFilter) {
	// Apply filter
	if !filter.shouldDisplay(span) {
		// Still recurse to children
		for _, child := range span.Children {
			displaySpan(child, indent, filter)
		}
		return
	}

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
			if strings.HasPrefix(k, "conductor.") || strings.HasPrefix(k, "llm.") || strings.HasPrefix(k, "http.") {
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
		displaySpan(child, indent+1, filter)
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

// runTracesTimeline displays an ASCII timeline visualization of trace execution.
func runTracesTimeline(cmd *cobra.Command, args []string) error {
	traceID := args[0]

	// Fetch trace data from API
	url := shared.BuildAPIURL(fmt.Sprintf("/v1/traces/%s", traceID), nil)
	resp, err := shared.MakeAPIRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch trace: %w", err)
	}

	// Parse API response
	var apiResp struct {
		TraceID string                 `json:"trace_id"`
		Spans   []*observability.Span  `json:"spans"`
	}
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Spans) == 0 {
		return fmt.Errorf("no spans found for trace %s", traceID)
	}

	// Create timeline renderer
	renderer, err := timeline.NewRenderer()
	if err != nil {
		return fmt.Errorf("failed to create timeline renderer: %w", err)
	}

	// Determine workflow ID from spans
	workflowID := traceID
	for _, span := range apiResp.Spans {
		if wfID, ok := span.Attributes["conductor.workflow_id"].(string); ok {
			workflowID = wfID
			break
		}
	}

	// Render timeline
	output, err := renderer.Render(workflowID, apiResp.Spans)
	if err != nil {
		return fmt.Errorf("failed to render timeline: %w", err)
	}

	fmt.Print(output)
	return nil
}

// runTracesExport exports trace data to various formats.
func runTracesExport(cmd *cobra.Command, args []string) error {
	traceID := args[0]

	// Fetch trace data from API
	url := shared.BuildAPIURL(fmt.Sprintf("/v1/traces/%s", traceID), nil)
	resp, err := shared.MakeAPIRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch trace: %w", err)
	}

	switch exportFormat {
	case "json":
		// Pretty print JSON
		var data interface{}
		if err := json.Unmarshal(resp, &data); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
		output, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil

	case "html":
		// Parse spans for HTML export
		var apiResp struct {
			TraceID string                 `json:"trace_id"`
			Spans   []*observability.Span  `json:"spans"`
		}
		if err := json.Unmarshal(resp, &apiResp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Generate HTML output
		html, err := generateHTMLTimeline(apiResp.TraceID, apiResp.Spans)
		if err != nil {
			return fmt.Errorf("failed to generate HTML: %w", err)
		}

		// Write to file
		filename := fmt.Sprintf("trace_%s.html", traceID)
		if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
			return fmt.Errorf("failed to write HTML file: %w", err)
		}

		fmt.Printf("Timeline exported to: %s\n", filename)
		return nil

	default:
		return fmt.Errorf("unsupported export format: %s (supported: html, json)", exportFormat)
	}
}

// runTracesDiff compares two trace executions.
func runTracesDiff(cmd *cobra.Command, args []string) error {
	traceID1 := args[0]
	traceID2 := args[1]

	// Fetch both traces
	url1 := shared.BuildAPIURL(fmt.Sprintf("/v1/traces/%s", traceID1), nil)
	resp1, err := shared.MakeAPIRequest("GET", url1, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch trace %s: %w", traceID1, err)
	}

	url2 := shared.BuildAPIURL(fmt.Sprintf("/v1/traces/%s", traceID2), nil)
	resp2, err := shared.MakeAPIRequest("GET", url2, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch trace %s: %w", traceID2, err)
	}

	// Parse responses
	var trace1, trace2 struct {
		TraceID string                 `json:"trace_id"`
		Spans   []*observability.Span  `json:"spans"`
	}
	if err := json.Unmarshal(resp1, &trace1); err != nil {
		return fmt.Errorf("failed to parse trace1: %w", err)
	}
	if err := json.Unmarshal(resp2, &trace2); err != nil {
		return fmt.Errorf("failed to parse trace2: %w", err)
	}

	// Build span maps by name for comparison
	spans1 := buildSpanMap(trace1.Spans)
	spans2 := buildSpanMap(trace2.Spans)

	// Display comparison
	fmt.Printf("Comparing traces:\n")
	fmt.Printf("  Trace 1: %s (%d spans)\n", traceID1, len(trace1.Spans))
	fmt.Printf("  Trace 2: %s (%d spans)\n\n", traceID2, len(trace2.Spans))

	// Find differences
	differences := 0
	allNames := make(map[string]bool)
	for name := range spans1 {
		allNames[name] = true
	}
	for name := range spans2 {
		allNames[name] = true
	}

	for name := range allNames {
		s1, exists1 := spans1[name]
		s2, exists2 := spans2[name]

		if !exists1 {
			fmt.Printf("✗ Span missing in trace1: %s\n", name)
			differences++
			continue
		}
		if !exists2 {
			fmt.Printf("✗ Span missing in trace2: %s\n", name)
			differences++
			continue
		}

		// Compare status
		if s1.Status.Code != s2.Status.Code {
			fmt.Printf("✗ %s: status differs\n", name)
			fmt.Printf("    Trace 1: %s\n", statusCodeToString(s1.Status.Code))
			fmt.Printf("    Trace 2: %s\n", statusCodeToString(s2.Status.Code))
			differences++
		}

		// Compare duration (>5% or >100ms)
		dur1 := s1.Duration()
		dur2 := s2.Duration()
		if dur1 > 0 && dur2 > 0 {
			diff := dur1 - dur2
			if diff < 0 {
				diff = -diff
			}
			threshold := dur1 / 20 // 5%
			if threshold < 100*time.Millisecond {
				threshold = 100 * time.Millisecond
			}
			if diff > threshold {
				fmt.Printf("✗ %s: duration differs\n", name)
				fmt.Printf("    Trace 1: %s\n", shared.FormatDuration(dur1))
				fmt.Printf("    Trace 2: %s\n", shared.FormatDuration(dur2))
				fmt.Printf("    Difference: %s\n", shared.FormatDuration(diff))
				differences++
			}
		}
	}

	if differences == 0 {
		fmt.Println("✓ No significant differences found")
	} else {
		fmt.Printf("\n%d difference(s) found\n", differences)
	}

	return nil
}

func buildSpanMap(spans []*observability.Span) map[string]*observability.Span {
	m := make(map[string]*observability.Span)
	for _, span := range spans {
		m[span.Name] = span
	}
	return m
}

func statusCodeToString(code observability.StatusCode) string {
	switch code {
	case observability.StatusCodeOK:
		return "OK"
	case observability.StatusCodeError:
		return "ERROR"
	default:
		return "UNSET"
	}
}

func generateHTMLTimeline(traceID string, spans []*observability.Span) (string, error) {
	// Simple HTML template for timeline visualization
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Trace Timeline: ` + traceID + `</title>
    <style>
        body { font-family: monospace; margin: 20px; }
        .timeline { border: 1px solid #ccc; padding: 10px; }
        .span { margin: 5px 0; }
        .span-bar { display: inline-block; height: 20px; background-color: #4CAF50; }
        .span-error { background-color: #f44336; }
        .span-name { display: inline-block; width: 200px; }
        .span-duration { display: inline-block; margin-left: 10px; }
    </style>
</head>
<body>
    <h1>Trace Timeline: ` + traceID + `</h1>
    <div class="timeline">
`

	// Find min/max time
	if len(spans) == 0 {
		return "", fmt.Errorf("no spans to render")
	}

	minTime := spans[0].StartTime
	maxTime := spans[0].EndTime
	for _, span := range spans {
		if span.StartTime.Before(minTime) {
			minTime = span.StartTime
		}
		if span.EndTime.After(maxTime) {
			maxTime = span.EndTime
		}
	}
	totalDuration := maxTime.Sub(minTime)

	// Render each span
	for _, span := range spans {
		duration := span.Duration()
		barWidth := int(float64(duration) / float64(totalDuration) * 400)
		if barWidth < 1 {
			barWidth = 1
		}

		statusClass := ""
		if span.Status.Code == observability.StatusCodeError {
			statusClass = " span-error"
		}

		html += fmt.Sprintf(`        <div class="span">
            <span class="span-name">%s</span>
            <span class="span-bar%s" style="width: %dpx;"></span>
            <span class="span-duration">%s</span>
        </div>
`, span.Name, statusClass, barWidth, shared.FormatDuration(duration))
	}

	html += `    </div>
</body>
</html>`

	return html, nil
}
