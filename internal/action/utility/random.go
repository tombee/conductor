package utility

import (
	"context"
	"fmt"
)

// randomInt generates a random integer in the range [min, max] (inclusive).
func (c *UtilityAction) randomInt(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	min, err := getInt64(inputs, "min")
	if err != nil {
		return nil, &OperationError{
			Operation:  "random_int",
			Message:    "invalid 'min' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'min' as an integer",
		}
	}

	max, err := getInt64(inputs, "max")
	if err != nil {
		return nil, &OperationError{
			Operation:  "random_int",
			Message:    "invalid 'max' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'max' as an integer",
		}
	}

	if min > max {
		return nil, &OperationError{
			Operation:  "random_int",
			Message:    "min must be <= max",
			ErrorType:  ErrorTypeRange,
			Suggestion: fmt.Sprintf("Swap values: min=%d, max=%d", max, min),
		}
	}

	result, err := c.randomSource.Int(min, max)
	if err != nil {
		return nil, err
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"operation": "random_int",
			"min":       min,
			"max":       max,
		},
	}, nil
}

// randomChoose selects one item randomly from an array.
func (c *UtilityAction) randomChoose(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	items, err := getArray(inputs, "items")
	if err != nil {
		return nil, &OperationError{
			Operation:  "random_choose",
			Message:    "invalid 'items' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'items' as an array",
		}
	}

	if len(items) == 0 {
		return nil, &OperationError{
			Operation:  "random_choose",
			Message:    "items cannot be empty",
			ErrorType:  ErrorTypeEmpty,
			Suggestion: "Provide at least one item to choose from",
		}
	}

	if len(items) > c.config.MaxArraySize {
		return nil, &OperationError{
			Operation:  "random_choose",
			Message:    fmt.Sprintf("items array exceeds maximum size of %d", c.config.MaxArraySize),
			ErrorType:  ErrorTypeValidation,
			Suggestion: fmt.Sprintf("Reduce array size to at most %d items", c.config.MaxArraySize),
		}
	}

	idx := c.randomSource.Intn(len(items))

	return &Result{
		Response: items[idx],
		Metadata: map[string]interface{}{
			"operation":   "random_choose",
			"total_items": len(items),
			"chosen_idx":  idx,
		},
	}, nil
}

// randomWeighted selects one item based on weighted probabilities.
func (c *UtilityAction) randomWeighted(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	items, err := getArray(inputs, "items")
	if err != nil {
		return nil, &OperationError{
			Operation:  "random_weighted",
			Message:    "invalid 'items' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'items' as an array of {value, weight} objects",
		}
	}

	if len(items) == 0 {
		return nil, &OperationError{
			Operation:  "random_weighted",
			Message:    "items cannot be empty",
			ErrorType:  ErrorTypeEmpty,
			Suggestion: "Provide at least one weighted item",
		}
	}

	if len(items) > c.config.MaxArraySize {
		return nil, &OperationError{
			Operation:  "random_weighted",
			Message:    fmt.Sprintf("items array exceeds maximum size of %d", c.config.MaxArraySize),
			ErrorType:  ErrorTypeValidation,
			Suggestion: fmt.Sprintf("Reduce array size to at most %d items", c.config.MaxArraySize),
		}
	}

	// Parse weighted items and calculate total weight
	type weightedItem struct {
		value  interface{}
		weight float64
	}
	var weightedItems []weightedItem
	var totalWeight float64

	for i, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return nil, &OperationError{
				Operation:  "random_weighted",
				Message:    fmt.Sprintf("item at index %d must be an object with 'value' and 'weight' fields", i),
				ErrorType:  ErrorTypeType,
				Suggestion: "Each item should be: {value: ..., weight: number}",
			}
		}

		value, hasValue := itemMap["value"]
		if !hasValue {
			return nil, &OperationError{
				Operation:  "random_weighted",
				Message:    fmt.Sprintf("item at index %d missing 'value' field", i),
				ErrorType:  ErrorTypeValidation,
				Suggestion: "Add a 'value' field to each item",
			}
		}

		weight, err := toFloat64(itemMap["weight"])
		if err != nil {
			return nil, &OperationError{
				Operation:  "random_weighted",
				Message:    fmt.Sprintf("item at index %d has invalid 'weight'", i),
				ErrorType:  ErrorTypeValidation,
				Cause:      err,
				Suggestion: "Weight must be a positive number",
			}
		}

		if weight < 0 {
			return nil, &OperationError{
				Operation:  "random_weighted",
				Message:    fmt.Sprintf("item at index %d has negative weight", i),
				ErrorType:  ErrorTypeRange,
				Suggestion: "Weight must be non-negative",
			}
		}

		weightedItems = append(weightedItems, weightedItem{value: value, weight: weight})
		totalWeight += weight
	}

	if totalWeight <= 0 {
		return nil, &OperationError{
			Operation:  "random_weighted",
			Message:    "total weight must be positive",
			ErrorType:  ErrorTypeRange,
			Suggestion: "At least one item must have a positive weight",
		}
	}

	// Generate random value in [0, totalWeight)
	randVal, err := c.randomSource.Int(0, int64(totalWeight*1000000))
	if err != nil {
		return nil, err
	}
	target := float64(randVal) / 1000000.0

	// Find the item corresponding to this random value
	var cumulative float64
	for _, wi := range weightedItems {
		cumulative += wi.weight
		if target < cumulative {
			return &Result{
				Response: wi.value,
				Metadata: map[string]interface{}{
					"operation":    "random_weighted",
					"total_items":  len(weightedItems),
					"total_weight": totalWeight,
				},
			}, nil
		}
	}

	// Fallback to last item (shouldn't happen with proper random)
	return &Result{
		Response: weightedItems[len(weightedItems)-1].value,
		Metadata: map[string]interface{}{
			"operation":    "random_weighted",
			"total_items":  len(weightedItems),
			"total_weight": totalWeight,
		},
	}, nil
}

