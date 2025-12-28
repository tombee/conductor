package operation

import (
	"fmt"
	"regexp"
	"strings"
)

// Validator provides input validation for connectors.
type Validator struct{}

// NewValidator creates a new validator instance.
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateLokiLabels validates Loki label names against the required pattern.
// Label names must match: [a-zA-Z_][a-zA-Z0-9_]*
// Reserved labels starting with __ are rejected.
func (v *Validator) ValidateLokiLabels(labels map[string]interface{}) error {
	if labels == nil {
		return nil
	}

	labelNamePattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

	for name, value := range labels {
		// Check for reserved labels
		if strings.HasPrefix(name, "__") {
			return &Error{
				Type:        ErrorTypeValidation,
				Message:     fmt.Sprintf("loki label name cannot start with '__': %s", name),
				SuggestText: "remove the leading underscores from the label name",
			}
		}

		// Check label name format
		if !labelNamePattern.MatchString(name) {
			return &Error{
				Type:        ErrorTypeValidation,
				Message:     fmt.Sprintf("invalid loki label name: %s (must match [a-zA-Z_][a-zA-Z0-9_]*)", name),
				SuggestText: "use only alphanumeric characters and underscores, starting with a letter or underscore",
			}
		}

		// Check for empty values
		if str, ok := value.(string); ok && str == "" {
			return &Error{
				Type:        ErrorTypeValidation,
				Message:     fmt.Sprintf("loki label value cannot be empty: %s", name),
				SuggestText: "provide a non-empty value for the label",
			}
		}
	}

	return nil
}

// ValidDatadogSites lists all valid Datadog site configurations.
var ValidDatadogSites = map[string]bool{
	"datadoghq.com":       true, // US1
	"us3.datadoghq.com":   true, // US3
	"us5.datadoghq.com":   true, // US5
	"datadoghq.eu":        true, // EU
	"ap1.datadoghq.com":   true, // AP1
	"ddog-gov.com":        true, // US1-FED (Government)
	"us1.datadoghq.com":   true, // US1 (alternative)
}

// ValidateDatadogSite validates a Datadog site configuration.
func (v *Validator) ValidateDatadogSite(site string) error {
	if site == "" {
		// Empty site defaults to datadoghq.com, which is valid
		return nil
	}

	if !ValidDatadogSites[site] {
		return &Error{
			Type:        ErrorTypeValidation,
			Message:     fmt.Sprintf("invalid datadog site: %s", site),
			SuggestText: "use one of: datadoghq.com, us3.datadoghq.com, us5.datadoghq.com, datadoghq.eu, ap1.datadoghq.com, ddog-gov.com",
		}
	}

	return nil
}

// ValidElasticsearchIndexChars defines valid characters for index names.
// Elasticsearch index names must be lowercase and cannot contain: \, /, *, ?, ", <, >, |, ` ` (space), ,, #
var elasticsearchIndexPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_\-+.]*$`)

// ValidateElasticsearchIndex validates an Elasticsearch index name.
func (v *Validator) ValidateElasticsearchIndex(index string) error {
	if index == "" {
		return &Error{
			Type:        ErrorTypeValidation,
			Message:     "elasticsearch index name cannot be empty",
			SuggestText: "provide a valid index name",
		}
	}

	// Check for invalid characters
	invalidChars := []string{"\\", "/", "*", "?", "\"", "<", ">", "|", " ", ",", "#"}
	for _, char := range invalidChars {
		if strings.Contains(index, char) {
			return &Error{
				Type:        ErrorTypeValidation,
				Message:     fmt.Sprintf("elasticsearch index name contains invalid character: %s", char),
				SuggestText: "remove invalid characters (\\, /, *, ?, \", <, >, |, space, comma, #)",
			}
		}
	}

	// Check if starts with uppercase (must be lowercase)
	if index != strings.ToLower(index) {
		return &Error{
			Type:        ErrorTypeValidation,
			Message:     fmt.Sprintf("elasticsearch index name must be lowercase: %s", index),
			SuggestText: "convert the index name to lowercase",
		}
	}

	// Check for reserved names
	if index == "." || index == ".." {
		return &Error{
			Type:        ErrorTypeValidation,
			Message:     "elasticsearch index name cannot be '.' or '..'",
			SuggestText: "use a different index name",
		}
	}

	// Check if starts with - or _
	if strings.HasPrefix(index, "-") || strings.HasPrefix(index, "_") || strings.HasPrefix(index, "+") {
		return &Error{
			Type:        ErrorTypeValidation,
			Message:     fmt.Sprintf("elasticsearch index name cannot start with -, _, or +: %s", index),
			SuggestText: "index names must start with a letter or number",
		}
	}

	// Check length (max 255 bytes)
	if len(index) > 255 {
		return &Error{
			Type:        ErrorTypeValidation,
			Message:     fmt.Sprintf("elasticsearch index name too long: %d bytes (max 255)", len(index)),
			SuggestText: "shorten the index name to 255 characters or less",
		}
	}

	return nil
}

// ValidateConnectorInputs validates inputs for a specific connector operation.
// This delegates to connector-specific validation based on the connector name.
func (v *Validator) ValidateConnectorInputs(connectorName string, operation string, inputs map[string]interface{}) error {
	switch connectorName {
	case "loki":
		return v.validateLokiInputs(operation, inputs)
	case "datadog":
		return v.validateDatadogInputs(operation, inputs)
	case "elasticsearch":
		return v.validateElasticsearchInputs(operation, inputs)
	case "cloudwatch":
		return v.validateCloudWatchInputs(operation, inputs)
	default:
		// No validation for other connectors
		return nil
	}
}

// validateLokiInputs validates Loki-specific inputs.
func (v *Validator) validateLokiInputs(operation string, inputs map[string]interface{}) error {
	if operation != "push" {
		return nil
	}

	// Validate labels
	if labels, ok := inputs["labels"].(map[string]interface{}); ok {
		if err := v.ValidateLokiLabels(labels); err != nil {
			return err
		}
	}

	return nil
}

// validateDatadogInputs validates Datadog-specific inputs.
func (v *Validator) validateDatadogInputs(operation string, inputs map[string]interface{}) error {
	// Site validation is done at connector config level, not per-operation
	// However, we can validate tags format here
	if tags, ok := inputs["tags"].([]interface{}); ok {
		for i, tag := range tags {
			tagStr, ok := tag.(string)
			if !ok {
				return &Error{
					Type:    ErrorTypeValidation,
					Message: fmt.Sprintf("datadog tag at index %d is not a string", i),
				}
			}

			// Tags should be in key:value format (warning, not error)
			if !strings.Contains(tagStr, ":") {
				// This is valid but not recommended - don't error
			}
		}
	}

	return nil
}

// validateElasticsearchInputs validates Elasticsearch-specific inputs.
func (v *Validator) validateElasticsearchInputs(operation string, inputs map[string]interface{}) error {
	// Validate index name if present
	if index, ok := inputs["index"].(string); ok && index != "" {
		// Skip validation for date math expressions (e.g., <logs-{now/d}>)
		if !strings.HasPrefix(index, "<") || !strings.HasSuffix(index, ">") {
			if err := v.ValidateElasticsearchIndex(index); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateCloudWatchInputs validates CloudWatch-specific inputs.
func (v *Validator) validateCloudWatchInputs(operation string, inputs map[string]interface{}) error {
	// CloudWatch unit validation is already done in the connector
	// No additional validation needed here
	return nil
}
