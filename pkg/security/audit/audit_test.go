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

package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockDestination is a mock destination for testing.
type mockDestination struct {
	mu     sync.Mutex
	events []Event
	closed bool
}

func (m *mockDestination) Write(event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockDestination) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockDestination) getEvents() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	events := make([]Event, len(m.events))
	copy(events, m.events)
	return events
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "empty config with defaults",
			config: Config{
				Destinations: []DestinationConfig{},
			},
			wantErr: false,
		},
		{
			name: "custom buffer size",
			config: Config{
				BufferSize:   500,
				Destinations: []DestinationConfig{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if logger != nil {
				defer logger.Close()

				expectedBufferSize := tt.config.BufferSize
				if expectedBufferSize == 0 {
					expectedBufferSize = DefaultBufferSize
				}
				if logger.bufferSize != expectedBufferSize {
					t.Errorf("Logger.bufferSize = %d, want %d", logger.bufferSize, expectedBufferSize)
				}
			}
		})
	}
}

func TestLogger_EventBuffering(t *testing.T) {
	logger, err := NewLogger(Config{
		BufferSize:   10,
		Destinations: []DestinationConfig{},
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Add a mock destination
	mock := &mockDestination{}
	logger.mu.Lock()
	logger.destinations = append(logger.destinations, mock)
	logger.mu.Unlock()

	// Log some events
	testEvent := Event{
		Timestamp: time.Now(),
		EventType: "test_event",
		Decision:  "allowed",
		Reason:    "test",
	}

	logger.Log(testEvent)

	// Give time for the background writer to process
	time.Sleep(50 * time.Millisecond)

	events := mock.getEvents()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	if len(events) > 0 {
		if events[0].EventType != "test_event" {
			t.Errorf("expected EventType 'test_event', got '%s'", events[0].EventType)
		}
	}
}

func TestLogger_BufferOverflow(t *testing.T) {
	// Create a logger with a very small buffer
	logger, err := NewLogger(Config{
		BufferSize:   2,
		Destinations: []DestinationConfig{},
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Block the background writer by not adding any destinations
	// and filling the buffer

	// Fill buffer beyond capacity
	for i := 0; i < 5; i++ {
		logger.Log(Event{
			Timestamp: time.Now(),
			EventType: "overflow_test",
			Decision:  "allowed",
		})
	}

	// Buffer should be full
	utilization := logger.BufferUtilization()
	if utilization < 0.5 {
		t.Errorf("expected buffer to be at least 50%% full, got %.2f%%", utilization*100)
	}
}

func TestLogger_BufferUtilization(t *testing.T) {
	logger, err := NewLogger(Config{
		BufferSize:   10,
		Destinations: []DestinationConfig{},
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Initially should be empty
	if util := logger.BufferUtilization(); util != 0.0 {
		t.Errorf("initial utilization should be 0.0, got %.2f", util)
	}

	// Add some events
	for i := 0; i < 5; i++ {
		logger.Log(Event{
			Timestamp: time.Now(),
			EventType: "util_test",
			Decision:  "allowed",
		})
	}

	// Should have some utilization
	util := logger.BufferUtilization()
	if util <= 0 || util > 1.0 {
		t.Errorf("utilization should be between 0 and 1, got %.2f", util)
	}
}

func TestLogger_Close(t *testing.T) {
	logger, err := NewLogger(Config{
		BufferSize:   10,
		Destinations: []DestinationConfig{},
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	mock := &mockDestination{}
	logger.mu.Lock()
	logger.destinations = append(logger.destinations, mock)
	logger.mu.Unlock()

	// Log an event
	logger.Log(Event{
		Timestamp: time.Now(),
		EventType: "close_test",
		Decision:  "allowed",
	})

	// Close logger
	if err := logger.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify destination was closed
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if !mock.closed {
		t.Error("destination was not closed")
	}
}

func TestFileDestination_Write(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	dest, err := NewFileDestination(DestinationConfig{
		Type:   "file",
		Path:   logFile,
		Format: "json",
	})
	if err != nil {
		t.Fatalf("NewFileDestination() error = %v", err)
	}
	defer dest.Close()

	event := Event{
		Timestamp: time.Now(),
		EventType: "test_event",
		Decision:  "allowed",
		Reason:    "testing file destination",
	}

	if err := dest.Write(event); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Read back the file
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Should contain JSON
	if !strings.Contains(string(data), "test_event") {
		t.Errorf("log file does not contain expected event")
	}

	// Verify it's valid JSON
	var readEvent Event
	if err := json.Unmarshal(data[:len(data)-1], &readEvent); err != nil {
		t.Errorf("log file does not contain valid JSON: %v", err)
	}
}

func TestFileDestination_TextFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.txt")

	dest, err := NewFileDestination(DestinationConfig{
		Type:   "file",
		Path:   logFile,
		Format: "text",
	})
	if err != nil {
		t.Fatalf("NewFileDestination() error = %v", err)
	}
	defer dest.Close()

	event := Event{
		Timestamp:    time.Now(),
		EventType:    "access_check",
		ResourceType: "file",
		Resource:     "/tmp/test.txt",
		Decision:     "denied",
		Reason:       "not in allowlist",
	}

	if err := dest.Write(event); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Read back the file
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Should contain human-readable text
	logLine := string(data)
	if !strings.Contains(logLine, "access_check") {
		t.Errorf("log line missing event type")
	}
	if !strings.Contains(logLine, "denied") {
		t.Errorf("log line missing decision")
	}
}

func TestWebhookDestination_Write(t *testing.T) {
	// Create a test server
	received := make(chan Event, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("failed to decode event: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- event
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dest, err := NewWebhookDestination(DestinationConfig{
		Type: "webhook",
		URL:  server.URL,
		Headers: map[string]string{
			"X-Test-Header": "test-value",
		},
	})
	if err != nil {
		t.Fatalf("NewWebhookDestination() error = %v", err)
	}
	defer dest.Close()

	event := Event{
		Timestamp: time.Now(),
		EventType: "webhook_test",
		Decision:  "allowed",
	}

	if err := dest.Write(event); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Wait for event to be received
	select {
	case receivedEvent := <-received:
		if receivedEvent.EventType != "webhook_test" {
			t.Errorf("received event type = %s, want %s", receivedEvent.EventType, "webhook_test")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for webhook event")
	}
}

func TestCreateDestination_UnknownType(t *testing.T) {
	_, err := createDestination(DestinationConfig{
		Type: "unknown",
	})
	if err == nil {
		t.Error("expected error for unknown destination type")
	}
	if !strings.Contains(err.Error(), "unknown destination type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestRotatingFileDestination_Integration tests end-to-end rotation behavior.
func TestRotatingFileDestination_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "audit.log")

	// Create logger with rotating destination
	logger, err := NewLogger(Config{
		Destinations: []DestinationConfig{
			{
				Type:        "rotating-file",
				Path:        basePath,
				Format:      "json",
				MaxSize:     1024, // 1KB for testing
				MaxAge:      24 * time.Hour,
				RotateDaily: false,
				Compress:    false,
			},
		},
		BufferSize: 100,
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Write events that will trigger rotation
	largeReason := strings.Repeat("x", 500) // Large event to trigger rotation quickly
	for i := 0; i < 5; i++ {
		logger.Log(Event{
			Timestamp: time.Now(),
			EventType: "rotation_test",
			Decision:  "allowed",
			Reason:    largeReason,
		})
	}

	// Give time for events to be written
	time.Sleep(200 * time.Millisecond)

	// Check that files exist
	files, err := filepath.Glob(filepath.Join(tmpDir, "audit*"))
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) == 0 {
		t.Error("expected at least one audit file")
	}

	// Verify we can read the current file
	data, err := os.ReadFile(basePath)
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("failed to read audit log: %v", err)
	}

	// Verify JSON format
	if len(data) > 0 {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			var event Event
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				t.Errorf("invalid JSON in log file: %v", err)
			}
		}
	}
}

// TestRotatingFileDestination_LoggerIntegration tests that rotating destination works via NewLogger.
func TestRotatingFileDestination_LoggerIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "audit.log")

	// Create logger with rotating destination through config
	logger, err := NewLogger(Config{
		Destinations: []DestinationConfig{
			{
				Type:        "rotating-file",
				Path:        basePath,
				Format:      "json",
				MaxSize:     DefaultMaxSize,
				MaxAge:      DefaultMaxAge,
				RotateDaily: true,
				Compress:    true,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewLogger() with rotating-file error = %v", err)
	}
	defer logger.Close()

	// Log some events
	for i := 0; i < 10; i++ {
		logger.Log(Event{
			Timestamp: time.Now(),
			EventType: "integration_test",
			Decision:  "allowed",
			Profile:   "standard",
		})
	}

	// Give time for background writer
	time.Sleep(100 * time.Millisecond)

	// Verify file was created
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		t.Error("audit log file was not created")
	}

	// Verify buffer utilization
	util := logger.BufferUtilization()
	if util < 0 || util > 1 {
		t.Errorf("buffer utilization out of range: %.2f", util)
	}
}

// TestRotatingFileDestination_ListRotatedLogs tests listing rotated log files.
func TestRotatingFileDestination_ListRotatedLogs(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "audit.log")

	// Create a rotating destination and write some data
	dest, err := NewRotatingFileDestination(RotationConfig{
		Path:        basePath,
		Format:      "json",
		MaxSize:     100, // Very small to force rotation
		MaxAge:      24 * time.Hour,
		RotateDaily: false,
		Compress:    false,
	})
	if err != nil {
		t.Fatalf("NewRotatingFileDestination() error = %v", err)
	}

	// Write enough data to trigger rotation
	largeEvent := Event{
		Timestamp: time.Now(),
		EventType: "list_test",
		Decision:  "allowed",
		Reason:    strings.Repeat("x", 200),
	}

	for i := 0; i < 3; i++ {
		if err := dest.Write(largeEvent); err != nil {
			t.Errorf("Write() error = %v", err)
		}
	}

	dest.Close()

	// List rotated logs
	logs, err := ListRotatedLogs(basePath)
	if err != nil {
		t.Fatalf("ListRotatedLogs() error = %v", err)
	}

	// Should have some rotated files (depending on rotation trigger)
	t.Logf("Found %d rotated log files", len(logs))

	// Verify log info structure
	for _, log := range logs {
		if log.Path == "" {
			t.Error("log path is empty")
		}
		if log.Size < 0 {
			t.Error("log size is negative")
		}
		if log.ModTime.IsZero() {
			t.Error("log mod time is zero")
		}
	}
}
