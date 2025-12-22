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
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAuditLogger_Write(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	entry := Entry{
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		UserID:    "user123",
		Action:    ActionTracesRead,
		Resource:  "/v1/traces/abc123",
		Result:    ResultSuccess,
		IPAddress: "192.168.1.1",
	}

	err := logger.Log(entry)
	if err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	// Verify the output is valid JSON
	var decoded Entry
	err = json.NewDecoder(&buf).Decode(&decoded)
	if err != nil {
		t.Fatalf("failed to decode logged entry: %v", err)
	}

	// Verify fields
	if decoded.UserID != entry.UserID {
		t.Errorf("expected user_id %q, got %q", entry.UserID, decoded.UserID)
	}
	if decoded.Action != entry.Action {
		t.Errorf("expected action %q, got %q", entry.Action, decoded.Action)
	}
	if decoded.Resource != entry.Resource {
		t.Errorf("expected resource %q, got %q", entry.Resource, decoded.Resource)
	}
	if decoded.Result != entry.Result {
		t.Errorf("expected result %q, got %q", entry.Result, decoded.Result)
	}
	if decoded.IPAddress != entry.IPAddress {
		t.Errorf("expected ip_address %q, got %q", entry.IPAddress, decoded.IPAddress)
	}
}

func TestAuditLogger_AppendOnly(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create logger and write first entry
	logger, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	err = logger.Log(Entry{
		UserID:   "user1",
		Action:   ActionTracesRead,
		Resource: "trace1",
		Result:   ResultSuccess,
	})
	if err != nil {
		t.Fatalf("failed to log first entry: %v", err)
	}

	logger.Close()

	// Reopen logger and write second entry
	logger, err = NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to reopen file logger: %v", err)
	}

	err = logger.Log(Entry{
		UserID:   "user2",
		Action:   ActionEventsRead,
		Resource: "event1",
		Result:   ResultSuccess,
	})
	if err != nil {
		t.Fatalf("failed to log second entry: %v", err)
	}

	logger.Close()

	// Read file and verify both entries exist
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// Verify first entry
	var entry1 Entry
	if err := json.Unmarshal([]byte(lines[0]), &entry1); err != nil {
		t.Fatalf("failed to unmarshal first entry: %v", err)
	}
	if entry1.UserID != "user1" {
		t.Errorf("expected first entry user_id %q, got %q", "user1", entry1.UserID)
	}

	// Verify second entry
	var entry2 Entry
	if err := json.Unmarshal([]byte(lines[1]), &entry2); err != nil {
		t.Fatalf("failed to unmarshal second entry: %v", err)
	}
	if entry2.UserID != "user2" {
		t.Errorf("expected second entry user_id %q, got %q", "user2", entry2.UserID)
	}
}

