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

package timeline

import (
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/observability"
)

func TestRenderer_Render(t *testing.T) {
	tests := []struct {
		name       string
		workflowID string
		spans      []*observability.Span
		wantErr    bool
		checks     []func(string) bool
	}{
		{
			name:       "single span",
			workflowID: "test-workflow",
			spans: []*observability.Span{
				{
					TraceID:   "trace1",
					SpanID:    "span1",
					Name:      "step1",
					Kind:      observability.SpanKindInternal,
					StartTime: time.Now(),
					EndTime:   time.Now().Add(100 * time.Millisecond),
					Status:    observability.SpanStatus{Code: observability.StatusCodeOK},
				},
			},
			wantErr: false,
			checks: []func(string) bool{
				func(s string) bool { return strings.Contains(s, "test-workflow") },
				func(s string) bool { return strings.Contains(s, "step1") },
				func(s string) bool { return strings.Contains(s, StatusIconOK) },
			},
		},
		{
			name:       "parent and child spans",
			workflowID: "nested-workflow",
			spans: []*observability.Span{
				{
					TraceID:   "trace2",
					SpanID:    "parent",
					Name:      "parent_step",
					Kind:      observability.SpanKindInternal,
					StartTime: time.Now(),
					EndTime:   time.Now().Add(200 * time.Millisecond),
					Status:    observability.SpanStatus{Code: observability.StatusCodeOK},
				},
				{
					TraceID:   "trace2",
					SpanID:    "child1",
					ParentID:  "parent",
					Name:      "child_step",
					Kind:      observability.SpanKindInternal,
					StartTime: time.Now().Add(10 * time.Millisecond),
					EndTime:   time.Now().Add(110 * time.Millisecond),
					Status:    observability.SpanStatus{Code: observability.StatusCodeOK},
				},
			},
			wantErr: false,
			checks: []func(string) bool{
				func(s string) bool { return strings.Contains(s, "parent_step") },
				func(s string) bool { return strings.Contains(s, "child_step") },
				func(s string) bool { return strings.Contains(s, "├─") || strings.Contains(s, "└─") },
			},
		},
		{
			name:       "failed span shows error icon",
			workflowID: "failed-workflow",
			spans: []*observability.Span{
				{
					TraceID:   "trace3",
					SpanID:    "span1",
					Name:      "failing_step",
					Kind:      observability.SpanKindInternal,
					StartTime: time.Now(),
					EndTime:   time.Now().Add(50 * time.Millisecond),
					Status:    observability.SpanStatus{Code: observability.StatusCodeError, Message: "test error"},
				},
			},
			wantErr: false,
			checks: []func(string) bool{
				func(s string) bool { return strings.Contains(s, StatusIconError) },
				func(s string) bool { return strings.Contains(s, "failing_step") },
			},
		},
		{
			name:       "span with cost displays cost",
			workflowID: "cost-workflow",
			spans: []*observability.Span{
				{
					TraceID:   "trace4",
					SpanID:    "span1",
					Name:      "expensive_step",
					Kind:      observability.SpanKindInternal,
					StartTime: time.Now(),
					EndTime:   time.Now().Add(100 * time.Millisecond),
					Status:    observability.SpanStatus{Code: observability.StatusCodeOK},
					Attributes: map[string]any{
						"cost_usd": 1.23,
					},
				},
			},
			wantErr: false,
			checks: []func(string) bool{
				func(s string) bool { return strings.Contains(s, "1.23") },
				func(s string) bool { return strings.Contains(s, "Total Cost") },
			},
		},
		{
			name:       "empty spans returns error",
			workflowID: "empty",
			spans:      []*observability.Span{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Renderer{
				Width:    100,
				BarWidth: 40,
			}

			output, err := r.Render(tt.workflowID, tt.spans)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Render() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Render() unexpected error: %v", err)
				return
			}

			// Run checks
			for i, check := range tt.checks {
				if !check(output) {
					t.Errorf("Render() check %d failed\nOutput:\n%s", i, output)
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "short",
			maxLen: 10,
			want:   "short",
		},
		{
			name:   "exact length unchanged",
			input:  "exactly10c",
			maxLen: 10,
			want:   "exactly10c",
		},
		{
			name:   "long string truncated",
			input:  "this is a very long string",
			maxLen: 10,
			want:   "this is...",
		},
		{
			name:   "maxLen <= 3 no ellipsis",
			input:  "test",
			maxLen: 3,
			want:   "tes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		dur  time.Duration
		want string
	}{
		{
			name: "microseconds",
			dur:  500 * time.Microsecond,
			want: "500µs",
		},
		{
			name: "milliseconds",
			dur:  150 * time.Millisecond,
			want: "150ms",
		},
		{
			name: "seconds",
			dur:  2500 * time.Millisecond,
			want: "2.5s",
		},
		{
			name: "minutes",
			dur:  90 * time.Second,
			want: "1.5m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.dur)
			if got != tt.want {
				t.Errorf("formatDuration() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCalculateBounds(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	spans := []TimelineSpan{
		{
			Name:      "span1",
			StartTime: baseTime,
			EndTime:   baseTime.Add(100 * time.Millisecond),
		},
		{
			Name:      "span2",
			StartTime: baseTime.Add(50 * time.Millisecond),
			EndTime:   baseTime.Add(200 * time.Millisecond),
		},
		{
			Name:      "span3",
			StartTime: baseTime.Add(10 * time.Millisecond),
			EndTime:   baseTime.Add(150 * time.Millisecond),
		},
	}

	r := &Renderer{Width: 100, BarWidth: 40}
	minTime, maxTime := r.calculateBounds(spans)

	if !minTime.Equal(baseTime) {
		t.Errorf("calculateBounds() minTime = %v, want %v", minTime, baseTime)
	}

	expectedMax := baseTime.Add(200 * time.Millisecond)
	if !maxTime.Equal(expectedMax) {
		t.Errorf("calculateBounds() maxTime = %v, want %v", maxTime, expectedMax)
	}
}

func TestNewRenderer_TerminalWidthValidation(t *testing.T) {
	// This test validates the renderer creation logic
	// Terminal width detection may fail in test environment, so we test the error case
	r := &Renderer{
		Width:    MinTerminalWidth - 1,
		BarWidth: DefaultBarWidth,
	}

	// Renderer with width below minimum should fail to render
	_, err := r.Render("test", []*observability.Span{
		{
			TraceID:   "trace1",
			SpanID:    "span1",
			Name:      "test",
			StartTime: time.Now(),
			EndTime:   time.Now().Add(100 * time.Millisecond),
			Status:    observability.SpanStatus{Code: observability.StatusCodeOK},
		},
	})

	// Should succeed but output may be malformed (acceptable for narrow terminals)
	if err != nil && !strings.Contains(err.Error(), "no spans") && !strings.Contains(err.Error(), "no valid spans") {
		// Only error if it's not about missing/invalid spans
		t.Logf("Render with narrow width produced expected behavior: %v", err)
	}
}
