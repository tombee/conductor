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
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != "info" {
		t.Errorf("expected default level 'info', got %q", cfg.Level)
	}

	if cfg.Format != FormatJSON {
		t.Errorf("expected default format 'json', got %q", cfg.Format)
	}

	if cfg.Output != os.Stderr {
		t.Errorf("expected default output to be os.Stderr")
	}

	if cfg.AddSource {
		t.Errorf("expected default AddSource to be false")
	}
}

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name:    "defaults when no env vars",
			envVars: map[string]string{},
			expected: &Config{
				Level:     "info",
				Format:    FormatJSON,
				Output:    os.Stderr,
				AddSource: false,
			},
		},
		{
			name: "LOG_LEVEL=debug",
			envVars: map[string]string{
				"LOG_LEVEL": "debug",
			},
			expected: &Config{
				Level:     "debug",
				Format:    FormatJSON,
				Output:    os.Stderr,
				AddSource: false,
			},
		},
		{
			name: "LOG_LEVEL=DEBUG (case insensitive)",
			envVars: map[string]string{
				"LOG_LEVEL": "DEBUG",
			},
			expected: &Config{
				Level:     "debug",
				Format:    FormatJSON,
				Output:    os.Stderr,
				AddSource: false,
			},
		},
		{
			name: "LOG_FORMAT=text",
			envVars: map[string]string{
				"LOG_FORMAT": "text",
			},
			expected: &Config{
				Level:     "info",
				Format:    FormatText,
				Output:    os.Stderr,
				AddSource: false,
			},
		},
		{
			name: "LOG_SOURCE=1",
			envVars: map[string]string{
				"LOG_SOURCE": "1",
			},
			expected: &Config{
				Level:     "info",
				Format:    FormatJSON,
				Output:    os.Stderr,
				AddSource: true,
			},
		},
		{
			name: "all env vars",
			envVars: map[string]string{
				"LOG_LEVEL":  "error",
				"LOG_FORMAT": "text",
				"LOG_SOURCE": "1",
			},
			expected: &Config{
				Level:     "error",
				Format:    FormatText,
				Output:    os.Stderr,
				AddSource: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				// Clean up
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			}()

			cfg := FromEnv()

			if cfg.Level != tt.expected.Level {
				t.Errorf("expected level %q, got %q", tt.expected.Level, cfg.Level)
			}
			if cfg.Format != tt.expected.Format {
				t.Errorf("expected format %q, got %q", tt.expected.Format, cfg.Format)
			}
			if cfg.AddSource != tt.expected.AddSource {
				t.Errorf("expected AddSource %v, got %v", tt.expected.AddSource, cfg.AddSource)
			}
		})
	}
}

func TestNew_JSONFormat(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:     "debug",
		Format:    FormatJSON,
		Output:    &buf,
		AddSource: false,
	}

	logger := New(cfg)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got: %s", output)
	}

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Errorf("expected valid JSON output, got error: %v", err)
	}

	// Check for expected fields
	if logEntry["msg"] != "test message" {
		t.Errorf("expected msg field to be 'test message', got: %v", logEntry["msg"])
	}

	if logEntry["key"] != "value" {
		t.Errorf("expected key field to be 'value', got: %v", logEntry["key"])
	}

	if logEntry["level"] != "INFO" {
		t.Errorf("expected level field to be 'INFO', got: %v", logEntry["level"])
	}
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:     "info",
		Format:    FormatText,
		Output:    &buf,
		AddSource: false,
	}

	logger := New(cfg)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got: %s", output)
	}

	if !strings.Contains(output, "key=value") {
		t.Errorf("expected output to contain 'key=value', got: %s", output)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"invalid", slog.LevelInfo}, // defaults to info
		{"", slog.LevelInfo},         // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLevel(tt.input)
			if level != tt.expected {
				t.Errorf("expected level %v, got %v", tt.expected, level)
			}
		})
	}
}

func TestLogLevel_Filtering(t *testing.T) {
	tests := []struct {
		name          string
		configLevel   string
		logFunc       func(*slog.Logger)
		shouldContain bool
	}{
		{
			name:        "debug log at debug level",
			configLevel: "debug",
			logFunc: func(l *slog.Logger) {
				l.Debug("debug message")
			},
			shouldContain: true,
		},
		{
			name:        "debug log at info level",
			configLevel: "info",
			logFunc: func(l *slog.Logger) {
				l.Debug("debug message")
			},
			shouldContain: false,
		},
		{
			name:        "info log at info level",
			configLevel: "info",
			logFunc: func(l *slog.Logger) {
				l.Info("info message")
			},
			shouldContain: true,
		},
		{
			name:        "info log at warn level",
			configLevel: "warn",
			logFunc: func(l *slog.Logger) {
				l.Info("info message")
			},
			shouldContain: false,
		},
		{
			name:        "error log at error level",
			configLevel: "error",
			logFunc: func(l *slog.Logger) {
				l.Error("error message")
			},
			shouldContain: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			cfg := &Config{
				Level:  tt.configLevel,
				Format: FormatJSON,
				Output: &buf,
			}

			logger := New(cfg)
			tt.logFunc(logger)

			output := buf.String()
			contains := len(output) > 0

			if contains != tt.shouldContain {
				t.Errorf("expected log output=%v, got output=%v (output: %s)", tt.shouldContain, contains, output)
			}
		})
	}
}

