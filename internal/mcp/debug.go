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
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// DebugFormatter formats JSON-RPC messages for debugging.
type DebugFormatter struct {
	// writer is where formatted output is written
	writer io.Writer

	// serverName is the name of the MCP server
	serverName string

	// showTimestamps indicates whether to include timestamps
	showTimestamps bool
}

// DebugFormatterConfig configures the debug formatter.
type DebugFormatterConfig struct {
	// Writer is where formatted output is written (required)
	Writer io.Writer

	// ServerName is the name of the MCP server (optional)
	ServerName string

	// ShowTimestamps indicates whether to include timestamps (defaults to true)
	ShowTimestamps bool
}

// NewDebugFormatter creates a new debug formatter.
func NewDebugFormatter(cfg DebugFormatterConfig) *DebugFormatter {
	if cfg.Writer == nil {
		cfg.Writer = io.Discard
	}

	showTimestamps := cfg.ShowTimestamps
	if !cfg.ShowTimestamps {
		// Default to true if not explicitly set to false
		showTimestamps = true
	}

	return &DebugFormatter{
		writer:         cfg.Writer,
		serverName:     cfg.ServerName,
		showTimestamps: showTimestamps,
	}
}

// FormatRequest formats a JSON-RPC request for debugging.
func (f *DebugFormatter) FormatRequest(method string, params interface{}) error {
	return f.formatMessage("REQUEST", method, params)
}

// FormatResponse formats a JSON-RPC response for debugging.
func (f *DebugFormatter) FormatResponse(method string, result interface{}) error {
	return f.formatMessage("RESPONSE", method, result)
}

// FormatError formats a JSON-RPC error for debugging.
func (f *DebugFormatter) FormatError(method string, err error) error {
	return f.formatMessage("ERROR", method, err.Error())
}

// formatMessage formats a JSON-RPC message for debugging.
func (f *DebugFormatter) formatMessage(msgType, method string, data interface{}) error {
	var builder strings.Builder

	// Write header
	if f.showTimestamps {
		builder.WriteString(time.Now().Format("15:04:05.000"))
		builder.WriteString(" ")
	}

	if f.serverName != "" {
		builder.WriteString("[")
		builder.WriteString(f.serverName)
		builder.WriteString("] ")
	}

	builder.WriteString(msgType)
	builder.WriteString(" ")
	builder.WriteString(method)
	builder.WriteString("\n")

	// Format data as indented JSON
	if data != nil {
		jsonData, err := json.MarshalIndent(data, "  ", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
		builder.WriteString("  ")
		builder.Write(jsonData)
		builder.WriteString("\n")
	}

	// Write to output
	_, err := f.writer.Write([]byte(builder.String()))
	return err
}

// LogRawMessage logs a raw JSON-RPC message.
func (f *DebugFormatter) LogRawMessage(direction, raw string) error {
	var builder strings.Builder

	if f.showTimestamps {
		builder.WriteString(time.Now().Format("15:04:05.000"))
		builder.WriteString(" ")
	}

	if f.serverName != "" {
		builder.WriteString("[")
		builder.WriteString(f.serverName)
		builder.WriteString("] ")
	}

	builder.WriteString("RAW ")
	builder.WriteString(direction)
	builder.WriteString("\n")
	builder.WriteString("  ")
	builder.WriteString(raw)
	builder.WriteString("\n")

	_, err := f.writer.Write([]byte(builder.String()))
	return err
}

// ParseAndFormat parses a JSON-RPC message and formats it for debugging.
func (f *DebugFormatter) ParseAndFormat(raw string, direction string) error {
	// Try to parse as JSON
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		// Not valid JSON, log as raw
		return f.LogRawMessage(direction, raw)
	}

	// Extract method and determine message type
	method, _ := msg["method"].(string)

	// Check if it's a request, response, or error
	if method != "" {
		// It's a request
		params := msg["params"]
		if direction == "SEND" {
			return f.FormatRequest(method, params)
		}
		// Incoming request notification
		return f.formatMessage("NOTIFICATION", method, params)
	}

	// Check for error response
	if errData, hasError := msg["error"]; hasError {
		errMsg := "unknown error"
		if errMap, ok := errData.(map[string]interface{}); ok {
			if message, ok := errMap["message"].(string); ok {
				errMsg = message
			}
		}
		return f.FormatError("response", fmt.Errorf("%s", errMsg))
	}

	// It's a successful response
	result := msg["result"]
	return f.FormatResponse("response", result)
}
