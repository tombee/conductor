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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/tracing/storage"
	"github.com/tombee/conductor/pkg/observability"
)

func setupTestStore(t *testing.T) *storage.SQLiteStore {
	t.Helper()

	store, err := storage.New(storage.Config{
		Path:             ":memory:",
		MaxOpenConns:     1,
		EnableEncryption: false,
	})
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

func TestTracesHandler_ListTraces(t *testing.T) {
	store := setupTestStore(t)
	handler := NewTracesHandler(store)

	ctx := context.Background()

	// Create test traces
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	traces := []struct {
		traceID string
		status  observability.StatusCode
		time    time.Time
	}{
		{"trace1", observability.StatusCodeOK, baseTime},
		{"trace2", observability.StatusCodeError, baseTime.Add(1 * time.Hour)},
		{"trace3", observability.StatusCodeOK, baseTime.Add(2 * time.Hour)},
	}

	for _, tc := range traces {
		span := &observability.Span{
			TraceID:   tc.traceID,
			SpanID:    tc.traceID + "-span1",
			Name:      "test-span",
			Kind:      "internal",
			StartTime: tc.time,
			EndTime:   tc.time.Add(100 * time.Millisecond),
			Status: observability.SpanStatus{
				Code: tc.status,
			},
			Attributes: map[string]any{},
		}

		if err := store.StoreSpan(ctx, span); err != nil {
			t.Fatalf("failed to store test span: %v", err)
		}
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedCount  int
		expectedStatus int
	}{
		{
			name:           "list all traces",
			queryParams:    "",
			expectedCount:  3,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by status ok",
			queryParams:    "status=ok",
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by status error",
			queryParams:    "status=error",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by since",
			queryParams:    "since=" + baseTime.Add(1*time.Hour).Format(time.RFC3339),
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by until",
			queryParams:    "until=" + baseTime.Add(1*time.Hour).Format(time.RFC3339),
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid since parameter",
			queryParams:    "since=invalid-date",
			expectedCount:  0,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid until parameter",
			queryParams:    "until=invalid-date",
			expectedCount:  0,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/traces?"+tt.queryParams, nil)
			rec := httptest.NewRecorder()

			handler.ListTraces(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				traces, ok := response["traces"].([]interface{})
				if !ok {
					t.Fatalf("expected traces array in response")
				}

				if len(traces) != tt.expectedCount {
					t.Errorf("expected %d traces, got %d", tt.expectedCount, len(traces))
				}

				count, ok := response["count"].(float64)
				if !ok {
					t.Fatalf("expected count in response")
				}

				if int(count) != tt.expectedCount {
					t.Errorf("expected count %d, got %d", tt.expectedCount, int(count))
				}
			}
		})
	}
}

func TestTracesHandler_GetTrace(t *testing.T) {
	store := setupTestStore(t)
	handler := NewTracesHandler(store)

	ctx := context.Background()

	// Create a test trace with multiple spans
	traceID := "test-trace-123"
	baseTime := time.Now()

	spans := []*observability.Span{
		{
			TraceID:   traceID,
			SpanID:    "span1",
			Name:      "root-span",
			Kind:      "server",
			StartTime: baseTime,
			EndTime:   baseTime.Add(200 * time.Millisecond),
			Status: observability.SpanStatus{
				Code: observability.StatusCodeOK,
			},
			Attributes: map[string]any{"operation": "test"},
		},
		{
			TraceID:   traceID,
			SpanID:    "span2",
			ParentID:  "span1",
			Name:      "child-span",
			Kind:      "internal",
			StartTime: baseTime.Add(10 * time.Millisecond),
			EndTime:   baseTime.Add(100 * time.Millisecond),
			Status: observability.SpanStatus{
				Code: observability.StatusCodeOK,
			},
			Attributes: map[string]any{},
		},
	}

	for _, span := range spans {
		if err := store.StoreSpan(ctx, span); err != nil {
			t.Fatalf("failed to store test span: %v", err)
		}
	}

	tests := []struct {
		name           string
		traceID        string
		expectedStatus int
		expectedSpans  int
	}{
		{
			name:           "get existing trace",
			traceID:        traceID,
			expectedStatus: http.StatusOK,
			expectedSpans:  2,
		},
		{
			name:           "trace not found",
			traceID:        "nonexistent-trace",
			expectedStatus: http.StatusNotFound,
			expectedSpans:  0,
		},
		{
			name:           "empty trace ID",
			traceID:        "",
			expectedStatus: http.StatusBadRequest,
			expectedSpans:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/traces/"+tt.traceID, nil)
			req.SetPathValue("id", tt.traceID)
			rec := httptest.NewRecorder()

			handler.GetTrace(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if response["trace_id"] != tt.traceID {
					t.Errorf("expected trace_id %q, got %q", tt.traceID, response["trace_id"])
				}

				spans, ok := response["spans"].([]interface{})
				if !ok {
					t.Fatalf("expected spans array in response")
				}

				if len(spans) != tt.expectedSpans {
					t.Errorf("expected %d spans, got %d", tt.expectedSpans, len(spans))
				}

				spanCount, ok := response["span_count"].(float64)
				if !ok {
					t.Fatalf("expected span_count in response")
				}

				if int(spanCount) != tt.expectedSpans {
					t.Errorf("expected span_count %d, got %d", tt.expectedSpans, int(spanCount))
				}
			}
		})
	}
}