func TestWithCorrelationID(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	loggerWithID := WithCorrelationID(logger, "test-correlation-id")
	loggerWithID.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test-correlation-id") {
		t.Errorf("expected output to contain correlation ID, got: %s", output)
	}

	// Verify it's in the JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if logEntry["correlation_id"] != "test-correlation-id" {
		t.Errorf("expected correlation_id field to be 'test-correlation-id', got: %v", logEntry["correlation_id"])
	}
}

func TestWithRequestID(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	loggerWithID := WithRequestID(logger, "test-request-id")
	loggerWithID.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test-request-id") {
		t.Errorf("expected output to contain request ID, got: %s", output)
	}

	// Verify it's in the JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if logEntry["request_id"] != "test-request-id" {
		t.Errorf("expected request_id field to be 'test-request-id', got: %v", logEntry["request_id"])
	}
}

func TestWithComponent(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	loggerWithComponent := WithComponent(logger, "test-component")
	loggerWithComponent.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test-component") {
		t.Errorf("expected output to contain component, got: %s", output)
	}

	// Verify it's in the JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if logEntry["component"] != "test-component" {
		t.Errorf("expected component field to be 'test-component', got: %v", logEntry["component"])
	}
}

func TestWithMultipleContextFields(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	enrichedLogger := WithCorrelationID(
		WithRequestID(
			WithComponent(logger, "test-component"),
			"test-request-id",
		),
		"test-correlation-id",
	)

	enrichedLogger.Info("test message")

	output := buf.String()

	// Verify it's in the JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if logEntry["component"] != "test-component" {
		t.Errorf("expected component field to be 'test-component', got: %v", logEntry["component"])
	}

	if logEntry["request_id"] != "test-request-id" {
		t.Errorf("expected request_id field to be 'test-request-id', got: %v", logEntry["request_id"])
	}

	if logEntry["correlation_id"] != "test-correlation-id" {
		t.Errorf("expected correlation_id field to be 'test-correlation-id', got: %v", logEntry["correlation_id"])
	}
}

func TestAddSource(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:     "info",
		Format:    FormatJSON,
		Output:    &buf,
		AddSource: true,
	}

	logger := New(cfg)
	logger.Info("test message")

	output := buf.String()

	// Verify it's in the JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	// Source should be present
	source, ok := logEntry["source"]
	if !ok {
		t.Errorf("expected source field to be present")
	}

	// Source should be a map with file and line
	sourceMap, ok := source.(map[string]interface{})
	if !ok {
		t.Errorf("expected source to be a map, got: %T", source)
	}

	if _, ok := sourceMap["file"]; !ok {
		t.Errorf("expected source.file to be present")
	}

	if _, ok := sourceMap["line"]; !ok {
		t.Errorf("expected source.line to be present")
	}
}

func TestAttrHelpers(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	logger.Info("test message",
		String("string_key", "string_value"),
		Int("int_key", 42),
		Int64("int64_key", int64(123)),
		Bool("bool_key", true),
		Duration("duration_key", 1500), // should become duration_key_ms
	)

	output := buf.String()

	// Verify it's in the JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if logEntry["string_key"] != "string_value" {
		t.Errorf("expected string_key to be 'string_value', got: %v", logEntry["string_key"])
	}

	if logEntry["int_key"] != float64(42) { // JSON numbers are float64
		t.Errorf("expected int_key to be 42, got: %v", logEntry["int_key"])
	}

	if logEntry["int64_key"] != float64(123) {
		t.Errorf("expected int64_key to be 123, got: %v", logEntry["int64_key"])
	}

	if logEntry["bool_key"] != true {
		t.Errorf("expected bool_key to be true, got: %v", logEntry["bool_key"])
	}

	if logEntry["duration_key_ms"] != float64(1500) {
		t.Errorf("expected duration_key_ms to be 1500, got: %v", logEntry["duration_key_ms"])
	}
}

func TestErrorAttr(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "error",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	testErr := errors.New("test error")
	logger.Error("test error message", Error(testErr))

	output := buf.String()

	// Verify it's in the JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if !strings.Contains(output, testErr.Error()) {
		t.Errorf("expected error message in output, got: %s", output)
	}
}

func TestNilConfig(t *testing.T) {
	// Should not panic when nil config is passed
	logger := New(nil)
	if logger == nil {
		t.Errorf("expected non-nil logger when nil config passed")
	}
}

func BenchmarkLogger_JSON(b *testing.B) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message",
			"iteration", i,
			"key1", "value1",
			"key2", "value2")
	}
}

func BenchmarkLogger_Text(b *testing.B) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatText,
		Output: &buf,
	}

	logger := New(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message",
			"iteration", i,
			"key1", "value1",
			"key2", "value2")
	}
}

