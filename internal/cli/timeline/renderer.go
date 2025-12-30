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

// Package timeline provides ASCII timeline rendering for workflow execution visualization.
package timeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/tombee/conductor/pkg/observability"
	"golang.org/x/term"
)

const (
	// MinTerminalWidth is the minimum supported terminal width
	MinTerminalWidth = 80
	// DefaultBarWidth is the default width for duration bars
	DefaultBarWidth = 40
	// StatusIconOK indicates successful completion
	StatusIconOK = "✓"
	// StatusIconError indicates failure
	StatusIconError = "✗"
)

// TimelineSpan represents a span in timeline format with position information.
type TimelineSpan struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Status    observability.StatusCode
	Cost      float64
	Level     int    // Indentation level for hierarchy
	Lane      int    // Lane number for parallel execution
	IsParent  bool   // Whether this span has children
}

// Renderer renders ASCII timelines from trace spans.
type Renderer struct {
	Width    int
	BarWidth int
}

// NewRenderer creates a new timeline renderer with terminal width detection.
func NewRenderer() (*Renderer, error) {
	width, _, err := term.GetSize(0)
	if err != nil {
		// Default to 100 if detection fails
		width = 100
	}

	if width < MinTerminalWidth {
		return nil, fmt.Errorf("terminal width %d is too narrow (minimum %d columns)", width, MinTerminalWidth)
	}

	// Reserve space for labels, status, cost, and borders
	// Format: "│ step_name ██████░░░░  duration  status  $cost │"
	// Breakdown: 2 (border) + 20 (name) + barWidth + 10 (duration) + 3 (status) + 10 (cost) = ~80 min
	barWidth := width - 50
	if barWidth > 60 {
		barWidth = 60
	}
	if barWidth < DefaultBarWidth {
		barWidth = DefaultBarWidth
	}

	return &Renderer{
		Width:    width,
		BarWidth: barWidth,
	}, nil
}

// Render generates an ASCII timeline from trace spans.
func (r *Renderer) Render(workflowID string, spans []*observability.Span) (string, error) {
	if len(spans) == 0 {
		return "", fmt.Errorf("no spans to render")
	}

	// Convert to timeline spans
	timelineSpans := r.prepareSpans(spans)
	if len(timelineSpans) == 0 {
		return "", fmt.Errorf("no valid spans to render")
	}

	// Calculate timeline bounds
	minTime, maxTime := r.calculateBounds(timelineSpans)
	totalDuration := maxTime.Sub(minTime)

	// Calculate total cost
	totalCost := 0.0
	for _, span := range timelineSpans {
		totalCost += span.Cost
	}

	var sb strings.Builder

	// Header
	border := strings.Repeat("─", r.Width-2)
	sb.WriteString("┌" + border + "┐\n")

	header := fmt.Sprintf("│ Workflow: %-*s Total: %s  │\n",
		r.Width-28,
		truncate(workflowID, r.Width-28),
		formatDuration(totalDuration))
	sb.WriteString(header)

	sb.WriteString("├" + border + "┤\n")

	// Render each span
	for _, span := range timelineSpans {
		line := r.renderSpan(span, minTime, totalDuration)
		sb.WriteString(line)
	}

	// Footer
	sb.WriteString("└" + border + "┘\n")

	// Total cost
	if totalCost > 0 {
		sb.WriteString(fmt.Sprintf("\nTotal Cost: $%.4f\n", totalCost))
	}

	return sb.String(), nil
}

// prepareSpans converts observability spans to timeline spans with position info.
func (r *Renderer) prepareSpans(spans []*observability.Span) []TimelineSpan {
	var result []TimelineSpan

	// Build span map and parent-child relationships
	spanMap := make(map[string]*observability.Span)
	children := make(map[string][]*observability.Span)

	for _, span := range spans {
		spanMap[span.SpanID] = span
		if span.ParentID != "" {
			children[span.ParentID] = append(children[span.ParentID], span)
		}
	}

	// Find root span(s)
	var roots []*observability.Span
	for _, span := range spans {
		if span.ParentID == "" {
			roots = append(roots, span)
		}
	}

	// Recursively convert spans
	for _, root := range roots {
		r.convertSpan(root, children, spanMap, 0, &result)
	}

	return result
}

// convertSpan recursively converts a span and its children to timeline format.
func (r *Renderer) convertSpan(span *observability.Span, children map[string][]*observability.Span,
	spanMap map[string]*observability.Span, level int, result *[]TimelineSpan) {

	duration := span.Duration()
	if duration == 0 && !span.EndTime.IsZero() {
		duration = span.EndTime.Sub(span.StartTime)
	}

	// Extract cost from attributes if available
	cost := 0.0
	if costVal, ok := span.Attributes["cost_usd"]; ok {
		if costFloat, ok := costVal.(float64); ok {
			cost = costFloat
		}
	}

	timelineSpan := TimelineSpan{
		Name:      span.Name,
		StartTime: span.StartTime,
		EndTime:   span.EndTime,
		Duration:  duration,
		Status:    span.Status.Code,
		Cost:      cost,
		Level:     level,
		IsParent:  len(children[span.SpanID]) > 0,
	}

	*result = append(*result, timelineSpan)

	// Convert children
	for _, child := range children[span.SpanID] {
		r.convertSpan(child, children, spanMap, level+1, result)
	}
}

// calculateBounds finds the earliest start and latest end time across all spans.
func (r *Renderer) calculateBounds(spans []TimelineSpan) (time.Time, time.Time) {
	if len(spans) == 0 {
		return time.Now(), time.Now()
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

	return minTime, maxTime
}

// renderSpan generates a timeline line for a single span.
func (r *Renderer) renderSpan(span TimelineSpan, minTime time.Time, totalDuration time.Duration) string {
	// Calculate bar position and length
	startOffset := span.StartTime.Sub(minTime)
	startPos := int(float64(startOffset) / float64(totalDuration) * float64(r.BarWidth))
	barLength := int(float64(span.Duration) / float64(totalDuration) * float64(r.BarWidth))

	if barLength < 1 {
		barLength = 1
	}
	if startPos+barLength > r.BarWidth {
		barLength = r.BarWidth - startPos
	}

	// Build the timeline bar
	bar := make([]rune, r.BarWidth)
	for i := 0; i < r.BarWidth; i++ {
		if i >= startPos && i < startPos+barLength {
			bar[i] = '█'
		} else {
			bar[i] = '░'
		}
	}

	// Status icon
	statusIcon := StatusIconOK
	if span.Status == observability.StatusCodeError {
		statusIcon = StatusIconError
	}

	// Format name with indentation
	indent := strings.Repeat("  ", span.Level)
	prefix := ""
	if span.Level > 0 {
		if span.IsParent {
			prefix = "├─ "
		} else {
			prefix = "└─ "
		}
	}

	nameWidth := 20 - len(indent) - len(prefix)
	if nameWidth < 10 {
		nameWidth = 10
	}
	name := truncate(span.Name, nameWidth)

	// Format cost
	costStr := ""
	if span.Cost > 0 {
		costStr = fmt.Sprintf("$%.4f", span.Cost)
	}

	// Build the line
	line := fmt.Sprintf("│ %s%s%-*s %s  %6s  %s  %8s │\n",
		indent,
		prefix,
		nameWidth,
		name,
		string(bar),
		formatDuration(span.Duration),
		statusIcon,
		costStr,
	)

	return line
}

// truncate shortens a string to maxLen with ellipsis if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}
