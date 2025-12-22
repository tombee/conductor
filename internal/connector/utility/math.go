package utility

import (
	"context"
	"fmt"
	"math"
)

// mathClamp constrains a value to a range [min, max].
func (c *UtilityConnector) mathClamp(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	value, err := getFloat64(inputs, "value")
	if err != nil {
		return nil, &OperationError{
			Operation:  "math_clamp",
			Message:    "invalid 'value' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'value' as a number",
		}
	}

	min, err := getFloat64(inputs, "min")
	if err != nil {
		return nil, &OperationError{
			Operation:  "math_clamp",
			Message:    "invalid 'min' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'min' as a number",
		}
	}

	max, err := getFloat64(inputs, "max")
	if err != nil {
		return nil, &OperationError{
			Operation:  "math_clamp",
			Message:    "invalid 'max' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'max' as a number",
		}
	}

	if min > max {
		return nil, &OperationError{
			Operation:  "math_clamp",
			Message:    "min must be <= max",
			ErrorType:  ErrorTypeRange,
			Suggestion: fmt.Sprintf("Swap values: min=%.2f, max=%.2f", max, min),
		}
	}

	// Handle NaN inputs
	if math.IsNaN(value) || math.IsNaN(min) || math.IsNaN(max) {
		return nil, &OperationError{
			Operation:  "math_clamp",
			Message:    "NaN is not a valid input",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Ensure all inputs are valid numbers, not NaN",
		}
	}

	result := value
	if result < min {
		result = min
	}
	if result > max {
		result = max
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"operation":      "math_clamp",
			"original_value": value,
			"min":            min,
			"max":            max,
			"was_clamped":    value != result,
		},
	}, nil
}

// mathRound rounds a value to the specified number of decimal places.
func (c *UtilityConnector) mathRound(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	value, err := getFloat64(inputs, "value")
	if err != nil {
		return nil, &OperationError{
			Operation:  "math_round",
			Message:    "invalid 'value' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'value' as a number",
		}
	}

	// Handle NaN and Inf
	if math.IsNaN(value) {
		return nil, &OperationError{
			Operation:  "math_round",
			Message:    "NaN is not a valid input",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Ensure value is a valid number, not NaN",
		}
	}
	if math.IsInf(value, 0) {
		return nil, &OperationError{
			Operation:  "math_round",
			Message:    "Infinity is not a valid input",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Ensure value is a finite number",
		}
	}

	// Get optional decimals parameter (default 0)
	decimals := 0
	if _, ok := inputs["decimals"]; ok {
		d, err := getInt(inputs, "decimals")
		if err != nil {
			return nil, &OperationError{
				Operation:  "math_round",
				Message:    "invalid 'decimals' parameter",
				ErrorType:  ErrorTypeValidation,
				Cause:      err,
				Suggestion: "Provide 'decimals' as a non-negative integer",
			}
		}
		decimals = d
	}

	if decimals < 0 {
		return nil, &OperationError{
			Operation:  "math_round",
			Message:    "decimals must be non-negative",
			ErrorType:  ErrorTypeRange,
			Suggestion: "Provide 'decimals' as 0 or a positive integer",
		}
	}

	if decimals > 15 {
		return nil, &OperationError{
			Operation:  "math_round",
			Message:    "decimals exceeds maximum precision (15)",
			ErrorType:  ErrorTypeRange,
			Suggestion: "Reduce 'decimals' to at most 15 for floating-point precision",
		}
	}

	// Round to specified decimal places
	multiplier := math.Pow(10, float64(decimals))
	result := math.Round(value*multiplier) / multiplier

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"operation":      "math_round",
			"original_value": value,
			"decimals":       decimals,
		},
	}, nil
}

// mathMin returns the minimum of the provided values.
func (c *UtilityConnector) mathMin(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	values, err := getNumberArray(inputs, "values")
	if err != nil {
		return nil, &OperationError{
			Operation:  "math_min",
			Message:    "invalid 'values' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'values' as an array of numbers",
		}
	}

	if len(values) == 0 {
		return nil, &OperationError{
			Operation:  "math_min",
			Message:    "values cannot be empty",
			ErrorType:  ErrorTypeEmpty,
			Suggestion: "Provide at least one value",
		}
	}

	if len(values) > c.config.MaxArraySize {
		return nil, &OperationError{
			Operation:  "math_min",
			Message:    fmt.Sprintf("values array exceeds maximum size of %d", c.config.MaxArraySize),
			ErrorType:  ErrorTypeValidation,
			Suggestion: fmt.Sprintf("Reduce array size to at most %d items", c.config.MaxArraySize),
		}
	}

	result := values[0]
	for _, v := range values[1:] {
		if math.IsNaN(v) {
			return nil, &OperationError{
				Operation:  "math_min",
				Message:    "values array contains NaN",
				ErrorType:  ErrorTypeValidation,
				Suggestion: "Ensure all values are valid numbers, not NaN",
			}
		}
		if v < result {
			result = v
		}
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"operation":    "math_min",
			"values_count": len(values),
		},
	}, nil
}

// mathMax returns the maximum of the provided values.
func (c *UtilityConnector) mathMax(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	values, err := getNumberArray(inputs, "values")
	if err != nil {
		return nil, &OperationError{
			Operation:  "math_max",
			Message:    "invalid 'values' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'values' as an array of numbers",
		}
	}

	if len(values) == 0 {
		return nil, &OperationError{
			Operation:  "math_max",
			Message:    "values cannot be empty",
			ErrorType:  ErrorTypeEmpty,
			Suggestion: "Provide at least one value",
		}
	}

	if len(values) > c.config.MaxArraySize {
		return nil, &OperationError{
			Operation:  "math_max",
			Message:    fmt.Sprintf("values array exceeds maximum size of %d", c.config.MaxArraySize),
			ErrorType:  ErrorTypeValidation,
			Suggestion: fmt.Sprintf("Reduce array size to at most %d items", c.config.MaxArraySize),
		}
	}

	result := values[0]
	for _, v := range values[1:] {
		if math.IsNaN(v) {
			return nil, &OperationError{
				Operation:  "math_max",
				Message:    "values array contains NaN",
				ErrorType:  ErrorTypeValidation,
				Suggestion: "Ensure all values are valid numbers, not NaN",
			}
		}
		if v > result {
			result = v
		}
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"operation":    "math_max",
			"values_count": len(values),
		},
	}, nil
}

// Helper functions

func getFloat64(inputs map[string]interface{}, key string) (float64, error) {
	v, ok := inputs[key]
	if !ok {
		return 0, fmt.Errorf("missing required parameter: %s", key)
	}

	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("%s must be a number, got %T", key, v)
	}
}

func getNumberArray(inputs map[string]interface{}, key string) ([]float64, error) {
	v, ok := inputs[key]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: %s", key)
	}

	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array, got %T", key, v)
	}

	result := make([]float64, len(arr))
	for i, item := range arr {
		switch n := item.(type) {
		case int:
			result[i] = float64(n)
		case int32:
			result[i] = float64(n)
		case int64:
			result[i] = float64(n)
		case float64:
			result[i] = n
		case float32:
			result[i] = float64(n)
		default:
			return nil, fmt.Errorf("values[%d] must be a number, got %T", i, item)
		}
	}

	return result, nil
}