func TestTracesHandler_GetTrace_NotFound(t *testing.T) {
	store := setupTestStore(t)
	handler := NewTracesHandler(store)

	req := httptest.NewRequest("GET", "/v1/traces/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()

	handler.GetTrace(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	body := rec.Body.String()
	if body != "trace not found\n" {
		t.Errorf("unexpected error message: %s", body)
	}
}

func TestTracesHandler_GetRunTrace(t *testing.T) {
	store := setupTestStore(t)
	handler := NewTracesHandler(store)

	ctx := context.Background()

	// Create a test trace with run_id in attributes
	traceID := "test-trace-with-run"
	runID := "run-12345"
	baseTime := time.Now()

	span := &observability.Span{
		TraceID:   traceID,
		SpanID:    "span1",
		Name:      "test-span",
		Kind:      "server",
		StartTime: baseTime,
		EndTime:   baseTime.Add(100 * time.Millisecond),
		Status: observability.SpanStatus{
			Code: observability.StatusCodeOK,
		},
		Attributes: map[string]any{
			"run_id": runID,
		},
	}

	if err := store.StoreSpan(ctx, span); err != nil {
		t.Fatalf("failed to store test span: %v", err)
	}

	tests := []struct {
		name           string
		runID          string
		expectedStatus int
		expectedTrace  string
	}{
		{
			name:           "get trace by run ID",
			runID:          runID,
			expectedStatus: http.StatusOK,
			expectedTrace:  traceID,
		},
		{
			name:           "run ID not found",
			runID:          "nonexistent-run",
			expectedStatus: http.StatusNotFound,
			expectedTrace:  "",
		},
		{
			name:           "empty run ID",
			runID:          "",
			expectedStatus: http.StatusBadRequest,
			expectedTrace:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/runs/"+tt.runID+"/trace", nil)
			req.SetPathValue("id", tt.runID)
			rec := httptest.NewRecorder()

			handler.GetRunTrace(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if response["run_id"] != tt.runID {
					t.Errorf("expected run_id %q, got %q", tt.runID, response["run_id"])
				}

				if response["trace_id"] != tt.expectedTrace {
					t.Errorf("expected trace_id %q, got %q", tt.expectedTrace, response["trace_id"])
				}

				spans, ok := response["spans"].([]interface{})
				if !ok {
					t.Fatalf("expected spans array in response")
				}

				if len(spans) == 0 {
					t.Error("expected at least one span in response")
				}
			}
		})
	}
}

func TestTracesHandler_GetTraceSpans(t *testing.T) {
	store := setupTestStore(t)
	handler := NewTracesHandler(store)

	ctx := context.Background()

	// Create a test trace
	traceID := "test-trace-spans"
	baseTime := time.Now()

	spans := []*observability.Span{
		{
			TraceID:   traceID,
			SpanID:    "span1",
			Name:      "span-1",
			Kind:      "server",
			StartTime: baseTime,
			EndTime:   baseTime.Add(100 * time.Millisecond),
			Status: observability.SpanStatus{
				Code: observability.StatusCodeOK,
			},
			Attributes: map[string]any{},
		},
		{
			TraceID:   traceID,
			SpanID:    "span2",
			ParentID:  "span1",
			Name:      "span-2",
			Kind:      "internal",
			StartTime: baseTime.Add(10 * time.Millisecond),
			EndTime:   baseTime.Add(50 * time.Millisecond),
			Status: observability.SpanStatus{
				Code: observability.StatusCodeOK,
			},
			Attributes: map[string]any{},
		},
	}

	for _, span := range spans {
		if err := store.StoreSpan(ctx, span); err != nil {
			t.Fatalf("failed to store test span: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/v1/traces/"+traceID+"/spans", nil)
	req.SetPathValue("id", traceID)
	rec := httptest.NewRecorder()

	handler.GetTraceSpans(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["trace_id"] != traceID {
		t.Errorf("expected trace_id %q, got %q", traceID, response["trace_id"])
	}

	responseSpans, ok := response["spans"].([]interface{})
	if !ok {
		t.Fatalf("expected spans array in response")
	}

	if len(responseSpans) != 2 {
		t.Errorf("expected 2 spans, got %d", len(responseSpans))
	}

	count, ok := response["count"].(float64)
	if !ok {
		t.Fatalf("expected count in response")
	}

	if int(count) != 2 {
		t.Errorf("expected count 2, got %d", int(count))
	}
}

func TestEventsHandler_ListEvents(t *testing.T) {
	store := setupTestStore(t)
	handler := NewEventsHandler(store)

	ctx := context.Background()

	// Create a test trace with events
	traceID := "test-trace-events"
	baseTime := time.Now()

	span := &observability.Span{
		TraceID:   traceID,
		SpanID:    "span1",
		Name:      "test-span",
		Kind:      "server",
		StartTime: baseTime,
		EndTime:   baseTime.Add(100 * time.Millisecond),
		Status: observability.SpanStatus{
			Code: observability.StatusCodeOK,
		},
		Attributes: map[string]any{},
		Events: []observability.Event{
			{
				Name:       "event1",
				Timestamp:  baseTime.Add(10 * time.Millisecond),
				Attributes: map[string]any{"key": "value1"},
			},
			{
				Name:       "event2",
				Timestamp:  baseTime.Add(50 * time.Millisecond),
				Attributes: map[string]any{"key": "value2"},
			},
		},
	}

	if err := store.StoreSpan(ctx, span); err != nil {
		t.Fatalf("failed to store test span: %v", err)
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedCount  int
		expectedStatus int
		skipEventCheck bool
	}{
		{
			name:           "list all events for trace",
			queryParams:    "trace_id=" + traceID,
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name: "filter by since",
			// Since filter excludes events before the time, so we use a time well after event1 (10ms) but before event2 (50ms)
			queryParams:    "trace_id=" + traceID + "&since=" + baseTime.Add(11*time.Millisecond).Format(time.RFC3339Nano),
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "no trace_id specified",
			queryParams:    "",
			expectedCount:  0,
			expectedStatus: http.StatusOK,
			skipEventCheck: true, // events field might not be present
		},
		{
			name:           "invalid since parameter",
			queryParams:    "trace_id=" + traceID + "&since=invalid",
			expectedCount:  0,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/events?"+tt.queryParams, nil)
			rec := httptest.NewRecorder()

			handler.ListEvents(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if !tt.skipEventCheck {
					events, ok := response["events"].([]interface{})
					if !ok {
						t.Fatalf("expected events array in response")
					}

					if len(events) != tt.expectedCount {
						t.Errorf("expected %d events, got %d", tt.expectedCount, len(events))
					}

					count, ok := response["count"].(float64)
					if !ok {
						t.Fatalf("expected count in response")
					}

					if int(count) != tt.expectedCount {
						t.Errorf("expected count %d, got %d", tt.expectedCount, int(count))
					}
				}
			}
		})
	}
}
