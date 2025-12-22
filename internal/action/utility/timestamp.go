package utility

import (
	"context"
	"fmt"
	"time"
)

// timestamp returns the current time in various formats.
func (c *UtilityAction) timestamp(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get optional format parameter (default: rfc3339)
	format := "rfc3339"
	if f, ok := inputs["format"]; ok {
		formatStr, ok := f.(string)
		if !ok {
			return nil, &OperationError{
				Operation:  "timestamp",
				Message:    "format must be a string",
				ErrorType:  ErrorTypeType,
				Suggestion: "Provide 'format' as a string: unix, unix_ms, rfc3339, iso8601, or a custom Go time format",
			}
		}
		format = formatStr
	}

	// Get optional timezone parameter (default: UTC)
	loc := time.UTC
	if tz, ok := inputs["timezone"]; ok {
		tzStr, ok := tz.(string)
		if !ok {
			return nil, &OperationError{
				Operation:  "timestamp",
				Message:    "timezone must be a string",
				ErrorType:  ErrorTypeType,
				Suggestion: "Provide 'timezone' as a string (e.g., 'America/New_York', 'UTC', 'Local')",
			}
		}

		var err error
		if tzStr == "Local" {
			loc = time.Local
		} else {
			loc, err = time.LoadLocation(tzStr)
			if err != nil {
				return nil, &OperationError{
					Operation:  "timestamp",
					Message:    fmt.Sprintf("invalid timezone: %s", tzStr),
					ErrorType:  ErrorTypeValidation,
					Cause:      err,
					Suggestion: "Use a valid IANA timezone name (e.g., 'America/New_York', 'Europe/London', 'UTC')",
				}
			}
		}
	}

	now := time.Now().In(loc)

	var response interface{}
	var formatUsed string

	switch format {
	case "unix":
		response = now.Unix()
		formatUsed = "unix"
	case "unix_ms":
		response = now.UnixMilli()
		formatUsed = "unix_ms"
	case "rfc3339":
		response = now.Format(time.RFC3339)
		formatUsed = "rfc3339"
	case "iso8601":
		// ISO8601 with milliseconds
		response = now.Format("2006-01-02T15:04:05.000Z07:00")
		formatUsed = "iso8601"
	default:
		// Treat as custom Go time format
		formatted := now.Format(format)
		// Basic validation: if the formatted result equals the format string,
		// it likely means the format had no valid directives
		if formatted == format && len(format) > 0 {
			// Check if format contains any valid Go time format directives
			// by looking for common patterns
			hasValidDirective := false
			directives := []string{"2006", "06", "01", "02", "15", "03", "04", "05", "PM", "pm", "Mon", "Jan", "MST", "Z07", "-07"}
			for _, d := range directives {
				if containsSubstring(format, d) {
					hasValidDirective = true
					break
				}
			}
			if !hasValidDirective {
				return nil, &OperationError{
					Operation:  "timestamp",
					Message:    fmt.Sprintf("invalid or unrecognized format: %s", format),
					ErrorType:  ErrorTypeValidation,
					Suggestion: "Use a valid format: unix, unix_ms, rfc3339, iso8601, or a custom Go time format (e.g., '2006-01-02')",
				}
			}
		}
		response = formatted
		formatUsed = "custom"
	}

	return &Result{
		Response: response,
		Metadata: map[string]interface{}{
			"operation": "timestamp",
			"format":    formatUsed,
			"timezone":  loc.String(),
		},
	}, nil
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
