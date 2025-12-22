package workflow

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"text/template"
)

// Input size limits per NFR5
const (
	MaxJSONSize  = 1 * 1024 * 1024 // 1MB
	MaxArrayLen  = 10000           // 10,000 elements
)

// TemplateFuncMap returns the custom functions available in workflow templates.
func TemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		// Math
		"add":  add,
		"sub":  sub,
		"mul":  mul,
		"div":  div,
		"divf": divf,
		"mod":  mod,
		"min":  min,
		"max":  max,

		// JSON
		"toJson":       toJson,
		"toJsonPretty": toJsonPretty,
		"fromJson":     fromJson,

		// Strings (mostly direct stdlib mappings)
		"join":       joinFunc,
		"split":      strings.Split,
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"title":      titleCase,
		"trim":       strings.TrimSpace,
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"replace":    strings.Replace,

		// Collections
		"first":  first,
		"last":   last,
		"keys":   keys,
		"values": values,
		"hasKey": hasKey,
		"pluck":  pluck,

		// Default
		"default":  defaultFunc,
		"coalesce": coalesce,

		// Type conversion
		"toInt":    toInt,
		"toFloat":  toFloat,
		"toString": toString,
		"toBool":   toBool,
	}
}

// Type conversion helpers

// toFloat64 converts interface{} to float64
func toFloat64(v interface{}) (float64, error) {
	if v == nil {
		return 0, fmt.Errorf("cannot convert nil to float64")
	}

	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int16:
		return float64(val), nil
	case int8:
		return float64(val), nil
	case uint:
		return float64(val), nil
	case uint64:
		return float64(val), nil
	case uint32:
		return float64(val), nil
	case uint16:
		return float64(val), nil
	case uint8:
		return float64(val), nil
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string %q to float64: %w", val, err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// toInt64 converts interface{} to int64
func toInt64(v interface{}) (int64, error) {
	if v == nil {
		return 0, fmt.Errorf("cannot convert nil to int64")
	}

	switch val := v.(type) {
	case int64:
		return val, nil
	case int:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case int16:
		return int64(val), nil
	case int8:
		return int64(val), nil
	case uint:
		return int64(val), nil
	case uint64:
		return int64(val), nil
	case uint32:
		return int64(val), nil
	case uint16:
		return int64(val), nil
	case uint8:
		return int64(val), nil
	case float64:
		return int64(val), nil
	case float32:
		return int64(val), nil
	case string:
		var i int64
		_, err := fmt.Sscanf(val, "%d", &i)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string %q to int64: %w", val, err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

// allInts checks if all values are integer types (not floats or strings)
func allInts(values []interface{}) bool {
	for _, v := range values {
		switch v.(type) {
		case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
			continue
		default:
			return false
		}
	}
	return true
}

// Math functions

// add performs variadic addition
func add(values ...interface{}) (interface{}, error) {
	if len(values) == 0 {
		return 0, nil
	}

	var sum float64
	for _, v := range values {
		n, err := toFloat64(v)
		if err != nil {
			return nil, fmt.Errorf("add: %w", err)
		}
		sum += n
	}

	// Return int if all inputs were ints
	if allInts(values) {
		return int64(sum), nil
	}
	return sum, nil
}

// sub performs subtraction (a - b)
func sub(a, b interface{}) (interface{}, error) {
	aVal, err := toFloat64(a)
	if err != nil {
		return nil, fmt.Errorf("sub: %w", err)
	}

	bVal, err := toFloat64(b)
	if err != nil {
		return nil, fmt.Errorf("sub: %w", err)
	}

	result := aVal - bVal

	// Return int if both inputs were ints
	if allInts([]interface{}{a, b}) {
		return int64(result), nil
	}
	return result, nil
}

// mul performs variadic multiplication
func mul(values ...interface{}) (interface{}, error) {
	if len(values) == 0 {
		return 1, nil
	}

	product := 1.0
	for _, v := range values {
		n, err := toFloat64(v)
		if err != nil {
			return nil, fmt.Errorf("mul: %w", err)
		}
		product *= n
	}

	// Return int if all inputs were ints
	if allInts(values) {
		return int64(product), nil
	}
	return product, nil
}

// div performs integer division (a / b)
func div(a, b interface{}) (int64, error) {
	aVal, err := toInt64(a)
	if err != nil {
		return 0, fmt.Errorf("div: %w", err)
	}

	bVal, err := toInt64(b)
	if err != nil {
		return 0, fmt.Errorf("div: %w", err)
	}

	if bVal == 0 {
		return 0, fmt.Errorf("div: division by zero")
	}

	return aVal / bVal, nil
}

// divf performs float division (a / b)
func divf(a, b interface{}) (float64, error) {
	aVal, err := toFloat64(a)
	if err != nil {
		return 0, fmt.Errorf("divf: %w", err)
	}

	bVal, err := toFloat64(b)
	if err != nil {
		return 0, fmt.Errorf("divf: %w", err)
	}

	if bVal == 0 {
		return 0, fmt.Errorf("divf: division by zero")
	}

	return aVal / bVal, nil
}

// mod performs modulo operation (a % b)
func mod(a, b interface{}) (int64, error) {
	aVal, err := toInt64(a)
	if err != nil {
		return 0, fmt.Errorf("mod: %w", err)
	}

	bVal, err := toInt64(b)
	if err != nil {
		return 0, fmt.Errorf("mod: %w", err)
	}

	if bVal == 0 {
		return 0, fmt.Errorf("mod: division by zero")
	}

	return aVal % bVal, nil
}

// min returns the minimum of variadic arguments
func min(values ...interface{}) (interface{}, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("min: requires at least one argument")
	}

	minVal, err := toFloat64(values[0])
	if err != nil {
		return nil, fmt.Errorf("min: %w", err)
	}

	for i := 1; i < len(values); i++ {
		val, err := toFloat64(values[i])
		if err != nil {
			return nil, fmt.Errorf("min: %w", err)
		}
		if val < minVal {
			minVal = val
		}
	}

	// Return int if all inputs were ints
	if allInts(values) {
		return int64(minVal), nil
	}
	return minVal, nil
}

// max returns the maximum of variadic arguments
func max(values ...interface{}) (interface{}, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("max: requires at least one argument")
	}

	maxVal, err := toFloat64(values[0])
	if err != nil {
		return nil, fmt.Errorf("max: %w", err)
	}

	for i := 1; i < len(values); i++ {
		val, err := toFloat64(values[i])
		if err != nil {
			return nil, fmt.Errorf("max: %w", err)
		}
		if val > maxVal {
			maxVal = val
		}
	}

	// Return int if all inputs were ints
	if allInts(values) {
		return int64(maxVal), nil
	}
	return maxVal, nil
}

// JSON functions

// toJson serializes a value to compact JSON
func toJson(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("toJson: %w", err)
	}

	if len(data) > MaxJSONSize {
		return "", fmt.Errorf("toJson: output exceeds maximum size of %d bytes", MaxJSONSize)
	}

	return string(data), nil
}

