package expression

import (
	"fmt"
	"reflect"
)

// containsFunc checks if a slice contains an element.
// Usage: contains(inputs.personas, "security")
//
// Supports slices of any type and performs deep equality comparison.
// Returns false if the first argument is not a slice.
func containsFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("contains requires exactly 2 arguments, got %d", len(args))
	}

	collection := args[0]
	target := args[1]

	if collection == nil {
		return false, nil
	}

	v := reflect.ValueOf(collection)

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i).Interface()
			if reflect.DeepEqual(elem, target) {
				return true, nil
			}
		}
		return false, nil

	case reflect.Map:
		// Check if key exists in map
		mapVal := v.MapIndex(reflect.ValueOf(target))
		return mapVal.IsValid(), nil

	case reflect.String:
		// Check if string contains substring
		str, ok := collection.(string)
		if !ok {
			return false, nil
		}
		substr, ok := target.(string)
		if !ok {
			return false, nil
		}
		return len(str) > 0 && len(substr) > 0 && contains(str, substr), nil

	default:
		return false, nil
	}
}

// contains checks if s contains substr (simple string contains).
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// lenFunc returns the length of a collection or string.
// Usage: len(inputs.items) > 0
func lenFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("len requires exactly 1 argument, got %d", len(args))
	}

	if args[0] == nil {
		return 0, nil
	}

	v := reflect.ValueOf(args[0])

	switch v.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
		return v.Len(), nil
	default:
		return nil, fmt.Errorf("len: unsupported type %T", args[0])
	}
}