// randomSample selects N items without replacement.
func (c *UtilityAction) randomSample(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	items, err := getArray(inputs, "items")
	if err != nil {
		return nil, &OperationError{
			Operation:  "random_sample",
			Message:    "invalid 'items' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'items' as an array",
		}
	}

	if len(items) == 0 {
		return nil, &OperationError{
			Operation:  "random_sample",
			Message:    "items cannot be empty",
			ErrorType:  ErrorTypeEmpty,
			Suggestion: "Provide at least one item to sample from",
		}
	}

	if len(items) > c.config.MaxArraySize {
		return nil, &OperationError{
			Operation:  "random_sample",
			Message:    fmt.Sprintf("items array exceeds maximum size of %d", c.config.MaxArraySize),
			ErrorType:  ErrorTypeValidation,
			Suggestion: fmt.Sprintf("Reduce array size to at most %d items", c.config.MaxArraySize),
		}
	}

	count, err := getInt(inputs, "count")
	if err != nil {
		return nil, &OperationError{
			Operation:  "random_sample",
			Message:    "invalid 'count' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'count' as a positive integer",
		}
	}

	if count <= 0 {
		return nil, &OperationError{
			Operation:  "random_sample",
			Message:    "count must be positive",
			ErrorType:  ErrorTypeRange,
			Suggestion: "Provide a count >= 1",
		}
	}

	if count > len(items) {
		return nil, &OperationError{
			Operation:  "random_sample",
			Message:    fmt.Sprintf("count (%d) exceeds items length (%d)", count, len(items)),
			ErrorType:  ErrorTypeRange,
			Suggestion: fmt.Sprintf("Reduce count to at most %d", len(items)),
		}
	}

	// Fisher-Yates shuffle on a copy, then take first count items
	result := make([]interface{}, len(items))
	copy(result, items)

	for i := len(result) - 1; i > 0; i-- {
		j := c.randomSource.Intn(i + 1)
		result[i], result[j] = result[j], result[i]
	}

	return &Result{
		Response: result[:count],
		Metadata: map[string]interface{}{
			"operation":   "random_sample",
			"total_items": len(items),
			"count":       count,
		},
	}, nil
}

// randomShuffle randomly reorders an array.
func (c *UtilityAction) randomShuffle(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	items, err := getArray(inputs, "items")
	if err != nil {
		return nil, &OperationError{
			Operation:  "random_shuffle",
			Message:    "invalid 'items' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'items' as an array",
		}
	}

	if len(items) > c.config.MaxArraySize {
		return nil, &OperationError{
			Operation:  "random_shuffle",
			Message:    fmt.Sprintf("items array exceeds maximum size of %d", c.config.MaxArraySize),
			ErrorType:  ErrorTypeValidation,
			Suggestion: fmt.Sprintf("Reduce array size to at most %d items", c.config.MaxArraySize),
		}
	}

	// Fisher-Yates shuffle on a copy
	result := make([]interface{}, len(items))
	copy(result, items)

	for i := len(result) - 1; i > 0; i-- {
		j := c.randomSource.Intn(i + 1)
		result[i], result[j] = result[j], result[i]
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"operation":   "random_shuffle",
			"total_items": len(items),
		},
	}, nil
}

// Helper functions for input parsing

func getInt64(inputs map[string]interface{}, key string) (int64, error) {
	v, ok := inputs[key]
	if !ok {
		return 0, fmt.Errorf("missing required parameter: %s", key)
	}

	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case float64:
		return int64(n), nil
	case float32:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("%s must be a number, got %T", key, v)
	}
}

func getInt(inputs map[string]interface{}, key string) (int, error) {
	v, err := getInt64(inputs, key)
	return int(v), err
}

func getArray(inputs map[string]interface{}, key string) ([]interface{}, error) {
	v, ok := inputs[key]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: %s", key)
	}

	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array, got %T", key, v)
	}

	return arr, nil
}

func toFloat64(v interface{}) (float64, error) {
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
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}
