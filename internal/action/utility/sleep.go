package utility

import (
	"context"
	"fmt"
	"time"
)

// MaxSleepDuration is the maximum allowed sleep duration to prevent abuse.
const MaxSleepDuration = 5 * time.Minute

// sleep pauses workflow execution for a specified duration.
// It accepts either a "duration" string (e.g., "5s", "100ms", "1m") or
// "milliseconds" as an integer.
func (c *UtilityAction) sleep(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	var sleepDuration time.Duration

	// Check for duration string parameter
	if durationStr, ok := inputs["duration"]; ok {
		str, ok := durationStr.(string)
		if !ok {
			return nil, &OperationError{
				Operation:  "sleep",
				Message:    "invalid 'duration' parameter",
				ErrorType:  ErrorTypeType,
				Suggestion: "Provide 'duration' as a string (e.g., \"5s\", \"100ms\", \"1m\")",
			}
		}

		parsed, err := time.ParseDuration(str)
		if err != nil {
			return nil, &OperationError{
				Operation:  "sleep",
				Message:    fmt.Sprintf("invalid duration format: %s", str),
				ErrorType:  ErrorTypeValidation,
				Cause:      err,
				Suggestion: "Use Go duration format (e.g., \"5s\", \"100ms\", \"1m30s\")",
			}
		}
		sleepDuration = parsed
	} else if msVal, ok := inputs["milliseconds"]; ok {
		// Check for milliseconds integer parameter
		ms, err := toInt64(msVal)
		if err != nil {
			return nil, &OperationError{
				Operation:  "sleep",
				Message:    "invalid 'milliseconds' parameter",
				ErrorType:  ErrorTypeType,
				Cause:      err,
				Suggestion: "Provide 'milliseconds' as an integer",
			}
		}
		sleepDuration = time.Duration(ms) * time.Millisecond
	} else {
		return nil, &OperationError{
			Operation:  "sleep",
			Message:    "missing duration parameter",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide either 'duration' (string, e.g., \"5s\") or 'milliseconds' (integer)",
		}
	}

	// Validate duration is positive
	if sleepDuration <= 0 {
		return nil, &OperationError{
			Operation:  "sleep",
			Message:    "duration must be positive",
			ErrorType:  ErrorTypeRange,
			Suggestion: "Provide a positive duration value",
		}
	}

	// Enforce maximum duration limit
	if sleepDuration > MaxSleepDuration {
		return nil, &OperationError{
			Operation:  "sleep",
			Message:    fmt.Sprintf("duration %v exceeds maximum allowed (%v)", sleepDuration, MaxSleepDuration),
			ErrorType:  ErrorTypeRange,
			Suggestion: fmt.Sprintf("Reduce duration to at most %v", MaxSleepDuration),
		}
	}

	// Perform the sleep with context cancellation support
	startTime := time.Now()
	select {
	case <-time.After(sleepDuration):
		// Sleep completed normally
	case <-ctx.Done():
		// Context was cancelled
		actualDuration := time.Since(startTime)
		return nil, &OperationError{
			Operation:  "sleep",
			Message:    "sleep cancelled",
			ErrorType:  ErrorTypeInternal,
			Cause:      ctx.Err(),
			Suggestion: fmt.Sprintf("Sleep was interrupted after %v of %v", actualDuration.Round(time.Millisecond), sleepDuration),
		}
	}

	actualDuration := time.Since(startTime)

	return &Result{
		Response: sleepDuration.Milliseconds(),
		Metadata: map[string]interface{}{
			"operation":             "sleep",
			"requested_duration":    sleepDuration.String(),
			"actual_duration_ms":    actualDuration.Milliseconds(),
			"requested_duration_ms": sleepDuration.Milliseconds(),
		},
	}, nil
}

// toInt64 converts various numeric types to int64.
func toInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case float64:
		return int64(n), nil
	case float32:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}
