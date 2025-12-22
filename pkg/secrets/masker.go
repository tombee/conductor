// Package secrets provides utilities for detecting and masking sensitive values.
package secrets

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Masker detects and masks sensitive values in strings and data structures.
// It uses pattern matching to identify environment variables that likely contain secrets.
type Masker struct {
	// patterns are suffixes that indicate a secret (e.g., _TOKEN, _SECRET)
	patterns []string

	// secrets is a map of known secret values to mask
	secrets map[string]bool
}

// NewMasker creates a new secret masker with default patterns.
func NewMasker() *Masker {
	return &Masker{
		patterns: []string{
			"_TOKEN",
			"_SECRET",
			"_KEY",
			"_PASSWORD",
			"_PASS",
			"_PWD",
		},
		secrets: make(map[string]bool),
	}
}

// AddSecret registers a value to be masked.
// This is useful for masking specific values that don't match pattern heuristics.
func (m *Masker) AddSecret(value string) {
	if value != "" {
		m.secrets[value] = true
	}
}

// AddSecretsFromEnv scans environment variables and adds values for keys matching secret patterns.
func (m *Masker) AddSecretsFromEnv(env map[string]string) {
	for key, value := range env {
		if m.isSecretKey(key) && value != "" {
			m.secrets[value] = true
		}
	}
}

// isSecretKey checks if an environment variable key matches a secret pattern.
func (m *Masker) isSecretKey(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, pattern := range m.patterns {
		if strings.HasSuffix(upperKey, pattern) {
			return true
		}
	}
	return false
}

// Mask replaces all known secrets in a string with "***".
func (m *Masker) Mask(s string) string {
	result := s
	for secret := range m.secrets {
		if secret != "" && strings.Contains(result, secret) {
			result = strings.ReplaceAll(result, secret, "***")
		}
	}
	return result
}

// MaskMap recursively masks secrets in a map structure.
// Returns a new map with secrets replaced.
func (m *Masker) MaskMap(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		result[k] = m.maskValue(v)
	}
	return result
}

// maskValue masks secrets in any value type.
func (m *Masker) maskValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		return m.Mask(val)
	case map[string]interface{}:
		return m.MaskMap(val)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = m.maskValue(item)
		}
		return result
	case json.Number:
		return val
	case bool:
		return val
	case nil:
		return nil
	default:
		// For unknown types, convert to string and mask
		return m.Mask(fmt.Sprintf("%v", val))
	}
}

// MaskJSON masks secrets in a JSON string.
// Returns the masked JSON or the original string if parsing fails.
func (m *Masker) MaskJSON(jsonStr string) string {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// Not valid JSON, just mask the string
		return m.Mask(jsonStr)
	}

	masked := m.maskValue(data)
	result, err := json.Marshal(masked)
	if err != nil {
		// Fallback to string masking
		return m.Mask(jsonStr)
	}

	return string(result)
}
