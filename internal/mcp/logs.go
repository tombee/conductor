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
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogEntry represents a single log entry.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"` // "stdout" or "stderr"
}

// RingBuffer is a fixed-size circular buffer for log entries.
type RingBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	head    int
	tail    int
	size    int
	count   int
}

// NewRingBuffer creates a new ring buffer with the specified capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1000 // Default
	}
	return &RingBuffer{
		entries: make([]LogEntry, capacity),
		size:    capacity,
	}
}

// Add adds a log entry to the buffer.
func (rb *RingBuffer) Add(entry LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries[rb.tail] = entry
	rb.tail = (rb.tail + 1) % rb.size

	if rb.count < rb.size {
		rb.count++
	} else {
		// Buffer is full, move head forward
		rb.head = (rb.head + 1) % rb.size
	}
}

// GetAll returns all entries in the buffer, oldest first.
func (rb *RingBuffer) GetAll() []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]LogEntry, rb.count)
	for i := 0; i < rb.count; i++ {
		result[i] = rb.entries[(rb.head+i)%rb.size]
	}
	return result
}

// GetLast returns the last n entries, oldest first.
func (rb *RingBuffer) GetLast(n int) []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n > rb.count {
		n = rb.count
	}

	result := make([]LogEntry, n)
	start := rb.count - n
	for i := 0; i < n; i++ {
		result[i] = rb.entries[(rb.head+start+i)%rb.size]
	}
	return result
}

// GetSince returns entries since the given time, oldest first.
func (rb *RingBuffer) GetSince(since time.Time) []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var result []LogEntry
	for i := 0; i < rb.count; i++ {
		entry := rb.entries[(rb.head+i)%rb.size]
		if entry.Timestamp.After(since) || entry.Timestamp.Equal(since) {
			result = append(result, entry)
		}
	}
	return result
}

// Count returns the number of entries in the buffer.
func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Clear removes all entries from the buffer.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.tail = 0
	rb.count = 0
}

// LogCapture captures stdout/stderr from an MCP server process.
type LogCapture struct {
	mu      sync.RWMutex
	buffers map[string]*RingBuffer // server name -> buffer
	maxSize int
}

// NewLogCapture creates a new log capture with default settings.
func NewLogCapture() *LogCapture {
	return &LogCapture{
		buffers: make(map[string]*RingBuffer),
		maxSize: 1000, // 1000 lines per server
	}
}

// GetBuffer returns the log buffer for a server, creating it if needed.
func (lc *LogCapture) GetBuffer(serverName string) *RingBuffer {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if buf, exists := lc.buffers[serverName]; exists {
		return buf
	}

	buf := NewRingBuffer(lc.maxSize)
	lc.buffers[serverName] = buf
	return buf
}

// AddLog adds a log entry for a server.
func (lc *LogCapture) AddLog(serverName string, level LogLevel, message string, source string) {
	buf := lc.GetBuffer(serverName)
	buf.Add(LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    source,
	})
}

// GetLogs returns log entries for a server.
func (lc *LogCapture) GetLogs(serverName string, lines int, since time.Time) []LogEntry {
	lc.mu.RLock()
	buf, exists := lc.buffers[serverName]
	lc.mu.RUnlock()

	if !exists {
		return nil
	}

	if !since.IsZero() {
		return buf.GetSince(since)
	}

	if lines > 0 {
		return buf.GetLast(lines)
	}

	return buf.GetAll()
}

// ClearLogs clears logs for a server.
func (lc *LogCapture) ClearLogs(serverName string) {
	lc.mu.RLock()
	buf, exists := lc.buffers[serverName]
	lc.mu.RUnlock()

	if exists {
		buf.Clear()
	}
}

// RemoveServer removes the log buffer for a server.
func (lc *LogCapture) RemoveServer(serverName string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	delete(lc.buffers, serverName)
}