// Test CONDUCTOR_LOG_LEVEL environment variable support
func TestFromEnv_ConductorLogLevel(t *testing.T) {
	tests := []struct {
		name              string
		conductorLogLevel string
		logLevel          string
		expectedLevel     string
	}{
		{
			name:              "CONDUCTOR_LOG_LEVEL takes precedence",
			conductorLogLevel: "debug",
			logLevel:          "error",
			expectedLevel:     "debug",
		},
		{
			name:              "LOG_LEVEL used when CONDUCTOR_LOG_LEVEL not set",
			conductorLogLevel: "",
			logLevel:          "warn",
			expectedLevel:     "warn",
		},
		{
			name:              "CONDUCTOR_LOG_LEVEL alone",
			conductorLogLevel: "error",
			logLevel:          "",
			expectedLevel:     "error",
		},
		{
			name:              "both unset defaults to info",
			conductorLogLevel: "",
			logLevel:          "",
			expectedLevel:     "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean slate
			os.Unsetenv("CONDUCTOR_LOG_LEVEL")
			os.Unsetenv("LOG_LEVEL")

			// Set environment variables
			if tt.conductorLogLevel != "" {
				os.Setenv("CONDUCTOR_LOG_LEVEL", tt.conductorLogLevel)
			}
			if tt.logLevel != "" {
				os.Setenv("LOG_LEVEL", tt.logLevel)
			}

			defer func() {
				os.Unsetenv("CONDUCTOR_LOG_LEVEL")
				os.Unsetenv("LOG_LEVEL")
			}()

			cfg := FromEnv()

			if cfg.Level != tt.expectedLevel {
				t.Errorf("expected level %q, got %q", tt.expectedLevel, cfg.Level)
			}
		})
	}
}

// Test WithRunContext helper
func TestWithRunContext(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	loggerWithRunContext := WithRunContext(logger, "run-123", "test-workflow")
	loggerWithRunContext.Info("test message")

	output := buf.String()

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if logEntry[RunIDKey] != "run-123" {
		t.Errorf("expected %s to be 'run-123', got: %v", RunIDKey, logEntry[RunIDKey])
	}

	if logEntry[WorkflowKey] != "test-workflow" {
		t.Errorf("expected %s to be 'test-workflow', got: %v", WorkflowKey, logEntry[WorkflowKey])
	}
}

// Test WithStepContext helper
func TestWithStepContext(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	loggerWithStepContext := WithStepContext(logger, "run-456", "step-789")
	loggerWithStepContext.Info("test message")

	output := buf.String()

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if logEntry[RunIDKey] != "run-456" {
		t.Errorf("expected %s to be 'run-456', got: %v", RunIDKey, logEntry[RunIDKey])
	}

	if logEntry[StepIDKey] != "step-789" {
		t.Errorf("expected %s to be 'step-789', got: %v", StepIDKey, logEntry[StepIDKey])
	}
}

// Test WithProvider helper
func TestWithProvider(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	loggerWithProvider := WithProvider(logger, "openai")
	loggerWithProvider.Info("test message")

	output := buf.String()

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if logEntry[ProviderKey] != "openai" {
		t.Errorf("expected %s to be 'openai', got: %v", ProviderKey, logEntry[ProviderKey])
	}
}

// Test SanitizeAPIKey
func TestSanitizeAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal API key",
			input:    "sk-1234567890abcdef",
			expected: "...cdef",
		},
		{
			name:     "short key redacted",
			input:    "abc",
			expected: "[REDACTED]",
		},
		{
			name:     "exactly 4 chars redacted",
			input:    "abcd",
			expected: "[REDACTED]",
		},
		{
			name:     "empty string redacted",
			input:    "",
			expected: "[REDACTED]",
		},
		{
			name:     "5 chars shows last 4",
			input:    "abcde",
			expected: "...bcde",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Test SanitizeSecret
func TestSanitizeSecret(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "normal secret",
			input: "super-secret-password",
		},
		{
			name:  "empty secret",
			input: "",
		},
		{
			name:  "long secret",
			input: "this-is-a-very-long-secret-that-should-never-appear-in-logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeSecret(tt.input)
			if result != "[REDACTED]" {
				t.Errorf("expected '[REDACTED]', got %q", result)
			}
			// Ensure original secret is not in the result
			if strings.Contains(result, tt.input) && tt.input != "" {
				t.Errorf("sanitized output should not contain original secret")
			}
		})
	}
}

// Test combining multiple context helpers
func TestCombinedContextHelpers(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	enrichedLogger := WithProvider(
		WithRunContext(logger, "run-999", "combined-workflow"),
		"anthropic",
	)
	enrichedLogger.Info("test message")

	output := buf.String()

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if logEntry[RunIDKey] != "run-999" {
		t.Errorf("expected %s to be 'run-999', got: %v", RunIDKey, logEntry[RunIDKey])
	}

	if logEntry[WorkflowKey] != "combined-workflow" {
		t.Errorf("expected %s to be 'combined-workflow', got: %v", WorkflowKey, logEntry[WorkflowKey])
	}

	if logEntry[ProviderKey] != "anthropic" {
		t.Errorf("expected %s to be 'anthropic', got: %v", ProviderKey, logEntry[ProviderKey])
	}
}
