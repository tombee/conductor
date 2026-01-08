package expression

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Template pattern matches {{...}} expressions
var templatePattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// PreprocessTemplate resolves Go template-style expressions ({{.steps.id.field}})
// by replacing them with expr-lang compatible literals before evaluation.
//
// This function performs a single-pass resolution for security:
// - Finds all {{...}} patterns
// - Resolves each path from the context
// - Replaces with appropriate literal (quoted strings, numbers, booleans, etc.)
//
// Example:
//
//	PreprocessTemplate(`{{.steps.check.status}} == "success"`, ctx)
//	=> `"success" == "success"`
//
// Returns the processed expression or an error if template syntax is invalid
// or if a referenced path cannot be resolved.
func PreprocessTemplate(expression string, ctx map[string]interface{}) (string, error) {
	if expression == "" {
		return expression, nil
	}

	var lastErr error
	result := templatePattern.ReplaceAllStringFunc(expression, func(match string) string {
		// Extract path from {{...}}
		path := strings.TrimSpace(match[2 : len(match)-2]) // Remove {{ and }}

		// Remove leading dot if present (.steps.id => steps.id)
		path = strings.TrimPrefix(path, ".")

		// Resolve the value from context
		value, err := resolvePath(path, ctx)
		if err != nil {
			lastErr = err
			return match // Keep original on error
		}

		// Convert value to expr-lang literal
		literal := valueToLiteral(value)
		return literal
	})

	if lastErr != nil {
		return "", fmt.Errorf("template resolution failed: %w", lastErr)
	}

	return result, nil
}

// resolvePath resolves a dot-separated path in the context.
// Example: "steps.check.status" => ctx["steps"]["check"]["status"]
func resolvePath(path string, ctx map[string]interface{}) (interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}

	parts := strings.Split(path, ".")
	var current interface{} = ctx

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("invalid path: empty segment at position %d", i)
		}

		// Navigate into the current value
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path not found: %s (missing key '%s')", path, part)
			}
			current = val
		default:
			return nil, fmt.Errorf("path not found: %s (cannot index into %T at '%s')", path, current, part)
		}
	}

	return current, nil
}

// valueToLiteral converts a Go value to an expr-lang literal string.
// - Strings are quoted and escaped
// - Numbers are rendered as-is
// - Booleans are rendered as true/false
// - nil becomes nil
// - Other types use string representation (best effort)
func valueToLiteral(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return "nil"
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case string:
		// Escape quotes and backslashes in string
		escaped := strings.ReplaceAll(v, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		return fmt.Sprintf(`"%s"`, escaped)
	default:
		// For arrays, maps, and other types, convert to string and quote
		// This is a fallback - most cases should be primitives
		str := fmt.Sprintf("%v", v)
		escaped := strings.ReplaceAll(str, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		return fmt.Sprintf(`"%s"`, escaped)
	}
}
