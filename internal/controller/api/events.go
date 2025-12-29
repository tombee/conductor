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
	"fmt"
	"net/http"
	"time"

	"github.com/tombee/conductor/internal/tracing/storage"
)

// EventsHandler provides HTTP handlers for event access.
type EventsHandler struct {
	store *storage.SQLiteStore
}

// NewEventsHandler creates a new events API handler.
func NewEventsHandler(store *storage.SQLiteStore) *EventsHandler {
	return &EventsHandler{
		store: store,
	}
}

// RegisterRoutes registers event API routes on the provided router.
func (h *EventsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/events", h.ListEvents)
	mux.HandleFunc("GET /v1/events/stream", h.StreamEvents)
}

// ListEvents handles GET /v1/events with filtering support.
func (h *EventsHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	query := r.URL.Query()
	limit := 100 // Default limit
	var since *time.Time
	var traceID *string

	// Parse trace ID filter
	if tid := query.Get("trace_id"); tid != "" {
		traceID = &tid
	}

	// Parse since filter
	if s := query.Get("since"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			http.Error(w, "invalid since parameter", http.StatusBadRequest)
			return
		}
		since = &t
	}

	// Get events from storage
	// For simplicity, we're returning events from trace spans
	// In a full implementation, this would query a dedicated events table
	var events []EventResponse

	if traceID != nil {
		// Get spans for this trace
		spans, err := h.store.GetTraceSpans(ctx, *traceID)
		if err != nil {
			http.Error(w, "failed to get events", http.StatusInternalServerError)
			return
		}

		// Extract events from spans
		for _, span := range spans {
			for _, event := range span.Events {
				// Filter by since if provided
				if since != nil && event.Timestamp.Before(*since) {
					continue
				}

				events = append(events, EventResponse{
					TraceID:    span.TraceID,
					SpanID:     span.SpanID,
					Name:       event.Name,
					Timestamp:  event.Timestamp,
					Attributes: event.Attributes,
				})

				if len(events) >= limit {
					break
				}
			}
			if len(events) >= limit {
				break
			}
		}
	}

	// Return results
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// StreamEvents handles GET /v1/events/stream for Server-Sent Events (SSE).
// This provides real-time event streaming.
func (h *EventsHandler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Send initial connection message
	fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	flusher.Flush()

	// In a full implementation, this would:
	// 1. Subscribe to a pub/sub channel for new events
	// 2. Stream events as they arrive
	// 3. Handle client disconnection via ctx.Done()

	// For now, send a simple heartbeat
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case <-ticker.C:
			// Send heartbeat
			fmt.Fprintf(w, "data: {\"type\":\"heartbeat\"}\n\n")
			flusher.Flush()
		}
	}
}

// EventResponse represents an event with metadata.
type EventResponse struct {
	TraceID    string         `json:"trace_id"`
	SpanID     string         `json:"span_id"`
	Name       string         `json:"name"`
	Timestamp  time.Time      `json:"timestamp"`
	Attributes map[string]any `json:"attributes,omitempty"`
}
