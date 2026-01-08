// Package tools provides utilities for tool execution and output processing.
package tools

import (
	"regexp"
	"sync"
)

// Redactor detects and redacts sensitive data patterns in strings.
// It uses compiled regex patterns to identify common sensitive values like API keys,
// tokens, passwords, and cloud provider credentials.
type Redactor struct {
	patterns []*redactionPattern
	mu       sync.RWMutex // Protects patterns for thread-safe usage
}

// redactionPattern represents a compiled pattern with its replacement string.
type redactionPattern struct {
	regex       *regexp.Regexp
	replacement string
}

// NewRedactor creates a new redactor with default patterns for common sensitive data.
// Patterns include:
// - AWS access keys (AKIA...)
// - AWS secret keys in configuration
// - Bearer tokens
// - API keys in various formats
// - Passwords in URLs
// - Database connection strings with credentials
func NewRedactor() *Redactor {
	r := &Redactor{
		patterns: make([]*redactionPattern, 0),
	}

	// AWS Access Key IDs (start with AKIA and are 20 characters)
	r.addPattern(`AKIA[A-Z0-9]{16}`, "[REDACTED]")

	// AWS Secret Access Keys in configuration contexts
	// Matches patterns like: aws_secret_access_key = "base64string"
	// or secret_key: "base64string" (40 character base64)
	r.addPattern(`(?i)(aws[_-]?secret[_-]?access[_-]?key|secret[_-]?key|aws[_-]?secret)\s*[=:]\s*['\"]?([A-Za-z0-9/+=]{40})['\"]?`, "$1=[REDACTED]")

	// Bearer tokens in Authorization headers
	// Only match if followed by token-like characters (at least 10 chars)
	r.addPattern(`(?i)Bearer\s+([a-zA-Z0-9_\-\.]{10,})`, "Bearer [REDACTED]")

	// API keys in various formats
	// Matches: api_key=xxx, apiKey: xxx, api-key="xxx", etc.
	r.addPattern(`(?i)(api[_-]?key|apikey)\s*[=:]\s*['\"]?([a-zA-Z0-9_\-]{20,})['\"]?`, "$1=[REDACTED]")

	// Generic token patterns
	r.addPattern(`(?i)(token|access[_-]?token|auth[_-]?token)\s*[=:]\s*['\"]?([a-zA-Z0-9_\-\.]{20,})['\"]?`, "$1=[REDACTED]")

	// Passwords in URLs (://user:password@host)
	// Note: @ characters in passwords should be URL-encoded as %40
	r.addPattern(`://([^:@\s]+):([^@\s]+)@`, "://$1:[REDACTED]@")

	// Database connection strings with passwords
	// Handle both quoted and unquoted values, with minimum length of 3
	r.addPattern(`(?i)(password|pwd|pass)\s*=\s*'([^']{3,})'`, "$1=[REDACTED]")
	r.addPattern(`(?i)(password|pwd|pass)\s*=\s*"([^"]{3,})"`, "$1=[REDACTED]")
	r.addPattern(`(?i)(password|pwd|pass)\s*=\s*([^;'\"\s]{3,})`, "$1=[REDACTED]")

	// Generic secret patterns (for environment variables or config)
	r.addPattern(`(?i)(secret|private[_-]?key)\s*[=:]\s*['\"]?([a-zA-Z0-9_\-/+=]{20,})['\"]?`, "$1=[REDACTED]")

	return r
}

// addPattern compiles and adds a new redaction pattern.
func (r *Redactor) addPattern(pattern, replacement string) {
	regex := regexp.MustCompile(pattern)
	r.patterns = append(r.patterns, &redactionPattern{
		regex:       regex,
		replacement: replacement,
	})
}

// Redact scans the input string and replaces all matches of sensitive patterns with [REDACTED].
// It applies all patterns in sequence and returns the redacted string.
// This method is thread-safe and can be called concurrently.
func (r *Redactor) Redact(s string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := s
	for _, p := range r.patterns {
		result = p.regex.ReplaceAllString(result, p.replacement)
	}
	return result
}
