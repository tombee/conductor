package transform

import (
	"context"
	"fmt"
)

// merge operation - combines multiple objects or arrays.
// For objects: shallow merge by default (rightmost wins), deep merge with strategy=deep.
// For arrays: concatenation.
func (c *TransformConnector) merge(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get sources - can be single data param or array of sources
	var sources []interface{}

	// Check for 'sources' parameter (multi-source array)
	if sourcesParam, ok := inputs["sources"]; ok {
		sourcesArr, ok := sourcesParam.([]interface{})
		if !ok {
			return nil, &OperationError{
				Operation:  "merge",
				Message:    "sources must be an array",
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "Provide sources as an array of objects to merge",
			}
		}
		sources = sourcesArr
	} else if data, ok := inputs["data"]; ok {
		// Single data parameter - should be an array of objects to merge
		dataArr, ok := data.([]interface{})
		if !ok {
			return nil, &OperationError{
				Operation:  "merge",
				Message:    "data must be an array of objects to merge",
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "Provide data as an array of objects, or use sources parameter",
			}
		}
		sources = dataArr
	} else {
		return nil, &OperationError{
			Operation:  "merge",
			Message:    "missing required parameter: data or sources",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide either data (array of objects) or sources (array of objects)",
		}
	}

	if len(sources) == 0 {
		return &Result{
			Response: map[string]interface{}{},
			Metadata: map[string]interface{}{
				"source_count": 0,
			},
		}, nil
	}

	// Get strategy parameter (default: shallow)
	strategy := "shallow"
	if strategyParam, ok := inputs["strategy"]; ok {
		if strategyStr, ok := strategyParam.(string); ok {
			strategy = strategyStr
		}
	}

	// Validate strategy
	if strategy != "shallow" && strategy != "deep" {
		return nil, &OperationError{
			Operation:  "merge",
			Message:    fmt.Sprintf("invalid strategy: %s", strategy),
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Use strategy 'shallow' (default) or 'deep'",
		}
	}

	// Check if all sources are objects or all are arrays
	firstSource := sources[0]
	var result interface{}
	var err error

	if _, isMap := firstSource.(map[string]interface{}); isMap {
		// Merge objects
		if strategy == "deep" {
			result, err = c.mergeObjectsDeep(sources)
		} else {
			result, err = c.mergeObjectsShallow(sources)
		}
	} else if _, isArray := firstSource.([]interface{}); isArray {
		// Concatenate arrays (strategy doesn't matter for arrays)
		result, err = c.concatenateArrays(sources)
	} else {
		return nil, &OperationError{
			Operation:  "merge",
			Message:    fmt.Sprintf("cannot merge type %T", firstSource),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Merge only works with objects or arrays",
		}
	}

	if err != nil {
		return nil, err
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"strategy":     strategy,
			"source_count": len(sources),
		},
	}, nil
}

// mergeObjectsShallow performs shallow merge where rightmost wins for conflicts.
func (c *TransformConnector) mergeObjectsShallow(sources []interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for i, source := range sources {
		obj, ok := source.(map[string]interface{})
		if !ok {
			return nil, &OperationError{
				Operation:  "merge",
				Message:    fmt.Sprintf("source %d is not an object (got %T)", i, source),
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "All sources must be objects for object merge",
			}
		}

		// Shallow copy - rightmost wins
		for key, value := range obj {
			result[key] = value
		}
	}

	return result, nil
}

// mergeObjectsDeep performs deep recursive merge.
// Objects are merged recursively, arrays are concatenated.
func (c *TransformConnector) mergeObjectsDeep(sources []interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for i, source := range sources {
		obj, ok := source.(map[string]interface{})
		if !ok {
			return nil, &OperationError{
				Operation:  "merge",
				Message:    fmt.Sprintf("source %d is not an object (got %T)", i, source),
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "All sources must be objects for object merge",
			}
		}

		result = c.deepMergeObjects(result, obj)
	}

	return result, nil
}

// deepMergeObjects recursively merges two objects.
func (c *TransformConnector) deepMergeObjects(dst, src map[string]interface{}) map[string]interface{} {
	for key, srcValue := range src {
		if dstValue, exists := dst[key]; exists {
			// Both values exist - check if we can merge recursively
			dstMap, dstIsMap := dstValue.(map[string]interface{})
			srcMap, srcIsMap := srcValue.(map[string]interface{})

			if dstIsMap && srcIsMap {
				// Both are objects - merge recursively
				dst[key] = c.deepMergeObjects(dstMap, srcMap)
				continue
			}

			// Check if both are arrays - concatenate them
			dstArr, dstIsArray := dstValue.([]interface{})
			srcArr, srcIsArray := srcValue.([]interface{})

			if dstIsArray && srcIsArray {
				// Both are arrays - concatenate
				dst[key] = append(dstArr, srcArr...)
				continue
			}
		}

		// No special handling - rightmost wins
		dst[key] = srcValue
	}

	return dst
}

