// Package schema provides JSON Schema validation for workflow output schemas.
package schema

import (
	"encoding/json"
	"fmt"
)

// Validator validates data against a JSON Schema.
type Validator interface {
	// Validate checks if data conforms to the schema
	Validate(schema map[string]interface{}, data interface{}) error
}

// DefaultValidator implements the Validator interface with support for
// a subset of JSON Schema Draft 7 keywords.
type DefaultValidator struct{}

// NewValidator creates a new schema validator.
func NewValidator() Validator {
	return &DefaultValidator{}
}

// Validate validates data against a JSON Schema.
// Supports: type, properties, required, enum, items
func (v *DefaultValidator) Validate(schema map[string]interface{}, data interface{}) error {
	return v.validate(schema, data, "$")
}

// validate is the recursive validation function with path tracking.
func (v *DefaultValidator) validate(schema map[string]interface{}, data interface{}, path string) error {
	// Check type constraint
	if schemaType, ok := schema["type"].(string); ok {
		if err := v.validateType(schemaType, data, path); err != nil {
			return err
		}

		// Type-specific validation
		switch schemaType {
		case "object":
			if err := v.validateObject(schema, data, path); err != nil {
				return err
			}
		case "array":
			if err := v.validateArray(schema, data, path); err != nil {
				return err
			}
		case "string":
			if err := v.validateString(schema, data, path); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateType checks if data matches the expected type.
func (v *DefaultValidator) validateType(schemaType string, data interface{}, path string) error {
	switch schemaType {
	case "object":
		if _, ok := data.(map[string]interface{}); !ok {
			return NewValidationError(path, "type", fmt.Sprintf("expected object, got %T", data))
		}
	case "array":
		if _, ok := data.([]interface{}); !ok {
			return NewValidationError(path, "type", fmt.Sprintf("expected array, got %T", data))
		}
	case "string":
		if _, ok := data.(string); !ok {
			return NewValidationError(path, "type", fmt.Sprintf("expected string, got %T", data))
		}
	case "number":
		switch data.(type) {
		case float64, int, int64, float32:
			// Valid number types
		default:
			return NewValidationError(path, "type", fmt.Sprintf("expected number, got %T", data))
		}
	case "integer":
		switch v := data.(type) {
		case float64:
			// JSON numbers are float64, check if it's a whole number
			if v != float64(int64(v)) {
				return NewValidationError(path, "type", fmt.Sprintf("expected integer, got %v", v))
			}
		case int, int64:
			// Valid integer types
		default:
			return NewValidationError(path, "type", fmt.Sprintf("expected integer, got %T", data))
		}
	case "boolean":
		if _, ok := data.(bool); !ok {
			return NewValidationError(path, "type", fmt.Sprintf("expected boolean, got %T", data))
		}
	default:
		return fmt.Errorf("unsupported schema type: %s", schemaType)
	}
	return nil
}

// validateObject validates object properties and required fields.
func (v *DefaultValidator) validateObject(schema map[string]interface{}, data interface{}, path string) error {
	obj, ok := data.(map[string]interface{})
	if !ok {
		return NewValidationError(path, "type", fmt.Sprintf("expected object, got %T", data))
	}

	// Validate required fields
	if required, ok := schema["required"].([]interface{}); ok {
		for _, reqField := range required {
			fieldName, ok := reqField.(string)
			if !ok {
				continue
			}
			if _, exists := obj[fieldName]; !exists {
				return NewValidationError(path, "required", fmt.Sprintf("missing required field: %s", fieldName))
			}
		}
	}

	// Validate properties
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for fieldName, fieldValue := range obj {
			// Get schema for this property
			if propSchema, ok := properties[fieldName].(map[string]interface{}); ok {
				fieldPath := fmt.Sprintf("%s.%s", path, fieldName)
				if err := v.validate(propSchema, fieldValue, fieldPath); err != nil {
					return err
				}
			}
			// Note: extra fields not in schema are allowed (silently ignored)
		}
	}

	return nil
}

// validateArray validates array items.
func (v *DefaultValidator) validateArray(schema map[string]interface{}, data interface{}, path string) error {
	arr, ok := data.([]interface{})
	if !ok {
		return NewValidationError(path, "type", fmt.Sprintf("expected array, got %T", data))
	}

	// Validate items schema
	if items, ok := schema["items"].(map[string]interface{}); ok {
		for i, item := range arr {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if err := v.validate(items, item, itemPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateString validates string constraints (enum).
func (v *DefaultValidator) validateString(schema map[string]interface{}, data interface{}, path string) error {
	str, ok := data.(string)
	if !ok {
		return NewValidationError(path, "type", fmt.Sprintf("expected string, got %T", data))
	}

	// Validate enum constraint
	if enum, ok := schema["enum"].([]interface{}); ok {
		valid := false
		for _, allowedValue := range enum {
			if allowedStr, ok := allowedValue.(string); ok && allowedStr == str {
				valid = true
				break
			}
		}
		if !valid {
			enumJSON, _ := json.Marshal(enum)
			return NewValidationError(path, "enum", fmt.Sprintf("value %q not in allowed values: %s", str, enumJSON))
		}
	}

	return nil
}
