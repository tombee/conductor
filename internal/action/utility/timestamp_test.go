package utility

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestTimestamp(t *testing.T) {
	uc, _ := New(nil)
	ctx := context.Background()

	t.Run("default format is rfc3339", func(t *testing.T) {
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts, ok := result.Response.(string)
		if !ok {
			t.Fatalf("expected string response, got %T", result.Response)
		}

		// Validate RFC3339 format
		_, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			t.Errorf("invalid RFC3339 format: %s, error: %v", ts, err)
		}

		// Check metadata
		if result.Metadata["format"] != "rfc3339" {
			t.Errorf("expected format 'rfc3339' in metadata, got %v", result.Metadata["format"])
		}
	})

	t.Run("unix format returns seconds", func(t *testing.T) {
		before := time.Now().Unix()
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format": "unix",
		})
		after := time.Now().Unix()

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts, ok := result.Response.(int64)
		if !ok {
			t.Fatalf("expected int64 response, got %T", result.Response)
		}

		if ts < before || ts > after {
			t.Errorf("unix timestamp %d not in expected range [%d, %d]", ts, before, after)
		}
	})

	t.Run("unix_ms format returns milliseconds", func(t *testing.T) {
		before := time.Now().UnixMilli()
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format": "unix_ms",
		})
		after := time.Now().UnixMilli()

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts, ok := result.Response.(int64)
		if !ok {
			t.Fatalf("expected int64 response, got %T", result.Response)
		}

		if ts < before || ts > after {
			t.Errorf("unix_ms timestamp %d not in expected range [%d, %d]", ts, before, after)
		}

		// Should be roughly 1000x larger than unix seconds
		if ts < 1000000000000 {
			t.Errorf("unix_ms timestamp %d seems too small for milliseconds", ts)
		}
	})

	t.Run("rfc3339 format", func(t *testing.T) {
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format": "rfc3339",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts, ok := result.Response.(string)
		if !ok {
			t.Fatalf("expected string response, got %T", result.Response)
		}

		_, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			t.Errorf("invalid RFC3339 format: %s", ts)
		}
	})

	t.Run("iso8601 format", func(t *testing.T) {
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format": "iso8601",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts, ok := result.Response.(string)
		if !ok {
			t.Fatalf("expected string response, got %T", result.Response)
		}

		// ISO8601 with milliseconds: 2006-01-02T15:04:05.000Z07:00
		iso8601Pattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}[Z+-]`)
		if !iso8601Pattern.MatchString(ts) {
			t.Errorf("invalid ISO8601 format: %s", ts)
		}
	})

	t.Run("custom Go format", func(t *testing.T) {
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format": "2006-01-02",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts, ok := result.Response.(string)
		if !ok {
			t.Fatalf("expected string response, got %T", result.Response)
		}

		// Validate date format YYYY-MM-DD
		datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
		if !datePattern.MatchString(ts) {
			t.Errorf("invalid date format: %s", ts)
		}

		// Verify it can be parsed
		_, err = time.Parse("2006-01-02", ts)
		if err != nil {
			t.Errorf("failed to parse date: %v", err)
		}
	})

	t.Run("custom format with time", func(t *testing.T) {
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format": "15:04:05",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts, ok := result.Response.(string)
		if !ok {
			t.Fatalf("expected string response, got %T", result.Response)
		}

		// Validate time format HH:MM:SS
		timePattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
		if !timePattern.MatchString(ts) {
			t.Errorf("invalid time format: %s", ts)
		}
	})

	t.Run("timezone UTC", func(t *testing.T) {
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format":   "rfc3339",
			"timezone": "UTC",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts, ok := result.Response.(string)
		if !ok {
			t.Fatalf("expected string response, got %T", result.Response)
		}

		// UTC timestamps should end with Z
		if ts[len(ts)-1] != 'Z' {
			t.Errorf("expected UTC timestamp to end with 'Z', got: %s", ts)
		}

		if result.Metadata["timezone"] != "UTC" {
			t.Errorf("expected timezone 'UTC' in metadata, got %v", result.Metadata["timezone"])
		}
	})

	t.Run("timezone America/New_York", func(t *testing.T) {
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format":   "rfc3339",
			"timezone": "America/New_York",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts, ok := result.Response.(string)
		if !ok {
			t.Fatalf("expected string response, got %T", result.Response)
		}

		// Parse and verify timezone
		parsed, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			t.Fatalf("failed to parse timestamp: %v", err)
		}

		// New York is either -05:00 or -04:00 (depending on DST)
		_, offset := parsed.Zone()
		if offset != -5*3600 && offset != -4*3600 {
			t.Errorf("unexpected timezone offset for New York: %d", offset)
		}
	})

	t.Run("timezone Local", func(t *testing.T) {
		result, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format":   "rfc3339",
			"timezone": "Local",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, ok := result.Response.(string)
		if !ok {
			t.Fatalf("expected string response, got %T", result.Response)
		}

		if result.Metadata["timezone"] != "Local" {
			t.Errorf("expected timezone 'Local' in metadata, got %v", result.Metadata["timezone"])
		}
	})

	t.Run("invalid timezone returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"timezone": "Invalid/Timezone",
		})
		if err == nil {
			t.Fatal("expected error for invalid timezone")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeValidation {
			t.Errorf("expected ErrorTypeValidation, got %v", opErr.ErrorType)
		}
	})

	t.Run("invalid format type returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format": 123,
		})
		if err == nil {
			t.Fatal("expected error for non-string format")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeType {
			t.Errorf("expected ErrorTypeType, got %v", opErr.ErrorType)
		}
	})

	t.Run("invalid timezone type returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"timezone": 123,
		})
		if err == nil {
			t.Fatal("expected error for non-string timezone")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeType {
			t.Errorf("expected ErrorTypeType, got %v", opErr.ErrorType)
		}
	})

	t.Run("invalid custom format returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format": "not-a-valid-format",
		})
		if err == nil {
			t.Fatal("expected error for invalid format")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeValidation {
			t.Errorf("expected ErrorTypeValidation, got %v", opErr.ErrorType)
		}
	})

	t.Run("unix with timezone still works", func(t *testing.T) {
		// Unix timestamps are timezone-independent (always UTC seconds since epoch)
		result1, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format":   "unix",
			"timezone": "UTC",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result2, err := uc.Execute(ctx, "timestamp", map[string]interface{}{
			"format":   "unix",
			"timezone": "America/New_York",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ts1 := result1.Response.(int64)
		ts2 := result2.Response.(int64)

		// Should be very close (within 1 second)
		if ts2-ts1 > 1 || ts1-ts2 > 1 {
			t.Errorf("unix timestamps with different timezones should be equal: %d vs %d", ts1, ts2)
		}
	})
}
