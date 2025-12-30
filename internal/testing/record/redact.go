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

package record

import (
	"encoding/json"
	"regexp"
	"strings"
)

// RedactionPattern defines a pattern for redacting sensitive data.
type RedactionPattern struct {
	Name        string
	Regex       *regexp.Regexp
	Replacement string
}

// StandardRedactionPatterns returns the default patterns for fixture recording.
// These patterns match the NFR5 requirements from the spec.
func StandardRedactionPatterns() []RedactionPattern {
	return []RedactionPattern{
		// API key patterns (case-sensitive prefix match)
		{
			Name:        "openai_key",
			Regex:       regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
			Replacement: "[REDACTED-OPENAI-KEY]",
		},
		{
			Name:        "stripe_key",
			Regex:       regexp.MustCompile(`sk_(live|test)_[a-zA-Z0-9]{24,}`),
			Replacement: "[REDACTED-STRIPE-KEY]",
		},
		{
			Name:        "github_token",
			Regex:       regexp.MustCompile(`ghp_[a-zA-Z0-9]{20,}`),
			Replacement: "[REDACTED-GITHUB-TOKEN]",
		},
		{
			Name:        "slack_token",
			Regex:       regexp.MustCompile(`xoxb-[a-zA-Z0-9\-]{20,}`),
			Replacement: "[REDACTED-SLACK-TOKEN]",
		},
		{
			Name:        "aws_access_key",
			Regex:       regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			Replacement: "[REDACTED-AWS-KEY]",
		},
		{
			Name:        "gitlab_token",
			Regex:       regexp.MustCompile(`glpat_[a-zA-Z0-9\-_]{20,}`),
			Replacement: "[REDACTED-GITLAB-TOKEN]",
		},
		{
			Name:        "auth0_token",
			Regex:       regexp.MustCompile(`auth0-[a-zA-Z0-9]{32,}`),
			Replacement: "[REDACTED-AUTH0-TOKEN]",
		},

		// JWT tokens
		{
			Name:        "jwt",
			Regex:       regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`),
			Replacement: "[REDACTED-JWT]",
		},

		// Generic Bearer tokens
		{
			Name:        "bearer_token",
			Regex:       regexp.MustCompile(`(?i)Bearer\s+[a-zA-Z0-9_\-\.]{20,}`),
			Replacement: "Bearer [REDACTED]",
		},

		// Private keys
		{
			Name:        "private_key",
			Regex:       regexp.MustCompile(`(?s)(-----BEGIN (RSA |EC |DSA )?PRIVATE KEY-----).*?(-----END (RSA |EC |DSA )?PRIVATE KEY-----)`),
			Replacement: "$1[REDACTED]$3",
		},

		// AWS secret access key pattern
		{
			Name:        "aws_secret",
			Regex:       regexp.MustCompile(`(?i)(aws_secret_access_key["\s:=]+)([a-zA-Z0-9/+=]{40})`),
			Replacement: "$1[REDACTED]",
		},
	}
}

// Redactor applies redaction patterns to sensitive data in fixtures.
type Redactor struct {
	patterns []RedactionPattern
}

// NewRedactor creates a new redactor with standard patterns.
func NewRedactor() *Redactor {
	return &Redactor{
		patterns: StandardRedactionPatterns(),
	}
}

// NewRedactorWithPatterns creates a redactor with custom patterns.
func NewRedactorWithPatterns(patterns []RedactionPattern) *Redactor {
	return &Redactor{
		patterns: patterns,
	}
}

// AddPattern adds a custom redaction pattern.
func (r *Redactor) AddPattern(pattern RedactionPattern) {
	r.patterns = append(r.patterns, pattern)
}

// RedactString applies all redaction patterns to a string.
func (r *Redactor) RedactString(s string) string {
	result := s
	for _, pattern := range r.patterns {
		result = pattern.Regex.ReplaceAllString(result, pattern.Replacement)
	}
	return result
}

// RedactMap recursively redacts values in a map based on key names and patterns.
// This handles JSON fields with case-insensitive matching.
func (r *Redactor) RedactMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range m {
		// Check if the key indicates sensitive data
		if r.shouldRedactKey(key) {
			result[key] = "[REDACTED]"
			continue
		}

		// Recursively process based on type
		switch v := value.(type) {
		case string:
			result[key] = r.RedactString(v)
		case map[string]interface{}:
			result[key] = r.RedactMap(v)
		case []interface{}:
			result[key] = r.redactArray(v)
		default:
			result[key] = value
		}
	}

	return result
}

// redactArray recursively redacts values in an array.
func (r *Redactor) redactArray(arr []interface{}) []interface{} {
	result := make([]interface{}, len(arr))

	for i, value := range arr {
		switch v := value.(type) {
		case string:
			result[i] = r.RedactString(v)
		case map[string]interface{}:
			result[i] = r.RedactMap(v)
		case []interface{}:
			result[i] = r.redactArray(v)
		default:
			result[i] = value
		}
	}

	return result
}

// shouldRedactKey checks if a key name indicates sensitive data.
// Uses case-insensitive matching per NFR5.
func (r *Redactor) shouldRedactKey(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitiveKeys := []string{
		"password",
		"secret",
		"token",
		"api_key",
		"apikey",
		"access_token",
		"refresh_token",
		"private_key",
		"privatekey",
		"aws_secret_access_key",
		"authorization",
	}

	for _, sensitive := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

// RedactHeaders redacts sensitive HTTP headers.
func (r *Redactor) RedactHeaders(headers map[string]string) map[string]string {
	result := make(map[string]string)

	for key, value := range headers {
		// Always redact authorization and API key headers
		lowerKey := strings.ToLower(key)
		if lowerKey == "authorization" || strings.Contains(lowerKey, "api") && strings.Contains(lowerKey, "key") {
			result[key] = "[REDACTED]"
			continue
		}

		// Apply pattern-based redaction to other headers
		result[key] = r.RedactString(value)
	}

	return result
}

// RedactJSON redacts sensitive data in a JSON string.
// It parses the JSON, applies redaction, and returns the redacted JSON.
func (r *Redactor) RedactJSON(jsonStr string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// If it's not valid JSON, just apply string redaction
		return r.RedactString(jsonStr), nil
	}

	// Apply redaction based on type
	var redacted interface{}
	switch v := data.(type) {
	case map[string]interface{}:
		redacted = r.RedactMap(v)
	case []interface{}:
		redacted = r.redactArray(v)
	default:
		// Primitive type, apply string redaction if it's a string
		if str, ok := v.(string); ok {
			return r.RedactString(str), nil
		}
		return jsonStr, nil
	}

	// Marshal back to JSON
	redactedJSON, err := json.Marshal(redacted)
	if err != nil {
		return "", err
	}

	return string(redactedJSON), nil
}

// RedactURL redacts sensitive data in URLs (e.g., tokens in query params).
func (r *Redactor) RedactURL(url string) string {
	// Apply pattern-based redaction to the entire URL
	// This will catch tokens in query parameters
	return r.RedactString(url)
}
