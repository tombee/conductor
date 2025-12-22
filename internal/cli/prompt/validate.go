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

package prompt

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ValidateString validates a string input.
// Rejects null bytes, control characters, and oversized inputs.
func ValidateString(input string) error {
	if len(input) > MaxInputSize {
		return fmt.Errorf("input exceeds maximum size of %d bytes", MaxInputSize)
	}

	for i, r := range input {
		if r == 0 {
			return fmt.Errorf("input contains null byte at position %d", i)
		}
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return fmt.Errorf("input contains invalid control character at position %d", i)
		}
	}

	return nil
}

// ValidateNumber validates and parses a numeric input.
func ValidateNumber(input string) (float64, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, fmt.Errorf("input is empty")
	}

	num, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return 0, fmt.Errorf("input must be a number")
	}

	return num, nil
}

// ValidateBool validates and parses a boolean input.
// Accepts: y/yes/true/1 and n/no/false/0 (case-insensitive).
func ValidateBool(input string) (bool, error) {
	input = strings.ToLower(strings.TrimSpace(input))

	switch input {
	case "y", "yes", "true", "1":
		return true, nil
	case "n", "no", "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("input must be y/yes/true/1 or n/no/false/0")
	}
}

// ValidateEnum validates an enum selection.
func ValidateEnum(input string, options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options available")
	}

	// Check if input is a number (1-indexed selection)
	if idx, err := strconv.Atoi(strings.TrimSpace(input)); err == nil {
		if idx < 1 || idx > len(options) {
			return "", fmt.Errorf("selection must be between 1 and %d", len(options))
		}
		return options[idx-1], nil
	}

	// Check if input matches an option directly
	for _, opt := range options {
		if strings.EqualFold(strings.TrimSpace(input), opt) {
			return opt, nil
		}
	}

	return "", fmt.Errorf("input must be a valid option or number between 1 and %d", len(options))
}

// ValidateArray validates and parses an array input.
// Supports comma-separated values and JSON array syntax.
func ValidateArray(input string) ([]interface{}, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return []interface{}{}, nil
	}

	// Try JSON array parsing first
	if strings.HasPrefix(input, "[") {
		var arr []interface{}
		if err := json.Unmarshal([]byte(input), &arr); err != nil {
			return nil, fmt.Errorf("invalid JSON array: %w", err)
		}
		return arr, nil
	}

	// Parse as comma-separated values with backslash escape support
	result := make([]interface{}, 0)
	var current strings.Builder
	escaped := false

	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			continue
		}

		if r == ',' {
			val := strings.TrimSpace(current.String())
			if val != "" {
				result = append(result, val)
			}
			current.Reset()
			continue
		}

		current.WriteRune(r)
	}

	// Add the last value
	val := strings.TrimSpace(current.String())
	if val != "" {
		result = append(result, val)
	}

	return result, nil
}

// ValidateObject validates and parses an object input.
// Enforces strict JSON parsing with maximum nesting depth.
func ValidateObject(input string) (map[string]interface{}, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("input is empty")
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(input), &obj); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}

	// Check nesting depth
	if err := checkDepth(obj, 0); err != nil {
		return nil, err
	}

	return obj, nil
}

// checkDepth recursively checks the nesting depth of an object.
func checkDepth(v interface{}, depth int) error {
	if depth > MaxNestedDepth {
		return fmt.Errorf("object nesting exceeds maximum depth of %d", MaxNestedDepth)
	}

	switch val := v.(type) {
	case map[string]interface{}:
		for _, nested := range val {
			if err := checkDepth(nested, depth+1); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, nested := range val {
			if err := checkDepth(nested, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
}