// toJsonPretty serializes a value to indented JSON
func toJsonPretty(v interface{}) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("toJsonPretty: %w", err)
	}

	if len(data) > MaxJSONSize {
		return "", fmt.Errorf("toJsonPretty: output exceeds maximum size of %d bytes", MaxJSONSize)
	}

	return string(data), nil
}

// fromJson parses a JSON string to interface{}
func fromJson(s string) (interface{}, error) {
	if len(s) > MaxJSONSize {
		return nil, fmt.Errorf("fromJson: input exceeds maximum size of %d bytes", MaxJSONSize)
	}

	var result interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil, fmt.Errorf("fromJson: %w", err)
	}

	return result, nil
}

// String functions

// joinFunc wraps strings.Join to accept []interface{} from templates
func joinFunc(arr interface{}, sep string) (string, error) {
	slice := reflect.ValueOf(arr)
	if slice.Kind() != reflect.Slice && slice.Kind() != reflect.Array {
		return "", fmt.Errorf("join: first argument must be array or slice, got %T", arr)
	}

	if slice.Len() > MaxArrayLen {
		return "", fmt.Errorf("join: array exceeds maximum length of %d elements", MaxArrayLen)
	}

	parts := make([]string, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		parts[i] = fmt.Sprint(slice.Index(i).Interface())
	}

	return strings.Join(parts, sep), nil
}

// titleCase converts a string to title case (first letter uppercase)
// Using custom implementation since strings.Title is deprecated
func titleCase(s string) string {
	if s == "" {
		return s
	}
	// Simple title case: uppercase first rune, lowercase rest
	runes := []rune(s)
	return string(append([]rune{toUpperRune(runes[0])}, toLowerRunes(runes[1:])...))
}

func toUpperRune(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

func toLowerRunes(runes []rune) []rune {
	result := make([]rune, len(runes))
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + 32
		} else {
			result[i] = r
		}
	}
	return result
}

// Collection functions

// first returns the first element of a slice
func first(arr interface{}) (interface{}, error) {
	slice := reflect.ValueOf(arr)
	if slice.Kind() != reflect.Slice && slice.Kind() != reflect.Array {
		return nil, fmt.Errorf("first: argument must be array or slice, got %T", arr)
	}

	if slice.Len() == 0 {
		return nil, fmt.Errorf("first: array is empty")
	}

	return slice.Index(0).Interface(), nil
}

// last returns the last element of a slice
func last(arr interface{}) (interface{}, error) {
	slice := reflect.ValueOf(arr)
	if slice.Kind() != reflect.Slice && slice.Kind() != reflect.Array {
		return nil, fmt.Errorf("last: argument must be array or slice, got %T", arr)
	}

	if slice.Len() == 0 {
		return nil, fmt.Errorf("last: array is empty")
	}

	return slice.Index(slice.Len() - 1).Interface(), nil
}