// concatenateArrays merges multiple arrays into one.
func (c *TransformConnector) concatenateArrays(sources []interface{}) ([]interface{}, error) {
	var result []interface{}

	for i, source := range sources {
		arr, ok := source.([]interface{})
		if !ok {
			return nil, &OperationError{
				Operation:  "merge",
				Message:    fmt.Sprintf("source %d is not an array (got %T)", i, source),
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "All sources must be arrays for array merge",
			}
		}

		result = append(result, arr...)
	}

	// Check array size limit
	if len(result) > c.config.MaxArrayItems {
		return nil, &OperationError{
			Operation:  "merge",
			Message:    fmt.Sprintf("merged array size (%d items) exceeds maximum (%d items)", len(result), c.config.MaxArrayItems),
			ErrorType:  ErrorTypeLimitExceeded,
			Suggestion: "Reduce the number of sources or items per source",
		}
	}

	return result, nil
}

// concat operation - concatenates multiple arrays.
func (c *TransformConnector) concat(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get sources - can be single data param or array of sources
	var sources []interface{}

	// Check for 'sources' parameter (multi-source array)
	if sourcesParam, ok := inputs["sources"]; ok {
		sourcesArr, ok := sourcesParam.([]interface{})
		if !ok {
			return nil, &OperationError{
				Operation:  "concat",
				Message:    "sources must be an array",
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "Provide sources as an array of arrays to concatenate",
			}
		}
		sources = sourcesArr
	} else if data, ok := inputs["data"]; ok {
		// Single data parameter - should be an array of arrays
		dataArr, ok := data.([]interface{})
		if !ok {
			return nil, &OperationError{
				Operation:  "concat",
				Message:    "data must be an array of arrays to concatenate",
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "Provide data as an array of arrays, or use sources parameter",
			}
		}
		sources = dataArr
	} else {
		return nil, &OperationError{
			Operation:  "concat",
			Message:    "missing required parameter: data or sources",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide either data (array of arrays) or sources (array of arrays)",
		}
	}

	if len(sources) == 0 {
		return &Result{
			Response: []interface{}{},
			Metadata: map[string]interface{}{
				"source_count": 0,
				"total_items":  0,
			},
		}, nil
	}

	var result []interface{}

	for i, source := range sources {
		arr, ok := source.([]interface{})
		if !ok {
			return nil, &OperationError{
				Operation:  "concat",
				Message:    fmt.Sprintf("source %d is not an array (got %T)", i, source),
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "All sources must be arrays",
			}
		}

		result = append(result, arr...)
	}

	// Check array size limit
	if len(result) > c.config.MaxArrayItems {
		return nil, &OperationError{
			Operation:  "concat",
			Message:    fmt.Sprintf("concatenated array size (%d items) exceeds maximum (%d items)", len(result), c.config.MaxArrayItems),
			ErrorType:  ErrorTypeLimitExceeded,
			Suggestion: "Reduce the number of sources or items per source",
		}
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"source_count": len(sources),
			"total_items":  len(result),
		},
	}, nil
}

// flatten operation - flattens nested arrays by one level (default) or recursively.
func (c *TransformConnector) flatten(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "flatten",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with array to flatten",
		}
	}

	// Check if data is nil/null
	if data == nil {
		return nil, &OperationError{
			Operation:  "flatten",
			Message:    "cannot flatten null or undefined value",
			ErrorType:  ErrorTypeEmptyInput,
			Suggestion: "Provide a valid array to flatten",
		}
	}

	// Verify input is an array
	arr, ok := data.([]interface{})
	if !ok {
		return nil, &OperationError{
			Operation:  "flatten",
			Message:    "input must be an array",
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Use transform.flatten only with array inputs",
		}
	}

	// Get depth parameter (default: 1)
	depth := 1
	if depthParam, ok := inputs["depth"]; ok {
		if depthFloat, ok := depthParam.(float64); ok {
			depth = int(depthFloat)
		} else if depthInt, ok := depthParam.(int); ok {
			depth = depthInt
		}
	}

	if depth < 1 {
		return nil, &OperationError{
			Operation:  "flatten",
			Message:    "depth must be at least 1",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide depth parameter with positive integer value",
		}
	}

	// Perform flattening
	result := c.flattenArray(arr, depth)

	// Check array size limit
	if len(result) > c.config.MaxArrayItems {
		return nil, &OperationError{
			Operation:  "flatten",
			Message:    fmt.Sprintf("flattened array size (%d items) exceeds maximum (%d items)", len(result), c.config.MaxArrayItems),
			ErrorType:  ErrorTypeLimitExceeded,
			Suggestion: "Reduce input size or use a smaller depth",
		}
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"depth":        depth,
			"input_items":  len(arr),
			"output_items": len(result),
		},
	}, nil
}

// flattenArray recursively flattens an array to the specified depth.
func (c *TransformConnector) flattenArray(arr []interface{}, depth int) []interface{} {
	if depth <= 0 {
		return arr
	}

	var result []interface{}

	for _, item := range arr {
		if nestedArr, ok := item.([]interface{}); ok {
			// Item is an array - flatten it
			flattened := c.flattenArray(nestedArr, depth-1)
			result = append(result, flattened...)
		} else {
			// Item is not an array - add as is
			result = append(result, item)
		}
	}

	return result
}