func TestAuditLogger_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	// Number of concurrent writers
	numWriters := 10
	numEntriesPerWriter := 100

	var wg sync.WaitGroup
	wg.Add(numWriters)

	// Launch concurrent writers
	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < numEntriesPerWriter; j++ {
				entry := Entry{
					UserID:   string(rune('A' + writerID)),
					Action:   ActionTracesRead,
					Resource: "concurrent-test",
					Result:   ResultSuccess,
				}
				if err := logger.Log(entry); err != nil {
					t.Errorf("writer %d failed to log entry %d: %v", writerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify we got all entries
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	expectedLines := numWriters * numEntriesPerWriter
	if len(lines) != expectedLines {
		t.Errorf("expected %d lines, got %d", expectedLines, len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestAuditLogger_AutoTimestamp(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	// Log entry without timestamp
	entry := Entry{
		UserID:   "user123",
		Action:   ActionTracesRead,
		Resource: "test",
		Result:   ResultSuccess,
	}

	before := time.Now()
	err := logger.Log(entry)
	after := time.Now()

	if err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	// Verify timestamp was set
	var decoded Entry
	if err := json.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("failed to decode entry: %v", err)
	}

	if decoded.Timestamp.IsZero() {
		t.Error("timestamp was not set automatically")
	}

	if decoded.Timestamp.Before(before) || decoded.Timestamp.After(after) {
		t.Errorf("timestamp %v is outside expected range [%v, %v]",
			decoded.Timestamp, before, after)
	}
}

func TestAuditStorage_Query(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create logger and write test entries
	logger, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	entries := []Entry{
		{
			Timestamp: baseTime,
			UserID:    "alice",
			Action:    ActionTracesRead,
			Resource:  "trace1",
			Result:    ResultSuccess,
			IPAddress: "192.168.1.1",
		},
		{
			Timestamp: baseTime.Add(1 * time.Hour),
			UserID:    "bob",
			Action:    ActionEventsRead,
			Resource:  "event1",
			Result:    ResultSuccess,
			IPAddress: "192.168.1.2",
		},
		{
			Timestamp: baseTime.Add(2 * time.Hour),
			UserID:    "alice",
			Action:    ActionTracesRead,
			Resource:  "trace2",
			Result:    ResultForbidden,
			IPAddress: "192.168.1.1",
		},
		{
			Timestamp: baseTime.Add(3 * time.Hour),
			UserID:    "charlie",
			Action:    ActionEventsStream,
			Resource:  "stream1",
			Result:    ResultError,
			IPAddress: "192.168.1.3",
		},
	}

	for _, entry := range entries {
		if err := logger.Log(entry); err != nil {
			t.Fatalf("failed to log entry: %v", err)
		}
	}
	logger.Close()

	// Create store and run queries
	store := NewStore(logPath)

	tests := []struct {
		name     string
		filter   QueryFilter
		expected int
		checkFn  func(t *testing.T, results []Entry)
	}{
		{
			name: "filter by user",
			filter: QueryFilter{
				UserID: "alice",
			},
			expected: 2,
			checkFn: func(t *testing.T, results []Entry) {
				for _, r := range results {
					if r.UserID != "alice" {
						t.Errorf("expected user_id %q, got %q", "alice", r.UserID)
					}
				}
			},
		},
		{
			name: "filter by action",
			filter: QueryFilter{
				Action: ActionTracesRead,
			},
			expected: 2,
			checkFn: func(t *testing.T, results []Entry) {
				for _, r := range results {
					if r.Action != ActionTracesRead {
						t.Errorf("expected action %q, got %q", ActionTracesRead, r.Action)
					}
				}
			},
		},
		{
			name: "filter by result",
			filter: QueryFilter{
				Result: ResultSuccess,
			},
			expected: 2,
		},
		{
			name: "filter by time range",
			filter: QueryFilter{
				Since: baseTime.Add(1 * time.Hour),
				Until: baseTime.Add(2*time.Hour + 30*time.Minute),
			},
			expected: 2,
		},
		{
			name: "filter with limit",
			filter: QueryFilter{
				Limit: 2,
			},
			expected: 2,
		},
		{
			name: "combined filters",
			filter: QueryFilter{
				UserID: "alice",
				Action: ActionTracesRead,
				Result: ResultForbidden,
			},
			expected: 1,
			checkFn: func(t *testing.T, results []Entry) {
				if len(results) != 1 {
					t.Fatalf("expected 1 result, got %d", len(results))
				}
				if results[0].Resource != "trace2" {
					t.Errorf("expected resource %q, got %q", "trace2", results[0].Resource)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.Query(tt.filter)
			if err != nil {
				t.Fatalf("query failed: %v", err)
			}

			if len(results) != tt.expected {
				t.Errorf("expected %d results, got %d", tt.expected, len(results))
			}

			if tt.checkFn != nil {
				tt.checkFn(t, results)
			}
		})
	}
}

func TestAuditStorage_QueryEmptyLog(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "nonexistent.log")

	store := NewStore(logPath)
	results, err := store.Query(QueryFilter{})

	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent log, got %d", len(results))
	}
}
