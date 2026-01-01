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
	"encoding/json"
	"net/http"
	"time"

	"github.com/tombee/conductor/internal/tracing/storage"
	"github.com/tombee/conductor/pkg/observability"
)

// TracesHandler provides HTTP handlers for trace access.
type TracesHandler struct {
	store *storage.SQLiteStore
}

// NewTracesHandler creates a new traces API handler.
func NewTracesHandler(store *storage.SQLiteStore) *TracesHandler {
	return &TracesHandler{
		store: store,
	}
}

// RegisterRoutes registers trace API routes on the provided router.
func (h *TracesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/traces", h.ListTraces)
	mux.HandleFunc("GET /v1/traces/{id}", h.GetTrace)
	mux.HandleFunc("GET /v1/traces/{id}/spans", h.GetTraceSpans)
	mux.HandleFunc("GET /v1/runs/{id}/trace", h.GetRunTrace)
}

// ListTraces handles GET /v1/traces with filtering support.
func (h *TracesHandler) ListTraces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	query := r.URL.Query()
	filter := storage.TraceFilter{
		Limit: 100, // Default limit
	}

	// Parse since filter
	if since := query.Get("since"); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			http.Error(w, "invalid since parameter", http.StatusBadRequest)
			return
		}
		filter.Since = &t
	}

	// Parse until filter
	if until := query.Get("until"); until != "" {
		t, err := time.Parse(time.RFC3339, until)
		if err != nil {
			http.Error(w, "invalid until parameter", http.StatusBadRequest)
			return
		}
		filter.Until = &t
	}

	// Parse status filter
	if status := query.Get("status"); status != "" {
		// Convert string to StatusCode
		var statusCode observability.StatusCode
		switch status {
		case "ok":
			statusCode = observability.StatusCodeOK
		case "error":
			statusCode = observability.StatusCodeError
		default:
			statusCode = observability.StatusCodeUnset
		}
		filter.Status = &statusCode
	}

	// Execute query
	traces, err := h.store.ListTraces(ctx, filter)
	if err != nil {
		http.Error(w, "failed to list traces", http.StatusInternalServerError)
		return
	}

	// Return results
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"traces": traces,
		"count":  len(traces),
	})
}

// GetTrace handles GET /v1/traces/{id} to retrieve trace details.
func (h *TracesHandler) GetTrace(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	traceID := r.PathValue("id")

	if traceID == "" {
		http.Error(w, "trace ID is required", http.StatusBadRequest)
		return
	}

	// Get all spans for this trace
	spans, err := h.store.GetTraceSpans(ctx, traceID)
	if err != nil {
		http.Error(w, "failed to get trace", http.StatusInternalServerError)
		return
	}

	if len(spans) == 0 {
		http.Error(w, "trace not found", http.StatusNotFound)
		return
	}

	// Return trace with all spans
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trace_id":   traceID,
		"spans":      spans,
		"span_count": len(spans),
	})
}

// GetTraceSpans handles GET /v1/traces/{id}/spans to retrieve trace spans.
func (h *TracesHandler) GetTraceSpans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	traceID := r.PathValue("id")

	if traceID == "" {
		http.Error(w, "trace ID is required", http.StatusBadRequest)
		return
	}

	// Get all spans for this trace
	spans, err := h.store.GetTraceSpans(ctx, traceID)
	if err != nil {
		http.Error(w, "failed to get spans", http.StatusInternalServerError)
		return
	}

	// Return spans
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trace_id": traceID,
		"spans":    spans,
		"count":    len(spans),
	})
}

// GetRunTrace handles GET /v1/runs/{id}/trace to link run to trace.
// This requires the run ID to be stored in span attributes.
func (h *TracesHandler) GetRunTrace(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	runID := r.PathValue("id")

	if runID == "" {
		http.Error(w, "run ID is required", http.StatusBadRequest)
		return
	}

	// Look up trace ID by run ID
	traceID, err := h.store.GetTraceByRunID(ctx, runID)
	if err != nil {
		http.Error(w, "failed to get trace for run", http.StatusInternalServerError)
		return
	}

	if traceID == "" {
		http.Error(w, "no trace found for run", http.StatusNotFound)
		return
	}

	// Get all spans for this trace
	spans, err := h.store.GetTraceSpans(ctx, traceID)
	if err != nil {
		http.Error(w, "failed to get trace", http.StatusInternalServerError)
		return
	}

	// Return trace with all spans
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"run_id":     runID,
		"trace_id":   traceID,
		"spans":      spans,
		"span_count": len(spans),
	})
}

// TraceResponse represents a trace with metadata.
type TraceResponse struct {
	TraceID   string                `json:"trace_id"`
	StartTime time.Time             `json:"start_time"`
	EndTime   time.Time             `json:"end_time"`
	Duration  time.Duration         `json:"duration_ms"`
	Status    string                `json:"status"`
	Spans     []*observability.Span `json:"spans,omitempty"`
	SpanCount int                   `json:"span_count"`
}
