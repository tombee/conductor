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

// Package audit provides audit logging for observability API access.
package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Action represents an action taken on a resource
type Action string

const (
	ActionTracesRead   Action = "traces:read"
	ActionTracesDelete Action = "traces:delete"
	ActionEventsRead   Action = "events:read"
	ActionEventsStream Action = "events:stream"
	ActionExportersCfg Action = "exporters:configure"
	ActionReplayCreate Action = "replay:create"
	ActionReplayExec   Action = "replay:execute"
)

// Result represents the outcome of an audited action
type Result string

const (
	ResultSuccess      Result = "success"
	ResultUnauthorized Result = "unauthorized"
	ResultForbidden    Result = "forbidden"
	ResultNotFound     Result = "not_found"
	ResultError        Result = "error"
)

// Entry represents a single audit log entry
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id"`
	Action    Action    `json:"action"`
	Resource  string    `json:"resource"`
	Result    Result    `json:"result"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Error     string    `json:"error,omitempty"`

	// Replay-specific metadata
	ParentRunID  string   `json:"parent_run_id,omitempty"`
	FromStep     string   `json:"from_step,omitempty"`
	OverrideKeys []string `json:"override_keys,omitempty"` // Keys of overridden inputs (not values)
	CostUSD      float64  `json:"cost_usd,omitempty"`
	CostSavedUSD float64  `json:"cost_saved_usd,omitempty"`
}

// Logger writes audit log entries to an append-only log
type Logger struct {
	writer io.Writer
	mu     sync.Mutex
}

// NewLogger creates a new audit logger
func NewLogger(writer io.Writer) *Logger {
	return &Logger{
		writer: writer,
	}
}

// NewFileLogger creates an audit logger that writes to a file
func NewFileLogger(path string) (*Logger, error) {
	// Open file in append mode, create if not exists
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &Logger{
		writer: f,
	}, nil
}

// NewStdoutLogger creates an audit logger that writes to stdout
func NewStdoutLogger() *Logger {
	return &Logger{
		writer: os.Stdout,
	}
}

// NewLoggerFromDestination creates an audit logger based on destination type.
// Valid destinations: "file", "stdout", "syslog"
func NewLoggerFromDestination(destination, filePath string) (*Logger, error) {
	switch destination {
	case "file":
		if filePath == "" {
			return nil, fmt.Errorf("file_path is required when destination is 'file'")
		}
		return NewFileLogger(filePath)
	case "stdout":
		return NewStdoutLogger(), nil
	case "syslog":
		// For syslog, we write to stdout in a syslog-compatible format
		// In production, this would typically be captured by a syslog daemon
		return NewStdoutLogger(), nil
	default:
		return nil, fmt.Errorf("invalid audit destination: %q (must be file, stdout, or syslog)", destination)
	}
}

// Log writes an audit entry
func (l *Logger) Log(entry Entry) error {
	// Set timestamp if not provided
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	// Write with newline
	l.mu.Lock()
	defer l.mu.Unlock()

	_, err = l.writer.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}

	return nil
}

// LogTraceAccess logs access to trace data
func (l *Logger) LogTraceAccess(userID, resource, ipAddress string, result Result, err error) error {
	entry := Entry{
		UserID:    userID,
		Action:    ActionTracesRead,
		Resource:  resource,
		Result:    result,
		IPAddress: ipAddress,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	return l.Log(entry)
}

// LogEventAccess logs access to event data
func (l *Logger) LogEventAccess(userID, resource, ipAddress string, result Result, err error) error {
	entry := Entry{
		UserID:    userID,
		Action:    ActionEventsRead,
		Resource:  resource,
		Result:    result,
		IPAddress: ipAddress,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	return l.Log(entry)
}

// LogEventStream logs access to event streaming
func (l *Logger) LogEventStream(userID, filters, ipAddress string, result Result) error {
	entry := Entry{
		UserID:    userID,
		Action:    ActionEventsStream,
		Resource:  filters,
		Result:    result,
		IPAddress: ipAddress,
	}

	return l.Log(entry)
}

// LogReplayCreate logs the creation of a replay operation
func (l *Logger) LogReplayCreate(userID, parentRunID, fromStep string, overrideKeys []string, ipAddress string, result Result, err error) error {
	entry := Entry{
		UserID:       userID,
		Action:       ActionReplayCreate,
		Resource:     parentRunID,
		Result:       result,
		IPAddress:    ipAddress,
		ParentRunID:  parentRunID,
		FromStep:     fromStep,
		OverrideKeys: overrideKeys,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	return l.Log(entry)
}

// LogReplayExecution logs the completion of a replay operation
func (l *Logger) LogReplayExecution(userID, parentRunID string, costUSD, costSavedUSD float64, result Result, err error) error {
	entry := Entry{
		UserID:       userID,
		Action:       ActionReplayExec,
		Resource:     parentRunID,
		Result:       result,
		ParentRunID:  parentRunID,
		CostUSD:      costUSD,
		CostSavedUSD: costSavedUSD,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	return l.Log(entry)
}

// Close closes the underlying writer if it implements io.Closer
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if closer, ok := l.writer.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}
