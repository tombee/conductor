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

package storage

import (
	"context"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/observability"
)

func TestSQLiteStore_StoreAndGetSpan(t *testing.T) {
	store, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a test span
	span := &observability.Span{
		TraceID:   "trace-123",
		SpanID:    "span-456",
		ParentID:  "",
		Name:      "test-operation",
		Kind:      observability.SpanKindInternal,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(100 * time.Millisecond),
		Status: observability.SpanStatus{
			Code:    observability.StatusCodeOK,
			Message: "success",
		},
		Attributes: map[string]any{
			"key1": "value1",
			"key2": 42,
		},
		Events: []observability.Event{
			{
				Name:      "checkpoint",
				Timestamp: time.Now(),
				Attributes: map[string]any{
					"stage": "initialization",
				},
			},
		},
	}

	// Store the span
	if err := store.StoreSpan(ctx, span); err != nil {
		t.Fatalf("failed to store span: %v", err)
	}

	// Retrieve the span
	retrieved, err := store.GetSpan(ctx, span.TraceID, span.SpanID)
	if err != nil {
		t.Fatalf("failed to get span: %v", err)
	}

	// Verify span fields
	if retrieved.TraceID != span.TraceID {
		t.Errorf("expected trace_id %s, got %s", span.TraceID, retrieved.TraceID)
	}
	if retrieved.SpanID != span.SpanID {
		t.Errorf("expected span_id %s, got %s", span.SpanID, retrieved.SpanID)
	}
	if retrieved.Name != span.Name {
		t.Errorf("expected name %s, got %s", span.Name, retrieved.Name)
	}
	if retrieved.Status.Code != span.Status.Code {
		t.Errorf("expected status code %d, got %d", span.Status.Code, retrieved.Status.Code)
	}

	// Verify attributes
	if retrieved.Attributes["key1"] != "value1" {
		t.Errorf("expected attribute key1=value1, got %v", retrieved.Attributes["key1"])
	}

	// Verify events
	if len(retrieved.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(retrieved.Events))
	}
	if retrieved.Events[0].Name != "checkpoint" {
		t.Errorf("expected event name 'checkpoint', got %s", retrieved.Events[0].Name)
	}
}

func TestSQLiteStore_GetTraceSpans(t *testing.T) {
	store, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	traceID := "trace-789"

	// Create a root span and child span
	rootSpan := &observability.Span{
		TraceID:   traceID,
		SpanID:    "span-root",
		Name:      "root-operation",
		Kind:      observability.SpanKindInternal,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(200 * time.Millisecond),
		Status: observability.SpanStatus{
			Code: observability.StatusCodeOK,
		},
	}

	childSpan := &observability.Span{
		TraceID:   traceID,
		SpanID:    "span-child",
		ParentID:  "span-root",
		Name:      "child-operation",
		Kind:      observability.SpanKindInternal,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(50 * time.Millisecond),
		Status: observability.SpanStatus{
			Code: observability.StatusCodeOK,
		},
	}

	// Store both spans
	if err := store.StoreSpan(ctx, rootSpan); err != nil {
		t.Fatalf("failed to store root span: %v", err)
	}
	if err := store.StoreSpan(ctx, childSpan); err != nil {
		t.Fatalf("failed to store child span: %v", err)
	}

	// Retrieve all spans for the trace
	spans, err := store.GetTraceSpans(ctx, traceID)
	if err != nil {
		t.Fatalf("failed to get trace spans: %v", err)
	}

	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	// Verify parent-child relationship
	var foundRoot, foundChild bool
	for _, span := range spans {
		if span.SpanID == "span-root" {
			foundRoot = true
			if span.ParentID != "" {
				t.Errorf("root span should have no parent, got %s", span.ParentID)
			}
		}
		if span.SpanID == "span-child" {
			foundChild = true
			if span.ParentID != "span-root" {
				t.Errorf("child span should have parent span-root, got %s", span.ParentID)
			}
		}
	}

	if !foundRoot || !foundChild {
		t.Errorf("did not find both root and child spans")
	}
}

