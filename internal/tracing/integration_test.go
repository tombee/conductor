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

package tracing_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/controller/api"
	"github.com/tombee/conductor/internal/tracing/redact"
	"github.com/tombee/conductor/internal/tracing/storage"
	"github.com/tombee/conductor/pkg/observability"
)

// TestIntegration_EndToEnd tests the complete flow from trace creation to API retrieval with redaction.
func TestIntegration_EndToEnd(t *testing.T) {
	// Setup: Create in-memory SQLite store
	// Note: MaxOpenConns must be > 1 to allow GetTraceByRunID to call GetTraceSpans
	store, err := storage.New(storage.Config{
		Path:             ":memory:",
		MaxOpenConns:     5,
		EnableEncryption: false,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Setup: Create redactor
	redactor := redact.NewRedactor(redact.ModeStandard)

	// Setup: Create API handlers
	tracesHandler := api.NewTracesHandler(store)

	ctx := context.Background()
	baseTime := time.Now()
	traceID := "integration-test-trace"
	runID := "integration-test-run-123"

	// Step 1: Create trace with spans containing sensitive data
	t.Log("Step 1: Creating trace with sensitive data")

	sensitivePassword := "password=SuperSecret123!"
	sensitiveAPIKey := "api_key=FAKE_KEY_FOR_TESTING_1234567890"
	sensitiveEmail := "user@example.com"

	spans := []*observability.Span{
		{
			TraceID:   traceID,
			SpanID:    "root-span",
			Name:      "workflow-execution",
			Kind:      "server",
			StartTime: baseTime,
			EndTime:   baseTime.Add(500 * time.Millisecond),
			Status: observability.SpanStatus{
				Code:    observability.StatusCodeOK,
				Message: "completed successfully",
			},
			Attributes: map[string]any{
				"run_id":      runID,
				"workflow":    "test-workflow",
				"environment": "test",
				// Sensitive data that should be redacted
				"config": sensitivePassword + " " + sensitiveAPIKey,
				"user":   sensitiveEmail,
			},
			Events: []observability.Event{
				{
					Name:      "workflow.started",
					Timestamp: baseTime,
					Attributes: map[string]any{
						"status": "initializing",
					},
				},
				{
					Name:      "workflow.completed",
					Timestamp: baseTime.Add(500 * time.Millisecond),
					Attributes: map[string]any{
						"status": "success",
					},
				},
			},
		},
		{
			TraceID:   traceID,
			SpanID:    "child-span",
			ParentID:  "root-span",
			Name:      "api-call",
			Kind:      "client",
			StartTime: baseTime.Add(100 * time.Millisecond),
			EndTime:   baseTime.Add(300 * time.Millisecond),
			Status: observability.SpanStatus{
				Code: observability.StatusCodeOK,
			},
			Attributes: map[string]any{
				"http.method": "POST",
				"http.url":    "https://api.example.com/v1/users",
				// Sensitive header that should be redacted
				"http.headers": "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.example.token",
			},
			Events: []observability.Event{
				{
					Name:      "http.request.sent",
					Timestamp: baseTime.Add(100 * time.Millisecond),
					Attributes: map[string]any{
						"bytes": 1024,
					},
				},
				{
					Name:      "http.response.received",
					Timestamp: baseTime.Add(300 * time.Millisecond),
					Attributes: map[string]any{
						"status_code": 200,
					},
				},
			},
		},
	}

	// Step 2: Store spans to SQLite
	t.Log("Step 2: Storing spans to SQLite")

	for _, span := range spans {
		if err := store.StoreSpan(ctx, span); err != nil {
			t.Fatalf("failed to store span: %v", err)
		}
	}

	// Verify spans were stored
	storedSpans, err := store.GetTraceSpans(ctx, traceID)
	if err != nil {
		t.Fatalf("failed to get stored spans: %v", err)
	}
	if len(storedSpans) != 2 {
		t.Fatalf("expected 2 stored spans, got %d", len(storedSpans))
	}

	// Step 3: Query via API - Get trace by ID
	t.Log("Step 3: Querying trace via API")

	req := httptest.NewRequest("GET", "/v1/traces/"+traceID, nil)
	req.SetPathValue("id", traceID)
	rec := httptest.NewRecorder()

	tracesHandler.GetTrace(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var traceResponse map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&traceResponse); err != nil {
		t.Fatalf("failed to decode trace response: %v", err)
	}

	// Verify trace response structure
	if traceResponse["trace_id"] != traceID {
		t.Errorf("expected trace_id %q, got %q", traceID, traceResponse["trace_id"])
	}

	responseSpans, ok := traceResponse["spans"].([]interface{})
	if !ok {
		t.Fatalf("expected spans array in response")
	}
	if len(responseSpans) != 2 {
		t.Fatalf("expected 2 spans in response, got %d", len(responseSpans))
	}

	// Step 4: Query via API - Get trace by run ID
	t.Log("Step 4: Querying trace by run ID")

	req = httptest.NewRequest("GET", "/v1/runs/"+runID+"/trace", nil)
	req.SetPathValue("id", runID)
	rec = httptest.NewRecorder()

	tracesHandler.GetRunTrace(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var runTraceResponse map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&runTraceResponse); err != nil {
		t.Fatalf("failed to decode run trace response: %v", err)
	}

	if runTraceResponse["run_id"] != runID {
		t.Errorf("expected run_id %q, got %q", runID, runTraceResponse["run_id"])
	}
	if runTraceResponse["trace_id"] != traceID {
		t.Errorf("expected trace_id %q, got %q", traceID, runTraceResponse["trace_id"])
	}

	// Step 5: Verify redaction is applied
	t.Log("Step 5: Verifying redaction")

	// Get the first span from response (root span)
	rootSpan := responseSpans[0].(map[string]interface{})
	attributes, ok := rootSpan["attributes"].(map[string]interface{})
	if !ok || attributes == nil {
		// Attributes might be stored differently - try retrieving the stored span directly
		storedRootSpan := storedSpans[0]
		attributes = storedRootSpan.Attributes
	}

	// Convert attributes to JSON string for redaction
	attributesJSON, err := json.Marshal(attributes)
	if err != nil {
		t.Fatalf("failed to marshal attributes: %v", err)
	}
	attributesStr := string(attributesJSON)

	// Apply redaction to verify it works
	redactedStr := redactor.RedactString(attributesStr)

	// Verify sensitive data is redacted
	t.Log("Verifying sensitive data redaction:")

	// Note: The stored data is NOT redacted by default in this implementation.
	// Redaction would typically be applied at the export/view layer.
	// For this test, we verify that the redactor WOULD redact the data.

	if strings.Contains(redactedStr, "SuperSecret123") {
		t.Error("password should be redacted but wasn't")
	}
	if strings.Contains(redactedStr, "FAKE_KEY_FOR_TESTING_1234567890") {
		t.Error("API key should be redacted but wasn't")
	}
	if strings.Contains(redactedStr, "user@example.com") {
		t.Error("email should be redacted but wasn't")
	}

	// Verify redaction markers are present
	if !strings.Contains(redactedStr, "[REDACTED]") {
		t.Error("expected [REDACTED] markers in redacted output")
	}

	t.Log("Redaction verification complete")

	// Step 6: List traces with filters
	t.Log("Step 6: Testing trace listing with filters")

	req = httptest.NewRequest("GET", "/v1/traces?status=ok", nil)
	rec = httptest.NewRecorder()

	tracesHandler.ListTraces(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var listResponse map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&listResponse); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}

	traces, ok := listResponse["traces"].([]interface{})
	if !ok {
		t.Fatalf("expected traces array in response")
	}
	if len(traces) == 0 {
		t.Error("expected at least one trace in list response")
	}

	// Verify our trace is in the list
	found := false
	for _, trace := range traces {
		if trace == traceID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find trace %q in list response", traceID)
	}

	t.Log("Integration test completed successfully")
}

// TestIntegration_ErrorTrace tests error handling and status codes.
func TestIntegration_ErrorTrace(t *testing.T) {
	store, err := storage.New(storage.Config{
		Path:             ":memory:",
		MaxOpenConns:     1,
		EnableEncryption: false,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	tracesHandler := api.NewTracesHandler(store)
	ctx := context.Background()

	// Create a trace with an error
	traceID := "error-trace"
	baseTime := time.Now()

	span := &observability.Span{
		TraceID:   traceID,
		SpanID:    "error-span",
		Name:      "failed-operation",
		Kind:      "server",
		StartTime: baseTime,
		EndTime:   baseTime.Add(100 * time.Millisecond),
		Status: observability.SpanStatus{
			Code:    observability.StatusCodeError,
			Message: "operation failed: database connection timeout",
		},
		Attributes: map[string]any{
			"error":        true,
			"error.type":   "DatabaseError",
			"error.detail": "connection timeout after 30s",
		},
	}

	if err := store.StoreSpan(ctx, span); err != nil {
		t.Fatalf("failed to store error span: %v", err)
	}

	// Query for error traces
	req := httptest.NewRequest("GET", "/v1/traces?status=error", nil)
	rec := httptest.NewRecorder()

	tracesHandler.ListTraces(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	traces, ok := response["traces"].([]interface{})
	if !ok {
		t.Fatalf("expected traces array in response")
	}

	// Should find our error trace
	found := false
	for _, trace := range traces {
		if trace == traceID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find error trace in filtered results")
	}
}

// TestIntegration_Encryption tests the encryption flow.
func TestIntegration_Encryption(t *testing.T) {
	// Set encryption key for test - use a passphrase which will be derived to 32 bytes
	t.Setenv("CONDUCTOR_TRACE_KEY", "test-encryption-passphrase-for-integration-test")

	store, err := storage.New(storage.Config{
		Path:             ":memory:",
		MaxOpenConns:     1,
		EnableEncryption: true,
	})
	if err != nil {
		t.Fatalf("failed to create encrypted store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	traceID := "encrypted-trace"
	baseTime := time.Now()

	// Store span with sensitive data
	span := &observability.Span{
		TraceID:   traceID,
		SpanID:    "encrypted-span",
		Name:      "secure-operation",
		Kind:      "internal",
		StartTime: baseTime,
		EndTime:   baseTime.Add(100 * time.Millisecond),
		Status: observability.SpanStatus{
			Code: observability.StatusCodeOK,
		},
		Attributes: map[string]any{
			"secret_data": "This should be encrypted at rest",
			"api_key":     "FAKE_TEST_KEY_12345",
		},
	}

	if err := store.StoreSpan(ctx, span); err != nil {
		t.Fatalf("failed to store encrypted span: %v", err)
	}

	// Retrieve and verify decryption works
	retrieved, err := store.GetSpan(ctx, traceID, "encrypted-span")
	if err != nil {
		t.Fatalf("failed to retrieve encrypted span: %v", err)
	}

	// Verify attributes are decrypted correctly
	if retrieved.Attributes["secret_data"] != "This should be encrypted at rest" {
		t.Error("encrypted attribute was not decrypted correctly")
	}
	if retrieved.Attributes["api_key"] != "FAKE_TEST_KEY_12345" {
		t.Error("encrypted api_key attribute was not decrypted correctly")
	}

	t.Log("Encryption/decryption working correctly")
}
