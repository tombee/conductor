package utility

import (
	"context"
	"testing"
	"time"
)

func TestSleep(t *testing.T) {
	uc, _ := New(nil)

	t.Run("with duration string", func(t *testing.T) {
		start := time.Now()
		result, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{
			"duration": "50ms",
		})
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}

		// Verify sleep duration was approximately correct (allow 20ms tolerance)
		if elapsed < 50*time.Millisecond {
			t.Errorf("sleep was too short: %v", elapsed)
		}
		if elapsed > 100*time.Millisecond {
			t.Errorf("sleep was too long: %v", elapsed)
		}

		// Verify response is the duration in milliseconds
		if result.Response != int64(50) {
			t.Errorf("expected response 50, got %v", result.Response)
		}

		// Verify metadata
		if result.Metadata["operation"] != "sleep" {
			t.Errorf("expected operation 'sleep', got %v", result.Metadata["operation"])
		}
		if result.Metadata["requested_duration"] != "50ms" {
			t.Errorf("expected requested_duration '50ms', got %v", result.Metadata["requested_duration"])
		}
	})

	t.Run("with milliseconds integer", func(t *testing.T) {
		start := time.Now()
		result, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{
			"milliseconds": 50,
		})
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}

		// Verify sleep duration was approximately correct
		if elapsed < 50*time.Millisecond {
			t.Errorf("sleep was too short: %v", elapsed)
		}

		if result.Response != int64(50) {
			t.Errorf("expected response 50, got %v", result.Response)
		}
	})

	t.Run("with milliseconds float64", func(t *testing.T) {
		result, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{
			"milliseconds": float64(25),
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if result.Response != int64(25) {
			t.Errorf("expected response 25, got %v", result.Response)
		}
	})

	t.Run("with various duration formats", func(t *testing.T) {
		// Test short durations that actually complete
		shortCases := []struct {
			duration string
			wantMs   int64
		}{
			{"10ms", 10},
			{"20ms", 20},
		}

		for _, tc := range shortCases {
			result, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{
				"duration": tc.duration,
			})
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.duration, err)
			}
			if result.Response != tc.wantMs {
				t.Errorf("for duration %s: expected %d ms, got %v", tc.duration, tc.wantMs, result.Response)
			}
		}

		// Test longer duration parsing by cancelling immediately
		// This verifies the duration is parsed correctly without waiting
		longCases := []struct {
			duration string
			wantMs   int64
		}{
			{"1s", 1000},
			{"1m", 60000},
			{"2m30s", 150000},
		}

		for _, tc := range longCases {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			_, err := uc.Execute(ctx, "sleep", map[string]interface{}{
				"duration": tc.duration,
			})
			// Should get a cancellation error, not a parsing error
			if err == nil {
				t.Fatalf("expected cancellation error for %s", tc.duration)
			}
			opErr, ok := err.(*OperationError)
			if !ok {
				t.Fatalf("expected OperationError for %s, got %T", tc.duration, err)
			}
			// Should be internal error from cancellation, not validation error from parsing
			if opErr.ErrorType != ErrorTypeInternal {
				t.Errorf("for duration %s: expected ErrorTypeInternal (parsing succeeded), got %v", tc.duration, opErr.ErrorType)
			}
		}
	})

	t.Run("error on missing duration parameter", func(t *testing.T) {
		_, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{})
		if err == nil {
			t.Fatal("expected error for missing duration")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeValidation {
			t.Errorf("expected ErrorTypeValidation, got %v", opErr.ErrorType)
		}
	})

	t.Run("error on invalid duration string", func(t *testing.T) {
		_, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{
			"duration": "invalid",
		})
		if err == nil {
			t.Fatal("expected error for invalid duration")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeValidation {
			t.Errorf("expected ErrorTypeValidation, got %v", opErr.ErrorType)
		}
	})

	t.Run("error on non-string duration", func(t *testing.T) {
		_, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{
			"duration": 123, // Should be a string
		})
		if err == nil {
			t.Fatal("expected error for non-string duration")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeType {
			t.Errorf("expected ErrorTypeType, got %v", opErr.ErrorType)
		}
	})

	t.Run("error on negative duration", func(t *testing.T) {
		_, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{
			"duration": "-5s",
		})
		if err == nil {
			t.Fatal("expected error for negative duration")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeRange {
			t.Errorf("expected ErrorTypeRange, got %v", opErr.ErrorType)
		}
	})

	t.Run("error on zero duration", func(t *testing.T) {
		_, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{
			"duration": "0s",
		})
		if err == nil {
			t.Fatal("expected error for zero duration")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeRange {
			t.Errorf("expected ErrorTypeRange, got %v", opErr.ErrorType)
		}
	})

	t.Run("error on duration exceeding maximum", func(t *testing.T) {
		_, err := uc.Execute(context.Background(), "sleep", map[string]interface{}{
			"duration": "10m", // Exceeds 5 minute max
		})
		if err == nil {
			t.Fatal("expected error for duration exceeding maximum")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeRange {
			t.Errorf("expected ErrorTypeRange, got %v", opErr.ErrorType)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel context after 30ms
		go func() {
			time.Sleep(30 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		_, err := uc.Execute(ctx, "sleep", map[string]interface{}{
			"duration": "1s",
		})
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected error when context is cancelled")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeInternal {
			t.Errorf("expected ErrorTypeInternal, got %v", opErr.ErrorType)
		}

		// Should have exited early due to cancellation
		if elapsed > 200*time.Millisecond {
			t.Errorf("sleep should have been cancelled early, took %v", elapsed)
		}
	})
}

func TestMaxSleepDuration(t *testing.T) {
	// Verify the constant is set correctly
	if MaxSleepDuration != 5*time.Minute {
		t.Errorf("expected MaxSleepDuration to be 5 minutes, got %v", MaxSleepDuration)
	}
}