// keys returns the keys of a map as a slice
func keys(m interface{}) ([]string, error) {
	mapVal := reflect.ValueOf(m)
	if mapVal.Kind() != reflect.Map {
		return nil, fmt.Errorf("keys: argument must be map, got %T", m)
	}

	if mapVal.Len() > MaxArrayLen {
		return nil, fmt.Errorf("keys: map exceeds maximum size of %d elements", MaxArrayLen)
	}

	result := make([]string, 0, mapVal.Len())
	for _, key := range mapVal.MapKeys() {
		result = append(result, fmt.Sprint(key.Interface()))
	}

	return result, nil
}

// values returns the values of a map as a slice
func values(m interface{}) ([]interface{}, error) {
	mapVal := reflect.ValueOf(m)
	if mapVal.Kind() != reflect.Map {
		return nil, fmt.Errorf("values: argument must be map, got %T", m)
	}

	if mapVal.Len() > MaxArrayLen {
		return nil, fmt.Errorf("values: map exceeds maximum size of %d elements", MaxArrayLen)
	}

	result := make([]interface{}, 0, mapVal.Len())
	for _, key := range mapVal.MapKeys() {
		result = append(result, mapVal.MapIndex(key).Interface())
	}

	return result, nil
}

// hasKey checks if a map has a specific key
func hasKey(m interface{}, key string) (bool, error) {
	mapVal := reflect.ValueOf(m)
	if mapVal.Kind() != reflect.Map {
		return false, fmt.Errorf("hasKey: first argument must be map, got %T", m)
	}

	keyVal := reflect.ValueOf(key)
	return mapVal.MapIndex(keyVal).IsValid(), nil
}

// pluck extracts a field from an array of objects
func pluck(arr interface{}, fieldName string) ([]interface{}, error) {
	slice := reflect.ValueOf(arr)
	if slice.Kind() != reflect.Slice && slice.Kind() != reflect.Array {
		return nil, fmt.Errorf("pluck: first argument must be array or slice, got %T", arr)
	}

	if slice.Len() > MaxArrayLen {
		return nil, fmt.Errorf("pluck: array exceeds maximum length of %d elements", MaxArrayLen)
	}

	result := make([]interface{}, 0, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		item := slice.Index(i)

		// Handle map[string]interface{}
		if item.Kind() == reflect.Map {
			keyVal := reflect.ValueOf(fieldName)
			fieldVal := item.MapIndex(keyVal)
			if fieldVal.IsValid() {
				result = append(result, fieldVal.Interface())
			}
			continue
		}

		// Handle struct
		if item.Kind() == reflect.Struct {
			fieldVal := item.FieldByName(fieldName)
			if fieldVal.IsValid() {
				result = append(result, fieldVal.Interface())
			}
			continue
		}
	}

	return result, nil
}

// Default and type conversion functions

// defaultFunc returns the default value if the value is nil or empty
func defaultFunc(value, defaultVal interface{}) interface{} {
	if value == nil {
		return defaultVal
	}

	// Check for empty string
	if s, ok := value.(string); ok && s == "" {
		return defaultVal
	}

	// Check for zero-length slice/map
	v := reflect.ValueOf(value)
	if (v.Kind() == reflect.Slice || v.Kind() == reflect.Map) && v.Len() == 0 {
		return defaultVal
	}

	return value
}

// coalesce returns the first non-empty value from variadic arguments
func coalesce(values ...interface{}) interface{} {
	for _, v := range values {
		if v == nil {
			continue
		}

		// Check for empty string
		if s, ok := v.(string); ok && s == "" {
			continue
		}

		// Check for zero-length slice/map
		rv := reflect.ValueOf(v)
		if (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Map) && rv.Len() == 0 {
			continue
		}

		return v
	}
	return nil
}

// toInt converts a value to integer
func toInt(v interface{}) (int64, error) {
	result, err := toInt64(v)
	if err != nil {
		return 0, fmt.Errorf("toInt: %w", err)
	}
	return result, nil
}

// toFloat converts a value to float
func toFloat(v interface{}) (float64, error) {
	result, err := toFloat64(v)
	if err != nil {
		return 0, fmt.Errorf("toFloat: %w", err)
	}
	return result, nil
}

// toString converts a value to string
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// toBool converts a value to boolean
func toBool(v interface{}) (bool, error) {
	if v == nil {
		return false, nil
	}

	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		switch strings.ToLower(val) {
		case "true", "1", "yes", "y":
			return true, nil
		case "false", "0", "no", "n", "":
			return false, nil
		default:
			return false, fmt.Errorf("toBool: cannot convert string %q to bool", val)
		}
	case int:
		return val != 0, nil
	case int64:
		return val != 0, nil
	case int32:
		return val != 0, nil
	case int16:
		return val != 0, nil
	case int8:
		return val != 0, nil
	case uint:
		return val != 0, nil
	case uint64:
		return val != 0, nil
	case uint32:
		return val != 0, nil
	case uint16:
		return val != 0, nil
	case uint8:
		return val != 0, nil
	case float64:
		return val != 0, nil
	case float32:
		return val != 0, nil
	default:
		return false, fmt.Errorf("toBool: cannot convert %T to bool", v)
	}
}
