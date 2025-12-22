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

package mcp

import (
	"log/slog"
	"time"
)

// EventType represents the type of MCP server event.
type EventType string

const (
	// EventStarted indicates a server has started.
	EventStarted EventType = "started"
	// EventStopped indicates a server has stopped.
	EventStopped EventType = "stopped"
	// EventFailed indicates a server has failed.
	EventFailed EventType = "failed"
	// EventRestarting indicates a server is restarting.
	EventRestarting EventType = "restarting"
	// EventToolsChanged indicates the server's tool list has changed.
	EventToolsChanged EventType = "tools_changed"
	// EventHealthy indicates a server has become healthy.
	EventHealthy EventType = "healthy"
	// EventUnhealthy indicates a server has become unhealthy.
	EventUnhealthy EventType = "unhealthy"
)

// MCPServerEvent represents an event from an MCP server.
type MCPServerEvent struct {
	// Type is the event type.
	Type EventType `json:"type"`

	// ServerName is the name of the server.
	ServerName string `json:"server_name"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Message is an optional human-readable message.
	Message string `json:"message,omitempty"`

	// Details contains additional event-specific information.
	Details map[string]any `json:"details,omitempty"`
}

// EventEmitter emits MCP server events.
type EventEmitter struct {
	logger *slog.Logger
}

// NewEventEmitter creates a new event emitter.
func NewEventEmitter(logger *slog.Logger) *EventEmitter {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventEmitter{logger: logger}
}

// Emit logs an event. In the future, this could also send to event subscribers.
func (e *EventEmitter) Emit(event MCPServerEvent) {
	// Log the event
	attrs := []any{
		"server", event.ServerName,
		"type", string(event.Type),
	}

	if event.Message != "" {
		attrs = append(attrs, "message", event.Message)
	}

	for k, v := range event.Details {
		attrs = append(attrs, k, v)
	}

	e.logger.Info("MCP server event", attrs...)
}

// EmitStarted emits a server started event.
func (e *EventEmitter) EmitStarted(serverName string) {
	e.Emit(MCPServerEvent{
		Type:       EventStarted,
		ServerName: serverName,
		Timestamp:  time.Now(),
		Message:    "Server started successfully",
	})
}

// EmitStopped emits a server stopped event.
func (e *EventEmitter) EmitStopped(serverName string) {
	e.Emit(MCPServerEvent{
		Type:       EventStopped,
		ServerName: serverName,
		Timestamp:  time.Now(),
		Message:    "Server stopped",
	})
}

// EmitFailed emits a server failed event.
func (e *EventEmitter) EmitFailed(serverName string, err error) {
	e.Emit(MCPServerEvent{
		Type:       EventFailed,
		ServerName: serverName,
		Timestamp:  time.Now(),
		Message:    "Server failed",
		Details: map[string]any{
			"error": err.Error(),
		},
	})
}

// EmitRestarting emits a server restarting event.
func (e *EventEmitter) EmitRestarting(serverName string, attempt int) {
	e.Emit(MCPServerEvent{
		Type:       EventRestarting,
		ServerName: serverName,
		Timestamp:  time.Now(),
		Message:    "Server restarting",
		Details: map[string]any{
			"attempt": attempt,
		},
	})
}

// EmitToolsChanged emits a tools changed event.
func (e *EventEmitter) EmitToolsChanged(serverName string, toolCount int) {
	e.Emit(MCPServerEvent{
		Type:       EventToolsChanged,
		ServerName: serverName,
		Timestamp:  time.Now(),
		Message:    "Server tools changed",
		Details: map[string]any{
			"tool_count": toolCount,
		},
	})
}

// EmitHealthy emits a server healthy event.
func (e *EventEmitter) EmitHealthy(serverName string) {
	e.Emit(MCPServerEvent{
		Type:       EventHealthy,
		ServerName: serverName,
		Timestamp:  time.Now(),
		Message:    "Server is healthy",
	})
}

// EmitUnhealthy emits a server unhealthy event.
func (e *EventEmitter) EmitUnhealthy(serverName string, reason string) {
	e.Emit(MCPServerEvent{
		Type:       EventUnhealthy,
		ServerName: serverName,
		Timestamp:  time.Now(),
		Message:    "Server is unhealthy",
		Details: map[string]any{
			"reason": reason,
		},
	})
}
