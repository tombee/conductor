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

package assert

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// AssertionFunctions returns a map of custom assertion functions.
// These extend expr-lang/expr for testing assertions.
// Note: expr-lang reserves "contains", "in", "matches", and "count" as built-in operators.
// We use different names to avoid conflicts.
func AssertionFunctions() map[string]interface{} {
	return map[string]interface{}{
		// String operators (using alternative names to avoid reserved words)
		"has":   containsOp, // Alternative to "contains"
		"match": matchesOp,  // Alternative to "matches"

		// Collection operators
		"includes": inOp, // Alternative to "in"
		"notIn":    notInOp,

		// Pipe operators (for chaining)
		"len":       lengthPipe, // Alternative to "length" (expr has "count")
		"lowercase": lowercasePipe,
		"uppercase": uppercasePipe,
		"json":      jsonPipe,
		"round":     roundPipe,
	}
}

// containsOp checks if a string contains a substring or if a collection contains an element.
// Usage: body contains "success"
// Usage: items contains {"id": 1}
func containsOp(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("contains requires exactly 2 arguments, got %d", len(args))
	}

	haystack := args[0]
	needle := args[1]

	if haystack == nil {
		return false, nil
	}

	v := reflect.ValueOf(haystack)

	switch v.Kind() {
	case reflect.String:
		str, ok := haystack.(string)
		if !ok {
			return false, nil
		}
		substr, ok := needle.(string)
		if !ok {
			return false, nil
		}
		return strings.Contains(str, substr), nil

	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i).Interface()
			if reflect.DeepEqual(elem, needle) {
				return true, nil
			}
		}
		return false, nil

	case reflect.Map:
		// Check if key exists in map
		mapVal := v.MapIndex(reflect.ValueOf(needle))
		return mapVal.IsValid(), nil

	default:
		return false, fmt.Errorf("contains: unsupported type %T", haystack)
	}
}

// matchesOp checks if a string matches a regular expression.
// Usage: id matches "^[A-Z]{3}-\\d+$"
func matchesOp(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("matches requires exactly 2 arguments, got %d", len(args))
	}

	str, ok := args[0].(string)
	if !ok {
		return false, fmt.Errorf("matches: first argument must be a string, got %T", args[0])
	}

	pattern, ok := args[1].(string)
	if !ok {
		return false, fmt.Errorf("matches: second argument must be a string pattern, got %T", args[1])
	}

	matched, err := regexp.MatchString(pattern, str)
	if err != nil {
		return false, fmt.Errorf("matches: invalid regex pattern: %w", err)
	}

	return matched, nil
}

// inOp checks if a value is in a collection.
// Usage: status in [200, 201, 202]
func inOp(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("in requires exactly 2 arguments, got %d", len(args))
	}

	needle := args[0]
	haystack := args[1]

	if haystack == nil {
		return false, nil
	}

	v := reflect.ValueOf(haystack)

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i).Interface()
			if reflect.DeepEqual(elem, needle) {
				return true, nil
			}
		}
		return false, nil

	case reflect.Map:
		// Check if key exists in map
		mapVal := v.MapIndex(reflect.ValueOf(needle))
		return mapVal.IsValid(), nil

	default:
		return false, fmt.Errorf("in: second argument must be a collection, got %T", haystack)
	}
}

// notInOp checks if a value is not in a collection.
// Usage: status notIn [400, 500]
func notInOp(args ...interface{}) (interface{}, error) {
	result, err := inOp(args...)
	if err != nil {
		return nil, err
	}

	isIn, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("notIn: unexpected result type %T", result)
	}

	return !isIn, nil
}
