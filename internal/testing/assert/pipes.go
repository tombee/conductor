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
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

// lengthPipe returns the length of a string, array, slice, or map.
// Usage: body.items | length
func lengthPipe(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("length requires exactly 1 argument, got %d", len(args))
	}

	if args[0] == nil {
		return 0, nil
	}

	v := reflect.ValueOf(args[0])

	switch v.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return v.Len(), nil
	default:
		return nil, fmt.Errorf("length: cannot get length of type %T; expected string, array, slice, or map", args[0])
	}
}

// lowercasePipe converts a string to lowercase.
// Usage: body.name | lowercase
func lowercasePipe(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("lowercase requires exactly 1 argument, got %d", len(args))
	}

	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("lowercase: expected string, got %T", args[0])
	}

	return strings.ToLower(str), nil
}

// uppercasePipe converts a string to uppercase.
// Usage: body.name | uppercase
func uppercasePipe(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("uppercase requires exactly 1 argument, got %d", len(args))
	}

	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("uppercase: expected string, got %T", args[0])
	}

	return strings.ToUpper(str), nil
}

// jsonPipe parses a JSON string into a value.
// Usage: body.data | json
func jsonPipe(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("json requires exactly 1 argument, got %d", len(args))
	}

	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("json: expected string, got %T", args[0])
	}

	var result interface{}
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		return nil, fmt.Errorf("json: failed to parse JSON: %w", err)
	}

	return result, nil
}

// roundPipe rounds a number to the nearest integer.
// Usage: body.score | round
func roundPipe(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("round requires exactly 1 argument, got %d", len(args))
	}

	// Handle different numeric types
	switch v := args[0].(type) {
	case float64:
		return math.Round(v), nil
	case float32:
		return math.Round(float64(v)), nil
	case int, int8, int16, int32, int64:
		return args[0], nil // Already an integer
	case uint, uint8, uint16, uint32, uint64:
		return args[0], nil // Already an integer
	default:
		// Try to convert to float64
		val := reflect.ValueOf(args[0])
		if val.CanFloat() {
			return math.Round(val.Float()), nil
		}
		if val.CanInt() {
			return val.Int(), nil // Already an integer
		}
		if val.CanUint() {
			return val.Uint(), nil // Already an integer
		}
		return nil, fmt.Errorf("round: expected number, got %T", args[0])
	}
}
