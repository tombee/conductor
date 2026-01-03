// Package format provides output format validation and CLI formatting functions.
package format

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ValidateString accepts any value (no validation).
func ValidateString(value interface{}) error {
	// String format accepts any value
	return nil
}

// ValidateNumber validates that the value can be parsed as a number.
// Accepts integers, floats, and scientific notation.
// Rejects empty strings, null, non-numeric strings, booleans, and objects.
func ValidateNumber(value interface{}) error {
	if value == nil {
		return fmt.Errorf("expected valid number")
	}

	// Convert to string for parsing
	var str string
	switch v := value.(type) {
	case string:
		str = v
	case float64, float32, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		// Already a number type, valid
		return nil
	case bool:
		return fmt.Errorf("expected valid number")
	default:
		return fmt.Errorf("expected valid number")
	}

	// Check if string is empty
	if strings.TrimSpace(str) == "" {
		return fmt.Errorf("expected valid number")
	}

	// Try to parse as float64 (handles integers, floats, and scientific notation)
	_, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return fmt.Errorf("expected valid number")
	}

	return nil
}

// ValidateMarkdown accepts any string including empty strings.
// Rejects null values.
func ValidateMarkdown(value interface{}) error {
	if value == nil {
		return fmt.Errorf("expected valid markdown")
	}

	// Must be a string
	if _, ok := value.(string); !ok {
		return fmt.Errorf("expected valid markdown")
	}

	return nil
}

// ValidateJSON validates that the value is parseable JSON.
// Rejects empty strings and null values.
func ValidateJSON(value interface{}) error {
	if value == nil {
		return fmt.Errorf("expected valid JSON")
	}

	// Convert to string for JSON parsing
	var str string
	switch v := value.(type) {
	case string:
		str = v
	default:
		// If it's already a Go object/array, try to marshal and unmarshal it
		// to verify it's valid JSON-serializable
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("expected valid JSON")
		}
		str = string(data)
	}

	// Check if string is empty
	if strings.TrimSpace(str) == "" {
		return fmt.Errorf("expected valid JSON")
	}

	// Try to parse as JSON
	var result interface{}
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		return fmt.Errorf("expected valid JSON")
	}

	return nil
}

// ValidateCode validates that the value is a non-empty string.
// Rejects empty strings and null values.
func ValidateCode(value interface{}) error {
	if value == nil {
		return fmt.Errorf("expected valid code")
	}

	// Must be a string
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected valid code")
	}

	// Must not be empty
	if strings.TrimSpace(str) == "" {
		return fmt.Errorf("expected valid code")
	}

	return nil
}

// Validate validates an output value against its format.
// Returns a generic error message that doesn't include the actual value.
func Validate(format string, value interface{}) error {
	if format == "" {
		format = "string"
	}

	// Normalize format to lowercase
	formatLower := strings.ToLower(format)

	// Handle code with language (e.g., "code:python")
	if strings.HasPrefix(formatLower, "code:") {
		return ValidateCode(value)
	}

	// Dispatch to specific validator
	switch formatLower {
	case "string":
		return ValidateString(value)
	case "number":
		return ValidateNumber(value)
	case "markdown":
		return ValidateMarkdown(value)
	case "json":
		return ValidateJSON(value)
	case "code":
		return ValidateCode(value)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}
