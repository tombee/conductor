package permissions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuditLogger(t *testing.T) {
	t.Run("basic event logging", func(t *testing.T) {
		var logged []AuditEvent
		logger := NewAuditLogger(100, func(event AuditEvent) {
			logged = append(logged, event)
		})

		event := AuditEvent{
			Type:           EventPermissionDenied,
			WorkflowID:     "wf-123",
			StepID:         "step-1",
			PermissionType: "paths.read",
			Resource:       "/etc/passwd",
			Allowed:        []string{"src/**"},
			Message:        "path not allowed",
			Enforced:       true,
		}

		success := logger.Log(event)
		assert.True(t, success)
		assert.Len(t, logged, 1)
		assert.Equal(t, EventPermissionDenied, logged[0].Type)
		assert.Equal(t, "wf-123", logged[0].WorkflowID)
		assert.False(t, logged[0].Timestamp.IsZero())
	})

	t.Run("rate limiting per workflow", func(t *testing.T) {
		var logged []AuditEvent
		logger := NewAuditLogger(5, func(event AuditEvent) {
			logged = append(logged, event)
		})

		now := time.Now()

		// Log 5 events - should all succeed
		for i := 0; i < 5; i++ {
			event := AuditEvent{
				Type:       EventPermissionDenied,
				WorkflowID: "wf-123",
				StepID:     "step-1",
				Timestamp:  now,
			}
			success := logger.Log(event)
			assert.True(t, success, "event %d should succeed", i)
		}

		assert.Len(t, logged, 5)

		// 6th event should be rate limited
		event := AuditEvent{
			Type:       EventPermissionDenied,
			WorkflowID: "wf-123",
			StepID:     "step-1",
			Timestamp:  now,
		}
		success := logger.Log(event)
		assert.False(t, success, "6th event should be rate limited")
		assert.Len(t, logged, 5)
	})

	t.Run("rate limiting resets after 1 minute", func(t *testing.T) {
		var logged []AuditEvent
		logger := NewAuditLogger(2, func(event AuditEvent) {
			logged = append(logged, event)
		})

		now := time.Now()

		// Log 2 events at time T
		for i := 0; i < 2; i++ {
			event := AuditEvent{
				Type:       EventPermissionDenied,
				WorkflowID: "wf-123",
				StepID:     "step-1",
				Timestamp:  now,
			}
			success := logger.Log(event)
			assert.True(t, success)
		}

		// 3rd event should be rate limited
		event := AuditEvent{
			Type:       EventPermissionDenied,
			WorkflowID: "wf-123",
			StepID:     "step-1",
			Timestamp:  now,
		}
		success := logger.Log(event)
		assert.False(t, success)
		assert.Len(t, logged, 2)

		// Log event at T+61 seconds - should succeed (old events expired)
		futureEvent := AuditEvent{
			Type:       EventPermissionDenied,
			WorkflowID: "wf-123",
			StepID:     "step-1",
			Timestamp:  now.Add(61 * time.Second),
		}
		success = logger.Log(futureEvent)
		assert.True(t, success)
		assert.Len(t, logged, 3)
	})

	t.Run("rate limiting is per workflow", func(t *testing.T) {
		var logged []AuditEvent
		logger := NewAuditLogger(2, func(event AuditEvent) {
			logged = append(logged, event)
		})

		now := time.Now()

		// Log 2 events for workflow 1
		for i := 0; i < 2; i++ {
			event := AuditEvent{
				Type:       EventPermissionDenied,
				WorkflowID: "wf-1",
				StepID:     "step-1",
				Timestamp:  now,
			}
			success := logger.Log(event)
			assert.True(t, success)
		}

		// Log 2 events for workflow 2 - should succeed (different workflow)
		for i := 0; i < 2; i++ {
			event := AuditEvent{
				Type:       EventPermissionDenied,
				WorkflowID: "wf-2",
				StepID:     "step-1",
				Timestamp:  now,
			}
			success := logger.Log(event)
			assert.True(t, success)
		}

		assert.Len(t, logged, 4)

		// 3rd event for workflow 1 should be rate limited
		event := AuditEvent{
			Type:       EventPermissionDenied,
			WorkflowID: "wf-1",
			StepID:     "step-1",
			Timestamp:  now,
		}
		success := logger.Log(event)
		assert.False(t, success)
		assert.Len(t, logged, 4)
	})

	t.Run("LogPermissionDenied helper", func(t *testing.T) {
		var logged []AuditEvent
		logger := NewAuditLogger(100, func(event AuditEvent) {
			logged = append(logged, event)
		})

		ctx := context.Background()
		permErr := &PermissionError{
			Type:     "paths.read",
			Resource: "/etc/passwd",
			Allowed:  []string{"src/**"},
			Message:  "path not allowed",
		}

		success := logger.LogPermissionDenied(ctx, "wf-123", "step-1", permErr, true)
		assert.True(t, success)
		assert.Len(t, logged, 1)
		assert.Equal(t, EventPermissionDenied, logged[0].Type)
		assert.Equal(t, "paths.read", logged[0].PermissionType)
		assert.True(t, logged[0].Enforced)
	})

	t.Run("LogBaselineBlocked helper", func(t *testing.T) {
		var logged []AuditEvent
		logger := NewAuditLogger(100, func(event AuditEvent) {
			logged = append(logged, event)
		})

		ctx := context.Background()
		success := logger.LogBaselineBlocked(ctx, "wf-123", "step-1", "network.metadata", "169.254.169.254", "metadata endpoint blocked")
		assert.True(t, success)
		assert.Len(t, logged, 1)
		assert.Equal(t, EventBaselineBlocked, logged[0].Type)
		assert.True(t, logged[0].Enforced)
	})

	t.Run("LogWouldBlock helper", func(t *testing.T) {
		var logged []AuditEvent
		logger := NewAuditLogger(100, func(event AuditEvent) {
			logged = append(logged, event)
		})

		ctx := context.Background()
		permErr := &PermissionError{
			Type:     "tools.denied",
			Resource: "shell.run",
			Allowed:  []string{"file.*"},
			Message:  "tool not allowed",
		}

		success := logger.LogWouldBlock(ctx, "wf-123", "step-1", permErr)
		assert.True(t, success)
		assert.Len(t, logged, 1)
		assert.Equal(t, EventPermissionWouldBlock, logged[0].Type)
		assert.False(t, logged[0].Enforced)
	})

	t.Run("GetEventCount", func(t *testing.T) {
		logger := NewAuditLogger(100, nil)

		now := time.Now()

		// Log 3 events
		for i := 0; i < 3; i++ {
			event := AuditEvent{
				Type:       EventPermissionDenied,
				WorkflowID: "wf-123",
				StepID:     "step-1",
				Timestamp:  now,
			}
			logger.Log(event)
		}

		count := logger.GetEventCount("wf-123")
		assert.Equal(t, 3, count)

		// Different workflow should have 0
		count = logger.GetEventCount("wf-456")
		assert.Equal(t, 0, count)
	})

	t.Run("Reset", func(t *testing.T) {
		logger := NewAuditLogger(100, nil)

		now := time.Now()

		// Log events
		for i := 0; i < 3; i++ {
			event := AuditEvent{
				Type:       EventPermissionDenied,
				WorkflowID: "wf-123",
				StepID:     "step-1",
				Timestamp:  now,
			}
			logger.Log(event)
		}

		assert.Equal(t, 3, logger.GetEventCount("wf-123"))

		// Reset
		logger.Reset("wf-123")
		assert.Equal(t, 0, logger.GetEventCount("wf-123"))
	})

	t.Run("default rate limit", func(t *testing.T) {
		// Passing 0 should default to 100
		logger := NewAuditLogger(0, nil)
		assert.Equal(t, 100, logger.maxEventsPerMinute)

		// Passing negative should default to 100
		logger = NewAuditLogger(-1, nil)
		assert.Equal(t, 100, logger.maxEventsPerMinute)
	})
}
