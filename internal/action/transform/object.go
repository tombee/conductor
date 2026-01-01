package transform

import (
	"context"
	"fmt"
)

// pick operation - selects only specified keys from an object.
func (c *TransformAction) pick(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "pick",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with object to pick keys from",
		}
	}

	// Check if data is nil/null
	if data == nil {
		return nil, &OperationError{
			Operation:  "pick",
			Message:    "cannot pick from null or undefined value",
			ErrorType:  ErrorTypeEmptyInput,
			Suggestion: "Provide a valid object to pick keys from",
		}
	}

	// Verify input is an object
	obj, ok := data.(map[string]interface{})
	if !ok {
		return nil, &OperationError{
			Operation:  "pick",
			Message:    fmt.Sprintf("data must be an object, got %T", data),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Use transform.pick only with object inputs",
		}
	}

	// Get keys parameter
	keysParam, ok := inputs["keys"]
	if !ok {
		return nil, &OperationError{
			Operation:  "pick",
			Message:    "missing required parameter: keys",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide keys parameter with array of key names to pick",
		}
	}

	// Validate keys is an array
	keysArray, ok := keysParam.([]interface{})
	if !ok {
		return nil, &OperationError{
			Operation:  "pick",
			Message:    fmt.Sprintf("keys must be an array, got %T", keysParam),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Provide keys as an array of strings",
		}
	}

	// Convert keys to strings
	keys := make([]string, 0, len(keysArray))
	for i, k := range keysArray {
		keyStr, ok := k.(string)
		if !ok {
			return nil, &OperationError{
				Operation:  "pick",
				Message:    fmt.Sprintf("key at index %d must be a string, got %T", i, k),
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "All keys must be strings",
			}
		}
		keys = append(keys, keyStr)
	}

	// Build result with only specified keys
	result := make(map[string]interface{})
	pickedCount := 0
	for _, key := range keys {
		if value, exists := obj[key]; exists {
			result[key] = value
			pickedCount++
		}
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"keys_requested": len(keys),
			"keys_found":     pickedCount,
		},
	}, nil
}

// omit operation - removes specified keys from an object.
func (c *TransformAction) omit(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "omit",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with object to omit keys from",
		}
	}

	// Check if data is nil/null
	if data == nil {
		return nil, &OperationError{
			Operation:  "omit",
			Message:    "cannot omit from null or undefined value",
			ErrorType:  ErrorTypeEmptyInput,
			Suggestion: "Provide a valid object to omit keys from",
		}
	}

	// Verify input is an object
	obj, ok := data.(map[string]interface{})
	if !ok {
		return nil, &OperationError{
			Operation:  "omit",
			Message:    fmt.Sprintf("data must be an object, got %T", data),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Use transform.omit only with object inputs",
		}
	}

	// Get keys parameter
	keysParam, ok := inputs["keys"]
	if !ok {
		return nil, &OperationError{
			Operation:  "omit",
			Message:    "missing required parameter: keys",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide keys parameter with array of key names to omit",
		}
	}

	// Validate keys is an array
	keysArray, ok := keysParam.([]interface{})
	if !ok {
		return nil, &OperationError{
			Operation:  "omit",
			Message:    fmt.Sprintf("keys must be an array, got %T", keysParam),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Provide keys as an array of strings",
		}
	}

	// Convert keys to a set for efficient lookup
	keysToOmit := make(map[string]bool, len(keysArray))
	for i, k := range keysArray {
		keyStr, ok := k.(string)
		if !ok {
			return nil, &OperationError{
				Operation:  "omit",
				Message:    fmt.Sprintf("key at index %d must be a string, got %T", i, k),
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "All keys must be strings",
			}
		}
		keysToOmit[keyStr] = true
	}

	// Build result with all keys except those to omit
	result := make(map[string]interface{})
	omittedCount := 0
	for key, value := range obj {
		if keysToOmit[key] {
			omittedCount++
		} else {
			result[key] = value
		}
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"keys_to_omit":  len(keysToOmit),
			"keys_omitted":  omittedCount,
			"keys_retained": len(result),
		},
	}, nil
}
