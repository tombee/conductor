package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Sensitive field patterns for redaction (case-insensitive substring matching)
var sensitivePatterns = []string{
	"password", "token", "key", "secret", "api_key", "auth", "credential",
}

// parseJSON implements the parse_json operation.
// Extracts and parses JSON from text, handling markdown code blocks.
func (c *TransformAction) parseJSON(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Extract data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "parse_json",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with JSON text to parse",
		}
	}

	// Handle null/undefined input
	if data == nil {
		return nil, &OperationError{
			Operation:  "parse_json",
			Message:    "input is null or undefined",
			ErrorType:  ErrorTypeEmptyInput,
			Suggestion: "Ensure the input contains valid data before parsing",
		}
	}

	// If already parsed (object or array), return as-is
	switch v := data.(type) {
	case map[string]interface{}, []interface{}:
		return &Result{
			Response: v,
			Metadata: map[string]interface{}{
				"already_parsed": true,
			},
		}, nil
	}

	// Must be a string to parse
	text, ok := data.(string)
	if !ok {
		return nil, &OperationError{
			Operation:  "parse_json",
			Message:    fmt.Sprintf("data must be string or JSON object/array, got %T", data),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Convert data to string or ensure it's already valid JSON",
		}
	}

	// Extract JSON from text using the algorithm from spec
	jsonText, extractionMethod := extractJSON(text)
	if jsonText == "" {
		return nil, &OperationError{
			Operation:  "parse_json",
			Message:    "no valid JSON structure found in input",
			ErrorType:  ErrorTypeParseError,
			Context:    redactSensitive(truncate(text, 100)),
			Suggestion: "Check if the input contains JSON, use output_schema on LLM steps, or try transform.extract",
		}
	}

	// Parse the JSON
	var result interface{}
	err := json.Unmarshal([]byte(jsonText), &result)
	if err != nil {
		// Calculate error position and context
		pos := 0
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			pos = int(syntaxErr.Offset)
		}

		// Get context around error position
		contextStart := max(0, pos-20)
		contextEnd := min(len(jsonText), pos+20)
		errorContext := jsonText[contextStart:contextEnd]

		return nil, &OperationError{
			Operation:  "parse_json",
			Message:    "invalid JSON syntax",
			ErrorType:  ErrorTypeParseError,
			Cause:      err,
			Position:   pos,
			Context:    redactSensitive(errorContext),
			Suggestion: "Verify JSON syntax, check for missing brackets/quotes, or use output_schema on LLM step",
		}
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"extraction_method": extractionMethod,
		},
	}, nil
}

// extractJSON extracts JSON from text following the algorithm in the spec:
// 1. If already object/array JSON, parse directly
// 2. If starts with { or [, parse directly
// 3. If contains ```json fence, extract from fence
// 4. If contains any ```, try each block
// 5. Locate first { or [ and extract to matching bracket
func extractJSON(text string) (string, string) {
	text = strings.TrimSpace(text)

	// Step 1 & 2: If starts with { or [, use as-is
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return text, "direct"
	}

	// Step 3: Check for ```json fence
	jsonFence := regexp.MustCompile("```json\\s*\\n([\\s\\S]*?)```")
	if matches := jsonFence.FindStringSubmatch(text); matches != nil {
		return strings.TrimSpace(matches[1]), "markdown_json_fence"
	}

	// Step 4: Try any markdown fence
	anyFence := regexp.MustCompile("```[^`]*?\\n([\\s\\S]*?)```")
	allMatches := anyFence.FindAllStringSubmatch(text, -1)
	for _, matches := range allMatches {
		content := strings.TrimSpace(matches[1])
		if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
			return content, "markdown_fence"
		}
	}

	// Step 5: Find first { or [ and extract to matching bracket
	openIdx := -1
	openChar := ""
	closeChar := ""

	for i, ch := range text {
		if ch == '{' {
			openIdx = i
			openChar = "{"
			closeChar = "}"
			break
		} else if ch == '[' {
			openIdx = i
			openChar = "["
			closeChar = "]"
			break
		}
	}

	if openIdx == -1 {
		return "", "none"
	}

	// Find matching closing bracket
	depth := 0
	inString := false
	escape := false

	for i := openIdx; i < len(text); i++ {
		ch := text[i]

		// Handle escape sequences in strings
		if escape {
			escape = false
			continue
		}
		if ch == '\\' {
			escape = true
			continue
		}

		// Track string boundaries
		if ch == '"' {
			inString = !inString
			continue
		}

		if !inString {
			if string(ch) == openChar {
				depth++
			} else if string(ch) == closeChar {
				depth--
				if depth == 0 {
					return text[openIdx : i+1], "bracket_matching"
				}
			}
		}
	}

	// Unmatched brackets
	return "", "none"
}

// redactSensitive redacts sensitive field values from text.
// Uses case-insensitive substring matching for field names.
func redactSensitive(text string) string {
	lowerText := strings.ToLower(text)

	// Check if any sensitive pattern appears in the text
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerText, pattern) {
			// Simple redaction: replace with [REDACTED]
			// In a more sophisticated version, we'd parse JSON and redact only values
			return "[REDACTED - contains sensitive data]"
		}
	}

	return text
}

// truncate limits text to maxLen characters
func truncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
