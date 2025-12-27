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
	"testing"
	"time"
)

func TestNewLogAggregator(t *testing.T) {
	la := NewLogAggregator()
	if la == nil {
		t.Fatal("expected non-nil LogAggregator")
	}
	if la.subscribers == nil {
		t.Error("expected non-nil subscribers map")
	}
}

func TestLogAggregator_AddLog(t *testing.T) {
	la := NewLogAggregator()

	run := &Run{
		ID:            "test-run",
		CorrelationID: "corr-123",
	}

	la.AddLog(run, "info", "test message", "step1")

	if len(run.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(run.Logs))
	}

	entry := run.Logs[0]
	if entry.Level != "info" {
		t.Errorf("expected level 'info', got %s", entry.Level)
	}
	if entry.Message != "test message" {
		t.Errorf("expected message 'test message', got %s", entry.Message)
	}
	if entry.StepID != "step1" {
		t.Errorf("expected stepID 'step1', got %s", entry.StepID)
	}
	if entry.CorrelationID != "corr-123" {
		t.Errorf("expected correlationID 'corr-123', got %s", entry.CorrelationID)
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestLogAggregator_AddLogEntry(t *testing.T) {
	la := NewLogAggregator()

	run := &Run{ID: "test-run"}
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "error",
		Message:   "custom entry",
	}

	la.AddLogEntry(run, entry)

	if len(run.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(run.Logs))
	}
	if run.Logs[0].Message != "custom entry" {
		t.Error("expected custom entry to be added")
	}
}

func TestLogAggregator_Subscribe(t *testing.T) {
	la := NewLogAggregator()
	runID := "test-run"

	ch, unsub := la.Subscribe(runID)
	defer unsub()

	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	// Verify subscriber was added
	if la.SubscriberCount(runID) != 1 {
		t.Error("expected 1 subscriber")
	}
}

func TestLogAggregator_Subscribe_ReceivesLogs(t *testing.T) {
	la := NewLogAggregator()

	run := &Run{ID: "test-run"}
	ch, unsub := la.Subscribe(run.ID)
	defer unsub()

	// Add log in separate goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		la.AddLog(run, "info", "broadcast message", "")
	}()

	// Wait for log
	select {
	case entry := <-ch:
		if entry.Message != "broadcast message" {
			t.Errorf("expected 'broadcast message', got %s", entry.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for log entry")
	}
}

func TestLogAggregator_Unsubscribe(t *testing.T) {
	la := NewLogAggregator()
	runID := "test-run"

	_, unsub := la.Subscribe(runID)

	if la.SubscriberCount(runID) != 1 {
		t.Error("expected 1 subscriber before unsub")
	}

	unsub()

	if la.SubscriberCount(runID) != 0 {
		t.Error("expected 0 subscribers after unsub")
	}

	// Channel is not closed (to avoid race conditions with concurrent senders),
	// but no more messages will be sent to it after unsubscribe.
	// Just verify subscriber count is 0.
}

func TestLogAggregator_MultipleSubscribers(t *testing.T) {
	la := NewLogAggregator()

	run := &Run{ID: "test-run"}

	ch1, unsub1 := la.Subscribe(run.ID)
	ch2, unsub2 := la.Subscribe(run.ID)
	defer unsub1()
	defer unsub2()

	if la.SubscriberCount(run.ID) != 2 {
		t.Error("expected 2 subscribers")
	}

	// Both should receive the log
	go func() {
		la.AddLog(run, "info", "multi-subscriber test", "")
	}()

	received := 0
	timeout := time.After(100 * time.Millisecond)
	for received < 2 {
		select {
		case entry := <-ch1:
			if entry.Message == "multi-subscriber test" {
				received++
			}
		case entry := <-ch2:
			if entry.Message == "multi-subscriber test" {
				received++
			}
		case <-timeout:
			t.Errorf("timeout: only received %d/2 logs", received)
			return
		}
	}
}

func TestLogAggregator_ChannelFull(t *testing.T) {
	la := NewLogAggregator()

	run := &Run{ID: "test-run"}
	ch, unsub := la.Subscribe(run.ID)
	defer unsub()

	// Fill the channel (buffer size is 100)
	for i := 0; i < 100; i++ {
		la.AddLog(run, "info", "filling buffer", "")
	}

	// This should not block - logs dropped when channel is full
	done := make(chan struct{})
	go func() {
		la.AddLog(run, "info", "overflow message", "")
		close(done)
	}()

	select {
	case <-done:
		// Success - didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("AddLog blocked on full channel")
	}

	// Drain the channel
	for i := 0; i < 100; i++ {
		<-ch
	}
}

func TestLogAggregator_SubscriberCount(t *testing.T) {
	la := NewLogAggregator()
	runID := "test-run"

	if la.SubscriberCount(runID) != 0 {
		t.Error("expected 0 subscribers initially")
	}

	_, unsub1 := la.Subscribe(runID)
	if la.SubscriberCount(runID) != 1 {
		t.Error("expected 1 subscriber")
	}

	_, unsub2 := la.Subscribe(runID)
	if la.SubscriberCount(runID) != 2 {
		t.Error("expected 2 subscribers")
	}

	unsub1()
	if la.SubscriberCount(runID) != 1 {
		t.Error("expected 1 subscriber after first unsub")
	}

	unsub2()
	if la.SubscriberCount(runID) != 0 {
		t.Error("expected 0 subscribers after second unsub")
	}
}

func TestLogAggregator_Concurrent(t *testing.T) {
	la := NewLogAggregator()
	run := &Run{ID: "test-run"}

	var wg sync.WaitGroup

	// Concurrent subscriptions
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, unsub := la.Subscribe(run.ID)
			time.Sleep(10 * time.Millisecond)
			unsub()
		}()
	}

	// Concurrent log additions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			la.AddLog(run, "info", "concurrent log", "")
		}(i)
	}

	wg.Wait()

	// Verify logs were added
	if len(run.Logs) < 10 {
		t.Errorf("expected at least 10 logs, got %d", len(run.Logs))
	}
}

func TestLogAggregator_DifferentRunIDs(t *testing.T) {
	la := NewLogAggregator()

	run1 := &Run{ID: "run-1"}
	run2 := &Run{ID: "run-2"}

	ch1, unsub1 := la.Subscribe(run1.ID)
	ch2, unsub2 := la.Subscribe(run2.ID)
	defer unsub1()
	defer unsub2()

	// Log to run1 - only ch1 should receive
	la.AddLog(run1, "info", "message for run1", "")

	select {
	case entry := <-ch1:
		if entry.Message != "message for run1" {
			t.Error("wrong message received on ch1")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("ch1 should have received message")
	}

	select {
	case <-ch2:
		t.Error("ch2 should not have received message for run1")
	case <-time.After(10 * time.Millisecond):
		// Expected - no message for ch2
	}
}
