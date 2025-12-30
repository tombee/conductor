package polltrigger

import (
	"fmt"
	"regexp"
	"strings"
)

// Sensitive field patterns that should be stripped from trigger context.
// These patterns prevent credential leakage to workflows.
var sensitiveFieldPatterns = []string{
	"*token",
	"*secret",
	"*password",
	"*key",
	"*auth*",
	"credential*",
	"api_key",
	"app_key",
}

// Integration-specific sensitive fields to strip.
var integrationSensitiveFields = map[string][]string{
	"pagerduty": {
		"escalation_policy.escalation_rules",
		"conference_bridge",
	},
	"slack": {
		"bot_profile",
		"app_id",
	},
	"jira": {
		// Custom fields with type=password are handled dynamically
	},
	"datadog": {
		// No additional fields beyond common patterns
	},
}

// Input validation pattern: alphanumeric, underscore, hyphen only.
// This prevents injection attacks in query parameters.
var safeIdentifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Extended pattern allowing spaces and periods for Jira/Slack usernames.
var safeExtendedIdentifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-. ]+$`)

// ValidateIdentifier validates that a string matches the safe identifier pattern.
// This prevents injection attacks in integration query parameters.
func ValidateIdentifier(value string) error {
	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}

	if !safeIdentifierPattern.MatchString(value) {
		return fmt.Errorf("value %q contains invalid characters (only alphanumeric, underscore, and hyphen allowed)", value)
	}

	return nil
}

// ValidateExtendedIdentifier validates that a string matches the extended pattern.
// Allows spaces and periods for usernames and display names.
func ValidateExtendedIdentifier(value string) error {
	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}

	if !safeExtendedIdentifierPattern.MatchString(value) {
		return fmt.Errorf("value %q contains invalid characters (only alphanumeric, underscore, hyphen, space, and period allowed)", value)
	}

	return nil
}

// ValidateQueryParameters validates all string values in a query map.
// Returns an error if any value contains potentially dangerous characters.
func ValidateQueryParameters(query map[string]interface{}) error {
	for key, value := range query {
		switch v := value.(type) {
		case string:
			// Use extended validation for username-like fields
			if isUsernameField(key) {
				if err := ValidateExtendedIdentifier(v); err != nil {
					return fmt.Errorf("invalid value for %q: %w", key, err)
				}
			} else {
				if err := ValidateIdentifier(v); err != nil {
					return fmt.Errorf("invalid value for %q: %w", key, err)
				}
			}
		case []interface{}:
			// Validate each element in array
			for i, elem := range v {
				if str, ok := elem.(string); ok {
					if isUsernameField(key) {
						if err := ValidateExtendedIdentifier(str); err != nil {
							return fmt.Errorf("invalid value in %q[%d]: %w", key, i, err)
						}
					} else {
						if err := ValidateIdentifier(str); err != nil {
							return fmt.Errorf("invalid value in %q[%d]: %w", key, i, err)
						}
					}
				}
			}
		}
	}

	return nil
}

// isUsernameField returns true if the field name is typically a username or display name.
func isUsernameField(fieldName string) bool {
	lower := strings.ToLower(fieldName)
	return strings.Contains(lower, "user") ||
		strings.Contains(lower, "assignee") ||
		strings.Contains(lower, "mention") ||
		strings.Contains(lower, "name")
}

// StripSensitiveFields removes sensitive fields from an event map.
// This prevents credentials and secrets from being passed to workflows.
func StripSensitiveFields(event map[string]interface{}, integration string) map[string]interface{} {
	cleaned := make(map[string]interface{})

	for key, value := range event {
		// Check if field matches a sensitive pattern
		if isSensitiveField(key) {
			continue
		}

		// Check integration-specific sensitive fields
		if isIntegrationSensitiveField(key, integration) {
			continue
		}

		// Recursively clean nested maps
		if nestedMap, ok := value.(map[string]interface{}); ok {
			cleaned[key] = StripSensitiveFields(nestedMap, integration)
		} else if nestedArray, ok := value.([]interface{}); ok {
			cleaned[key] = stripSensitiveFromArray(nestedArray, integration)
		} else {
			cleaned[key] = value
		}
	}

	return cleaned
}

// isSensitiveField checks if a field name matches any sensitive pattern.
func isSensitiveField(fieldName string) bool {
	lower := strings.ToLower(fieldName)

	for _, pattern := range sensitiveFieldPatterns {
		if matchesPattern(lower, pattern) {
			return true
		}
	}

	return false
}

// isIntegrationSensitiveField checks if a field is sensitive for a specific integration.
func isIntegrationSensitiveField(fieldName, integration string) bool {
	fields, ok := integrationSensitiveFields[integration]
	if !ok {
		return false
	}

	for _, sensitiveField := range fields {
		if strings.EqualFold(fieldName, sensitiveField) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a field name matches a wildcard pattern.
func matchesPattern(fieldName, pattern string) bool {
	// Handle * wildcard prefix
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		// *auth* - contains
		substr := strings.Trim(pattern, "*")
		return strings.Contains(fieldName, substr)
	} else if strings.HasPrefix(pattern, "*") {
		// *token - ends with
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(fieldName, suffix)
	} else if strings.HasSuffix(pattern, "*") {
		// credential* - starts with
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(fieldName, prefix)
	}

	// Exact match
	return fieldName == pattern
}

// stripSensitiveFromArray recursively strips sensitive fields from array elements.
func stripSensitiveFromArray(arr []interface{}, integration string) []interface{} {
	cleaned := make([]interface{}, 0, len(arr))

	for _, item := range arr {
		if itemMap, ok := item.(map[string]interface{}); ok {
			cleaned = append(cleaned, StripSensitiveFields(itemMap, integration))
		} else if itemArr, ok := item.([]interface{}); ok {
			cleaned = append(cleaned, stripSensitiveFromArray(itemArr, integration))
		} else {
			cleaned = append(cleaned, item)
		}
	}

	return cleaned
}

// SanitizeErrorMessage removes sensitive information from error messages.
// This prevents credentials from leaking in logs and error responses.
func SanitizeErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	// Patterns to redact
	redactPatterns := []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`Bearer [a-zA-Z0-9_-]+`), "Bearer [REDACTED]"},
		{regexp.MustCompile(`Token token=[a-zA-Z0-9_-]+`), "Token token=[REDACTED]"},
		{regexp.MustCompile(`Basic [a-zA-Z0-9+/=]+`), "Basic [REDACTED]"},
		{regexp.MustCompile(`DD-API-KEY: [a-zA-Z0-9]+`), "DD-API-KEY: [REDACTED]"},
		{regexp.MustCompile(`DD-APPLICATION-KEY: [a-zA-Z0-9]+`), "DD-APPLICATION-KEY: [REDACTED]"},
		{regexp.MustCompile(`xoxb-[a-zA-Z0-9-]+`), "[REDACTED-SLACK-TOKEN]"},
		{regexp.MustCompile(`xoxp-[a-zA-Z0-9-]+`), "[REDACTED-SLACK-TOKEN]"},
	}

	sanitized := msg
	for _, p := range redactPatterns {
		sanitized = p.pattern.ReplaceAllString(sanitized, p.replacement)
	}

	return sanitized
}

// ValidateIntegrationConfig validates configuration values for an integration.
// This ensures required fields are present and properly formatted.
func ValidateIntegrationConfig(integration string, config map[string]string) error {
	switch integration {
	case "pagerduty":
		if config["api_token"] == "" {
			return fmt.Errorf("pagerduty integration requires api_token")
		}
	case "slack":
		token := config["bot_token"]
		if token == "" {
			return fmt.Errorf("slack integration requires bot_token")
		}
		if !strings.HasPrefix(token, "xoxb-") {
			return fmt.Errorf("slack integration requires bot token (xoxb-), user tokens are not supported")
		}
	case "jira":
		if config["email"] == "" {
			return fmt.Errorf("jira integration requires email")
		}
		if config["api_token"] == "" {
			return fmt.Errorf("jira integration requires api_token")
		}
		if config["base_url"] == "" {
			return fmt.Errorf("jira integration requires base_url")
		}
	case "datadog":
		if config["api_key"] == "" {
			return fmt.Errorf("datadog integration requires api_key")
		}
		if config["app_key"] == "" {
			return fmt.Errorf("datadog integration requires app_key")
		}
		// site is optional, defaults to datadoghq.com
	default:
		return fmt.Errorf("unknown integration: %s", integration)
	}

	return nil
}
