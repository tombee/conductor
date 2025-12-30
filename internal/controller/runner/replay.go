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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tombee/conductor/internal/controller/backend"
)

// ValidateReplayConfig validates a replay configuration against business rules.
// It checks for required fields, validates override formats, and sanitizes inputs.
func ValidateReplayConfig(config *backend.ReplayConfig) error {
	if config == nil {
		return fmt.Errorf("replay config cannot be nil")
	}

	if config.ParentRunID == "" {
		return fmt.Errorf("parent_run_id is required")
	}

	// Validate override inputs are properly formatted and safe
	if err := validateOverrideInputs(config.OverrideInputs); err != nil {
		return fmt.Errorf("invalid override inputs: %w", err)
	}

	// Validate override steps are properly formatted JSON
	if err := validateOverrideSteps(config.OverrideSteps); err != nil {
		return fmt.Errorf("invalid override steps: %w", err)
	}

	// Validate cost limit is non-negative
	if config.MaxCost < 0 {
		return fmt.Errorf("max_cost cannot be negative: %f", config.MaxCost)
	}

	return nil
}

// validateOverrideInputs validates that override inputs are safe for template injection.
// This prevents template expressions from being injected through user-provided values.
func validateOverrideInputs(inputs map[string]any) error {
	if inputs == nil {
		return nil
	}

	// Pattern for detecting template expressions
	templatePattern := regexp.MustCompile(`\{\{|\}\}|\$\{`)

	for key, value := range inputs {
		// Validate key doesn't contain special characters
		if !isValidIdentifier(key) {
			return fmt.Errorf("invalid input key '%s': must be alphanumeric with underscores", key)
		}

		// Check string values for template injection attempts
		if str, ok := value.(string); ok {
			if templatePattern.MatchString(str) {
				return fmt.Errorf("input '%s' contains template expressions, which are not allowed in overrides", key)
			}
		}

		// Recursively validate nested maps
		if nested, ok := value.(map[string]any); ok {
			if err := validateOverrideInputs(nested); err != nil {
				return fmt.Errorf("in input '%s': %w", key, err)
			}
		}
	}

	return nil
}

// validateOverrideSteps validates that step overrides are properly formatted.
func validateOverrideSteps(steps map[string]any) error {
	if steps == nil {
		return nil
	}

	for stepID, value := range steps {
		// Validate step ID format
		if !isValidIdentifier(stepID) {
			return fmt.Errorf("invalid step ID '%s': must be alphanumeric with underscores", stepID)
		}

		// Validate value can be marshaled to JSON (ensures it's serializable)
		if _, err := json.Marshal(value); err != nil {
			return fmt.Errorf("step '%s' override value is not valid JSON: %w", stepID, err)
		}
	}

	return nil
}

// isValidIdentifier checks if a string is a valid identifier (alphanumeric + underscores).
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, s)
	return match
}

// SanitizeOverrideInputs sanitizes override input values to prevent injection attacks.
// This escapes special characters in string values while preserving data structure.
func SanitizeOverrideInputs(inputs map[string]any) map[string]any {
	if inputs == nil {
		return nil
	}

	sanitized := make(map[string]any)
	for key, value := range inputs {
		switch v := value.(type) {
		case string:
			// Escape template delimiters and shell special characters
			sanitized[key] = escapeStringValue(v)
		case map[string]any:
			// Recursively sanitize nested maps
			sanitized[key] = SanitizeOverrideInputs(v)
		case []any:
			// Sanitize array elements
			sanitized[key] = sanitizeArray(v)
		default:
			// Numbers, booleans, nil - pass through
			sanitized[key] = v
		}
	}
	return sanitized
}

// escapeStringValue escapes special characters that could be interpreted as template expressions.
func escapeStringValue(s string) string {
	// Replace template delimiters with HTML entities to prevent interpretation
	s = strings.ReplaceAll(s, "{{", "&#123;&#123;")
	s = strings.ReplaceAll(s, "}}", "&#125;&#125;")
	s = strings.ReplaceAll(s, "${", "&#36;&#123;")
	return s
}

// sanitizeArray sanitizes all elements in an array.
func sanitizeArray(arr []any) []any {
	sanitized := make([]any, len(arr))
	for i, elem := range arr {
		switch v := elem.(type) {
		case string:
			sanitized[i] = escapeStringValue(v)
		case map[string]any:
			sanitized[i] = SanitizeOverrideInputs(v)
		case []any:
			sanitized[i] = sanitizeArray(v)
		default:
			sanitized[i] = v
		}
	}
	return sanitized
}
