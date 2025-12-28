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

package server

import (
	"log/slog"
	"testing"
)

func TestCreateLogger_ValidLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"empty defaults to info", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := createLogger(tt.level)
			if err != nil {
				t.Fatalf("createLogger(%q) returned error: %v", tt.level, err)
			}
			if logger == nil {
				t.Fatal("createLogger returned nil logger")
			}

			// Verify the logger is enabled for the expected level
			if !logger.Enabled(nil, tt.expected) {
				t.Errorf("logger not enabled for level %v", tt.expected)
			}
		})
	}
}

func TestCreateLogger_InvalidLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"invalid string", "invalid"},
		{"uppercase", "INFO"},
		{"numeric", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := createLogger(tt.level)
			if err == nil {
				t.Errorf("createLogger(%q) should return error, got nil", tt.level)
			}
			if logger != nil {
				t.Errorf("createLogger(%q) should return nil logger on error, got %v", tt.level, logger)
			}
		})
	}
}

func TestNewServer_ValidConfig(t *testing.T) {
	config := ServerConfig{
		Name:     "test-server",
		Version:  "1.0.0",
		LogLevel: "debug",
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	if server == nil {
		t.Fatal("NewServer() returned nil server")
	}

	if server.name != "test-server" {
		t.Errorf("server.name = %q, want %q", server.name, "test-server")
	}

	if server.version != "1.0.0" {
		t.Errorf("server.version = %q, want %q", server.version, "1.0.0")
	}

	if server.logger == nil {
		t.Error("server.logger is nil")
	}
}

func TestNewServer_InvalidLogLevel(t *testing.T) {
	config := ServerConfig{
		Name:     "test-server",
		Version:  "1.0.0",
		LogLevel: "invalid",
	}

	server, err := NewServer(config)
	if err == nil {
		t.Error("NewServer() with invalid log level should return error")
	}

	if server != nil {
		t.Errorf("NewServer() with invalid log level should return nil server, got %v", server)
	}
}

func TestNewServer_Defaults(t *testing.T) {
	config := ServerConfig{
		// Empty config - test defaults
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	if server.name != "conductor" {
		t.Errorf("server.name = %q, want %q", server.name, "conductor")
	}

	if server.version != "dev" {
		t.Errorf("server.version = %q, want %q", server.version, "dev")
	}

	if server.logger == nil {
		t.Error("server.logger is nil")
	}
}
