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

package log

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format represents the log output format.
type Format string

const (
	// FormatJSON outputs logs in JSON format for machine parsing.
	FormatJSON Format = "json"
	// FormatText outputs logs in human-readable text format.
	FormatText Format = "text"
)

// Custom log levels extending slog's standard levels.
const (
	// LevelTrace is more verbose than Debug, used for detailed tracing
	// (e.g., HTTP request/response bodies, LLM prompts/responses).
	LevelTrace = slog.Level(-8)
)

// Standard field keys for structured logging.
// These constants ensure consistent field naming across the codebase.
const (
	// RunIDKey is the field key for workflow run identifiers.
	RunIDKey = "run_id"
	// StepIDKey is the field key for workflow step identifiers.
	StepIDKey = "step_id"
	// ProviderKey is the field key for LLM provider names.
	ProviderKey = "provider"
	// DurationKey is the field key for duration in milliseconds.
	DurationKey = "duration_ms"
	// WorkflowKey is the field key for workflow names.
	WorkflowKey = "workflow"
	// EventKey is the field key for event types.
	EventKey = "event"
)

// Config holds the logging configuration.
type Config struct {
	// Level sets the minimum log level (debug, info, warn, error).
	// Default: info
	Level string

	// Format sets the output format (json, text).
	// Default: json
	Format Format

	// Output is the writer for log output.
	// Default: os.Stderr
	Output io.Writer

	// AddSource adds source file and line information to logs.
	// Default: false
	AddSource bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Level:     "info",
		Format:    FormatJSON,
		Output:    os.Stderr,
		AddSource: false,
	}
}

// FromEnv creates a Config from environment variables.
// Supported environment variables:
//   - CONDUCTOR_DEBUG: true/1 to enable debug level and source logging (takes precedence)
//   - CONDUCTOR_LOG_LEVEL: debug, info, warn, error (takes precedence over LOG_LEVEL)
//   - LOG_LEVEL: debug, info, warn, error (default: info)
//   - LOG_FORMAT: json, text (default: json)
//   - LOG_SOURCE: 1 to enable source file/line (default: 0)
func FromEnv() *Config {
	cfg := DefaultConfig()

	// CONDUCTOR_DEBUG enables debug logging and source information
	debug := os.Getenv("CONDUCTOR_DEBUG")
	if debug == "true" || debug == "1" {
		cfg.Level = "debug"
		cfg.AddSource = true
	}

	// CONDUCTOR_LOG_LEVEL takes precedence over LOG_LEVEL (but not CONDUCTOR_DEBUG)
	if debug == "" {
		if level := os.Getenv("CONDUCTOR_LOG_LEVEL"); level != "" {
			cfg.Level = strings.ToLower(level)
		} else if level := os.Getenv("LOG_LEVEL"); level != "" {
			cfg.Level = strings.ToLower(level)
		}
	}

	if format := os.Getenv("LOG_FORMAT"); format != "" {
		cfg.Format = Format(strings.ToLower(format))
	}

	if os.Getenv("LOG_SOURCE") == "1" {
		cfg.AddSource = true
	}

	return cfg
}

// New creates a new structured logger from the given configuration.
func New(cfg *Config) *slog.Logger {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Parse log level
	level := parseLevel(cfg.Level)

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	// Select handler based on format
	var handler slog.Handler
	switch cfg.Format {
	case FormatText:
		handler = slog.NewTextHandler(cfg.Output, opts)
	case FormatJSON:
		fallthrough
	default:
		handler = slog.NewJSONHandler(cfg.Output, opts)
	}

	return slog.New(handler)
}

// parseLevel converts a string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return LevelTrace
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithCorrelationID returns a new logger with a correlation ID field.
// Correlation IDs are used for cross-process tracing.
func WithCorrelationID(logger *slog.Logger, correlationID string) *slog.Logger {
	return logger.With("correlation_id", correlationID)
}

// WithRequestID returns a new logger with a request ID field.
// Request IDs are used for tracing individual requests.
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With("request_id", requestID)
}

// WithComponent returns a new logger with a component name field.
// Component names help identify which part of the system generated the log.
func WithComponent(logger *slog.Logger, component string) *slog.Logger {
	return logger.With("component", component)
}

// LogAttrs is a convenience type for structured log attributes.
type LogAttrs []slog.Attr

// Attr creates a new attribute with the given key and value.
func Attr(key string, value any) slog.Attr {
	return slog.Any(key, value)
}

// String creates a string attribute.
func String(key, value string) slog.Attr {
	return slog.String(key, value)
}

// Int creates an int attribute.
func Int(key string, value int) slog.Attr {
	return slog.Int(key, value)
}

// Int64 creates an int64 attribute.
func Int64(key string, value int64) slog.Attr {
	return slog.Int64(key, value)
}

// Bool creates a bool attribute.
func Bool(key string, value bool) slog.Attr {
	return slog.Bool(key, value)
}

// Error creates an error attribute.
func Error(err error) slog.Attr {
	return slog.Any("error", err)
}

// Duration creates a duration attribute in milliseconds.
func Duration(key string, value int64) slog.Attr {
	return slog.Int64(key+"_ms", value)
}

// WithRunContext returns a new logger with workflow run context fields.
// This adds run_id and workflow name to all subsequent log entries.
func WithRunContext(logger *slog.Logger, runID, workflowName string) *slog.Logger {
	return logger.With(
		slog.String(RunIDKey, runID),
		slog.String(WorkflowKey, workflowName),
	)
}

// WithStepContext returns a new logger with workflow step context fields.
// This adds run_id and step_id to all subsequent log entries.
func WithStepContext(logger *slog.Logger, runID, stepID string) *slog.Logger {
	return logger.With(
		slog.String(RunIDKey, runID),
		slog.String(StepIDKey, stepID),
	)
}

// WithProvider returns a new logger with provider context.
// This adds provider name to all subsequent log entries.
func WithProvider(logger *slog.Logger, provider string) *slog.Logger {
	return logger.With(slog.String(ProviderKey, provider))
}

// SanitizeAPIKey masks an API key, showing only the last 4 characters.
// This prevents accidental credential leakage in logs.
// Returns "[REDACTED]" if the key is shorter than 4 characters.
func SanitizeAPIKey(key string) string {
	if len(key) <= 4 {
		return "[REDACTED]"
	}
	return "..." + key[len(key)-4:]
}

// SanitizeSecret completely redacts a secret value.
// This should be used for any sensitive data that should never appear in logs.
func SanitizeSecret(secret string) string {
	return "[REDACTED]"
}

// Trace logs a message at trace level with optional attributes.
// This is used for highly verbose debugging output like HTTP bodies and LLM prompts.
func Trace(logger *slog.Logger, msg string, attrs ...slog.Attr) {
	if !logger.Enabled(nil, LevelTrace) {
		return
	}
	logger.LogAttrs(nil, LevelTrace, msg, attrs...)
}
