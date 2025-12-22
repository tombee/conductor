// Package format provides CLI output formatting with TTY detection.
package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/glamour"
)

const (
	// Maximum output sizes per NFR4
	maxJSONSize     = 10 * 1024 * 1024  // 10MB
	maxMarkdownSize = 5 * 1024 * 1024   // 5MB
	maxCodeSize     = 2 * 1024 * 1024   // 2MB
	maxNumberSize   = 1024              // 1KB
	maxStringSize   = 100 * 1024 * 1024 // 100MB
)

// ansiEscapeRegex matches ANSI escape sequences for sanitization.
var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// sanitizeANSI removes ANSI escape sequences from a string.
func sanitizeANSI(s string) string {
	return ansiEscapeRegex.ReplaceAllString(s, "")
}

// enforceSize checks if content exceeds the maximum size for its format.
func enforceSize(content string, format string, maxSize int) error {
	if len(content) > maxSize {
		return fmt.Errorf("output size (%d bytes) exceeds maximum for %s format (%d bytes)", len(content), format, maxSize)
	}
	return nil
}

// FormatMarkdown renders markdown with ANSI formatting if stdout is a TTY.
// Falls back to plain text if glamour fails or stdout is not a TTY.
func FormatMarkdown(content string, isTTY bool) (string, error) {
	// Enforce size limit
	if err := enforceSize(content, "markdown", maxMarkdownSize); err != nil {
		return "", err
	}

	// If not a TTY, return as-is
	if !isTTY {
		return content, nil
	}

	// Render with glamour
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		// Fallback to plain text if glamour fails
		return content, nil
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		// Fallback to plain text if rendering fails
		return content, nil
	}

	// Sanitize ANSI escape sequences for security
	return sanitizeANSI(rendered), nil
}

// FormatJSON pretty-prints JSON with 2-space indentation.
// Returns formatted JSON if valid, error otherwise.
func FormatJSON(content string, isTTY bool) (string, error) {
	// Enforce size limit
	if err := enforceSize(content, "json", maxJSONSize); err != nil {
		return "", err
	}

	// Parse JSON
	var obj interface{}
	if err := json.Unmarshal([]byte(content), &obj); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	// Pretty-print with 2-space indentation
	formatted, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}

	return string(formatted), nil
}

// FormatCode applies syntax highlighting if language is specified and stdout is a TTY.
// Falls back to plain code if language is unrecognized or stdout is not a TTY.
func FormatCode(content string, format string, isTTY bool) (string, error) {
	// Enforce size limit
	if err := enforceSize(content, "code", maxCodeSize); err != nil {
		return "", err
	}

	// If not a TTY, return as-is
	if !isTTY {
		return content, nil
	}

	// Extract language from format (e.g., "code:python" -> "python")
	var language string
	if strings.HasPrefix(strings.ToLower(format), "code:") {
		language = strings.TrimPrefix(strings.ToLower(format), "code:")
	}

	// If no language specified, return as-is
	if language == "" {
		return content, nil
	}

	// Apply syntax highlighting with chroma
	var buf bytes.Buffer
	err := quick.Highlight(&buf, content, language, "terminal256", "monokai")
	if err != nil {
		// If language is not recognized, return plain code (no error)
		return content, nil
	}

	// Sanitize ANSI escape sequences for security
	return sanitizeANSI(buf.String()), nil
}

// FormatNumber returns the number as-is.
func FormatNumber(content string, isTTY bool) (string, error) {
	// Enforce size limit
	if err := enforceSize(content, "number", maxNumberSize); err != nil {
		return "", err
	}

	return content, nil
}

// FormatString returns the string as-is.
func FormatString(content string, isTTY bool) (string, error) {
	// Enforce size limit
	if err := enforceSize(content, "string", maxStringSize); err != nil {
		return "", err
	}

	return content, nil
}

// Format formats output content based on its format type.
// Returns formatted content or error if formatting fails.
func Format(content string, format string, isTTY bool) (string, error) {
	if format == "" {
		format = "string"
	}

	// Normalize format to lowercase
	formatLower := strings.ToLower(format)

	// Handle code with language (e.g., "code:python")
	if strings.HasPrefix(formatLower, "code:") {
		return FormatCode(content, format, isTTY)
	}

	// Dispatch to specific formatter
	switch formatLower {
	case "string":
		return FormatString(content, isTTY)
	case "number":
		return FormatNumber(content, isTTY)
	case "markdown":
		return FormatMarkdown(content, isTTY)
	case "json":
		return FormatJSON(content, isTTY)
	case "code":
		return FormatCode(content, format, isTTY)
	default:
		return "", fmt.Errorf("unknown format: %s", format)
	}
}
