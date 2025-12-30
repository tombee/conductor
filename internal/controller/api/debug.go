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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/controller/debug"
)

const (
	// SSE heartbeat interval
	sseHeartbeatInterval = 30 * time.Second

	// Maximum concurrent SSE connections per session
	maxSSEConnections = 1000
)

// DebugHandler handles debug-related API requests.
type DebugHandler struct {
	sessionManager  *debug.SessionManager
	activeSSEConnections int
}

// NewDebugHandler creates a new debug handler.
func NewDebugHandler(sessionManager *debug.SessionManager) *DebugHandler {
	return &DebugHandler{
		sessionManager: sessionManager,
	}
}

// RegisterRoutes registers debug API routes on the router.
func (h *DebugHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/runs/{id}/debug/events", h.handleDebugEvents)
	mux.HandleFunc("POST /v1/runs/{id}/debug/command", h.handleDebugCommand)
}

// handleDebugEvents handles GET /v1/runs/{id}/debug/events (SSE endpoint).
func (h *DebugHandler) handleDebugEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	runID := r.PathValue("id")

	// Enforce TLS for non-localhost connections
	if !isLocalhost(r) && !isTLS(r) {
		writeError(w, http.StatusForbidden, "TLS required for non-localhost debug connections")
		return
	}

	// Check Bearer token authentication
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeError(w, http.StatusUnauthorized, "authorization required")
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		writeError(w, http.StatusUnauthorized, "invalid authorization header format")
		return
	}

	// Get session ID from query parameter
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id query parameter required")
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session not found: %s", sessionID))
		return
	}

	// Verify session belongs to this run
	if session.RunID != runID {
		writeError(w, http.StatusForbidden, "session does not belong to this run")
		return
	}

	// Check if we've reached the connection limit
	if h.activeSSEConnections >= maxSSEConnections {
		writeError(w, http.StatusServiceUnavailable, "maximum SSE connections reached")
		return
	}

	// Generate observer ID from token (in real implementation, decode JWT)
	observerID := fmt.Sprintf("observer-%s", token[:8])

	// Check if observer already exists
	isExisting, isOwner := h.sessionManager.IsObserver(sessionID, observerID)

	// Add observer if not already connected
	if !isExisting {
		// First connection is owner, others are read-only observers
		isOwner = false
		count, _ := h.sessionManager.GetObserverCount(sessionID)
		if count == 0 {
			isOwner = true
		}

		if err := h.sessionManager.AddObserver(sessionID, observerID, isOwner); err != nil {
			writeError(w, http.StatusTooManyRequests, err.Error())
			return
		}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Increment active connections
	h.activeSSEConnections++
	defer func() {
		h.activeSSEConnections--
		h.sessionManager.RemoveObserver(sessionID, observerID)
	}()

	// Send existing event buffer for reconnection support
	eventBuffer, err := h.sessionManager.GetEventBuffer(sessionID)
	if err == nil {
		for _, event := range eventBuffer {
			sseData, err := event.ToSSE()
			if err != nil {
				continue
			}
			fmt.Fprint(w, sseData)
			flusher.Flush()
		}
	}

	// Create heartbeat ticker
	heartbeatTicker := time.NewTicker(sseHeartbeatInterval)
	defer heartbeatTicker.Stop()

	// Create event channel for this observer
	eventChan := make(chan debug.Event, 10)
	defer close(eventChan)

	// Start goroutine to broadcast events to this observer
	// In a real implementation, this would use a pub/sub system
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				// Poll for new events
				// In production, this should use channels or pub/sub
				buffer, err := h.sessionManager.GetEventBuffer(sessionID)
				if err != nil {
					continue
				}
				// Send only new events (simplified - should track last event ID)
				if len(buffer) > 0 {
					lastEvent := buffer[len(buffer)-1]
					select {
					case eventChan <- lastEvent:
					default:
					}
				}
			}
		}
	}()

	// Stream events
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return

		case <-heartbeatTicker.C:
			// Send heartbeat
			event := debug.NewHeartbeatEvent()
			sseData, err := event.ToSSE()
			if err != nil {
				continue
			}
			fmt.Fprint(w, sseData)
			flusher.Flush()

		case event := <-eventChan:
			// Send debug event
			sseData, err := event.ToSSE()
			if err != nil {
				continue
			}
			fmt.Fprint(w, sseData)
			flusher.Flush()
		}
	}
}

// handleDebugCommand handles POST /v1/runs/{id}/debug/command.
func (h *DebugHandler) handleDebugCommand(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	runID := r.PathValue("id")

	// Check Bearer token authentication
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeError(w, http.StatusUnauthorized, "authorization required")
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		writeError(w, http.StatusUnauthorized, "invalid authorization header format")
		return
	}

	// Get session ID from query parameter
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id query parameter required")
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session not found: %s", sessionID))
		return
	}

	// Verify session belongs to this run
	if session.RunID != runID {
		writeError(w, http.StatusForbidden, "session does not belong to this run")
		return
	}

	// Generate observer ID from token
	observerID := fmt.Sprintf("observer-%s", token[:8])

	// Check if user is the session owner
	isObserver, isOwner := h.sessionManager.IsObserver(sessionID, observerID)
	if !isObserver {
		writeError(w, http.StatusForbidden, "not connected to this session")
		return
	}

	if !isOwner {
		writeError(w, http.StatusForbidden, "only session owner can send commands")
		return
	}

	// Parse command from request body
	var cmd debug.Command
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &cmd); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid command format: %s", err))
		return
	}

	// Validate command type
	validCommands := map[debug.CommandType]bool{
		debug.CommandContinue: true,
		debug.CommandNext:     true,
		debug.CommandSkip:     true,
		debug.CommandAbort:    true,
		debug.CommandInspect:  true,
		debug.CommandContext:  true,
	}

	if !validCommands[cmd.Type] {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid command type: %s", cmd.Type))
		return
	}

	// Validate command against current session state
	if err := h.validateCommand(session, cmd); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Send command to session
	if err := h.sessionManager.SendCommand(sessionID, cmd); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send command: %s", err))
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "acknowledged",
		"command": cmd.Type,
	})
}

// validateCommand validates a command against the current session state.
func (h *DebugHandler) validateCommand(session *debug.DebugSession, cmd debug.Command) error {
	// Can only send commands when session is paused
	if session.State != debug.SessionStatePaused {
		return fmt.Errorf("commands can only be sent when session is paused (current state: %s)", session.State)
	}

	// Validate specific commands
	switch cmd.Type {
	case debug.CommandContinue, debug.CommandNext, debug.CommandSkip:
		// These commands resume execution - valid when paused
		return nil

	case debug.CommandAbort:
		// Can abort at any time when paused
		return nil

	case debug.CommandInspect, debug.CommandContext:
		// These are read-only commands - always valid when paused
		return nil

	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// isLocalhost checks if the request is from localhost.
func isLocalhost(r *http.Request) bool {
	host := r.Host
	if colonIdx := strings.Index(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// isTLS checks if the request is over TLS.
func isTLS(r *http.Request) bool {
	// Check TLS connection state
	if r.TLS != nil && r.TLS.Version != 0 {
		return true
	}

	// Check X-Forwarded-Proto header (for reverse proxies)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
		return true
	}

	return false
}

// Helper function to get TLS state
func getTLSState(r *http.Request) *tls.ConnectionState {
	return r.TLS
}