func TestSQLiteStore_ListTraces(t *testing.T) {
	store, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create spans for multiple traces
	traces := []string{"trace-1", "trace-2", "trace-3"}
	for _, traceID := range traces {
		span := &observability.Span{
			TraceID:   traceID,
			SpanID:    "span-" + traceID,
			Name:      "operation-" + traceID,
			Kind:      observability.SpanKindInternal,
			StartTime: time.Now(),
			EndTime:   time.Now().Add(10 * time.Millisecond),
			Status: observability.SpanStatus{
				Code: observability.StatusCodeOK,
			},
		}
		if err := store.StoreSpan(ctx, span); err != nil {
			t.Fatalf("failed to store span for %s: %v", traceID, err)
		}
	}

	// List all traces
	traceIDs, err := store.ListTraces(ctx, TraceFilter{})
	if err != nil {
		t.Fatalf("failed to list traces: %v", err)
	}

	if len(traceIDs) != 3 {
		t.Fatalf("expected 3 traces, got %d", len(traceIDs))
	}

	// Test limit
	traceIDs, err = store.ListTraces(ctx, TraceFilter{Limit: 2})
	if err != nil {
		t.Fatalf("failed to list traces with limit: %v", err)
	}

	if len(traceIDs) != 2 {
		t.Fatalf("expected 2 traces with limit, got %d", len(traceIDs))
	}
}

func TestSQLiteStore_DeleteTracesOlderThan(t *testing.T) {
	store, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create old and new spans
	oldSpan := &observability.Span{
		TraceID:   "trace-old",
		SpanID:    "span-old",
		Name:      "old-operation",
		Kind:      observability.SpanKindInternal,
		StartTime: time.Now().Add(-48 * time.Hour), // 2 days ago
		EndTime:   time.Now().Add(-48 * time.Hour).Add(10 * time.Millisecond),
		Status: observability.SpanStatus{
			Code: observability.StatusCodeOK,
		},
	}

	newSpan := &observability.Span{
		TraceID:   "trace-new",
		SpanID:    "span-new",
		Name:      "new-operation",
		Kind:      observability.SpanKindInternal,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(10 * time.Millisecond),
		Status: observability.SpanStatus{
			Code: observability.StatusCodeOK,
		},
	}

	if err := store.StoreSpan(ctx, oldSpan); err != nil {
		t.Fatalf("failed to store old span: %v", err)
	}
	if err := store.StoreSpan(ctx, newSpan); err != nil {
		t.Fatalf("failed to store new span: %v", err)
	}

	// Delete traces older than 24 hours
	deleted, err := store.DeleteTracesOlderThan(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("failed to delete old traces: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 trace deleted, got %d", deleted)
	}

	// Verify only new trace remains
	traceIDs, err := store.ListTraces(ctx, TraceFilter{})
	if err != nil {
		t.Fatalf("failed to list traces: %v", err)
	}

	if len(traceIDs) != 1 {
		t.Fatalf("expected 1 trace remaining, got %d", len(traceIDs))
	}

	if traceIDs[0] != "trace-new" {
		t.Errorf("expected trace-new to remain, got %s", traceIDs[0])
	}
}

func TestSQLiteStore_UpdateSpan(t *testing.T) {
	store, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create an active span (no end time)
	span := &observability.Span{
		TraceID:   "trace-update",
		SpanID:    "span-update",
		Name:      "updating-operation",
		Kind:      observability.SpanKindInternal,
		StartTime: time.Now(),
		Status: observability.SpanStatus{
			Code: observability.StatusCodeUnset,
		},
	}

	// Store the span
	if err := store.StoreSpan(ctx, span); err != nil {
		t.Fatalf("failed to store span: %v", err)
	}

	// Update the span with end time and status
	span.EndTime = time.Now().Add(100 * time.Millisecond)
	span.Status.Code = observability.StatusCodeOK
	span.Status.Message = "completed"

	if err := store.StoreSpan(ctx, span); err != nil {
		t.Fatalf("failed to update span: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetSpan(ctx, span.TraceID, span.SpanID)
	if err != nil {
		t.Fatalf("failed to get updated span: %v", err)
	}

	if retrieved.EndTime.IsZero() {
		t.Error("expected end time to be set after update")
	}
	if retrieved.Status.Code != observability.StatusCodeOK {
		t.Errorf("expected status OK after update, got %d", retrieved.Status.Code)
	}
	if retrieved.Status.Message != "completed" {
		t.Errorf("expected status message 'completed', got %s", retrieved.Status.Message)
	}
}
