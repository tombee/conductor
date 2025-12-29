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

package runner

import (
	"sync"
	"time"
)

// LogAggregator handles log collection and subscription routing.
type LogAggregator struct {
	mu          sync.RWMutex
	subscribers map[string][]chan LogEntry
}

// NewLogAggregator creates a new LogAggregator.
func NewLogAggregator() *LogAggregator {
	return &LogAggregator{
		subscribers: make(map[string][]chan LogEntry),
	}
}

// AddLog adds a log entry to a run and notifies subscribers.
// The run's Logs slice is appended with the new entry.
// Thread-safe: acquires run.mu for log slice modification.
func (l *LogAggregator) AddLog(run *Run, level, message, stepID string) {
	entry := LogEntry{
		Timestamp:     time.Now(),
		Level:         level,
		Message:       message,
		StepID:        stepID,
		CorrelationID: run.CorrelationID,
	}

	// Append to run's logs under lock
	run.mu.Lock()
	run.Logs = append(run.Logs, entry)
	run.mu.Unlock()

	// Notify subscribers (outside lock to avoid blocking)
	l.notifySubscribers(run.ID, entry)
}

// AddLogEntry adds a pre-constructed log entry to a run and notifies subscribers.
// Thread-safe: acquires run.mu for log slice modification.
func (l *LogAggregator) AddLogEntry(run *Run, entry LogEntry) {
	run.mu.Lock()
	run.Logs = append(run.Logs, entry)
	run.mu.Unlock()

	l.notifySubscribers(run.ID, entry)
}

// notifySubscribers sends a log entry to all subscribers for a run.
// Makes a copy of the subscriber slice to avoid race with unsubscribe.
func (l *LogAggregator) notifySubscribers(runID string, entry LogEntry) {
	l.mu.RLock()
	origSubs := l.subscribers[runID]
	// Make a copy to avoid race with unsubscribe modifying the slice
	subs := make([]chan LogEntry, len(origSubs))
	copy(subs, origSubs)
	l.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- entry:
		default:
			// Channel full, skip
		}
	}
}

// Subscribe returns a channel that receives log entries for a run.
// Returns the channel and an unsubscribe function.
func (l *LogAggregator) Subscribe(runID string) (<-chan LogEntry, func()) {
	ch := make(chan LogEntry, 100)

	l.mu.Lock()
	l.subscribers[runID] = append(l.subscribers[runID], ch)
	l.mu.Unlock()

	// Unsubscribe function removes the channel from the subscriber map.
	// Note: We don't close the channel to avoid race conditions with concurrent senders.
	// The channel will be garbage collected when no longer referenced.
	unsub := func() {
		l.mu.Lock()
		defer l.mu.Unlock()

		subs := l.subscribers[runID]
		for i, sub := range subs {
			if sub == ch {
				l.subscribers[runID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}

	return ch, unsub
}

// SubscriberCount returns the number of subscribers for a run.
func (l *LogAggregator) SubscriberCount(runID string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.subscribers[runID])
}
