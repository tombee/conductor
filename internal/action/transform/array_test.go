package transform

import (
	"context"
	"testing"
)

func TestSplit(t *testing.T) {
	action, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name        string
		inputs      map[string]interface{}
		wantErr     bool
		wantErrType ErrorType
		checkResult func(t *testing.T, result *Result)
	}{
		{
			name: "pass through array",
			inputs: map[string]interface{}{
				"data": []interface{}{"a", "b", "c"},
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 3 {
					t.Errorf("expected 3 items, got %d", len(arr))
				}
				if arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
					t.Errorf("array values incorrect: %v", arr)
				}
				if count, ok := result.Metadata["item_count"].(int); !ok || count != 3 {
					t.Errorf("expected item_count=3 in metadata, got %v", result.Metadata["item_count"])
				}
			},
		},
		{
			name: "empty array",
			inputs: map[string]interface{}{
				"data": []interface{}{},
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 0 {
					t.Errorf("expected 0 items, got %d", len(arr))
				}
			},
		},
		{
			name: "array of objects",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"id": 1},
					map[string]interface{}{"id": 2},
				},
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 2 {
					t.Errorf("expected 2 items, got %d", len(arr))
				}
			},
		},
		{
			name: "error on non-array input",
			inputs: map[string]interface{}{
				"data": "not an array",
			},
			wantErr:     true,
			wantErrType: ErrorTypeTypeError,
		},
		{
			name: "error on object input",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"key": "value"},
			},
			wantErr:     true,
			wantErrType: ErrorTypeTypeError,
		},
		{
			name: "error on null input",
			inputs: map[string]interface{}{
				"data": nil,
			},
			wantErr:     true,
			wantErrType: ErrorTypeEmptyInput,
		},
		{
			name:        "error on missing data parameter",
			inputs:      map[string]interface{}{},
			wantErr:     true,
			wantErrType: ErrorTypeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := action.split(ctx, tt.inputs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				opErr, ok := err.(*OperationError)
				if !ok {
					t.Errorf("expected OperationError, got %T", err)
					return
				}
				if opErr.ErrorType != tt.wantErrType {
					t.Errorf("expected error type %s, got %s", tt.wantErrType, opErr.ErrorType)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	action, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name        string
		inputs      map[string]interface{}
		wantErr     bool
		wantErrType ErrorType
		checkResult func(t *testing.T, result *Result)
	}{
		{
			name: "filter by boolean field",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"name": "item1", "active": true},
					map[string]interface{}{"name": "item2", "active": false},
					map[string]interface{}{"name": "item3", "active": true},
				},
				"expr": ".active",
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 2 {
					t.Errorf("expected 2 items, got %d", len(arr))
					return
				}
				// Verify filtered items have active=true
				for _, item := range arr {
					obj, ok := item.(map[string]interface{})
					if !ok {
						t.Errorf("expected object in result, got %T", item)
						continue
					}
					if obj["active"] != true {
						t.Errorf("expected active=true, got %v", obj["active"])
					}
				}
			},
		},
		{
			name: "filter by comparison",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"count": float64(5)},
					map[string]interface{}{"count": float64(15)},
					map[string]interface{}{"count": float64(3)},
				},
				"expr": ".count > 10",
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 1 {
					t.Errorf("expected 1 item, got %d", len(arr))
					return
				}
			},
		},
		{
			name: "filter all items out",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"active": false},
					map[string]interface{}{"active": false},
				},
				"expr": ".active",
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 0 {
					t.Errorf("expected 0 items, got %d", len(arr))
				}
			},
		},
		{
			name: "filter with string comparison",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"severity": "high"},
					map[string]interface{}{"severity": "low"},
					map[string]interface{}{"severity": "high"},
				},
				"expr": `.severity == "high"`,
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 2 {
					t.Errorf("expected 2 items, got %d", len(arr))
				}
			},
		},
		{
			name: "empty array",
			inputs: map[string]interface{}{
				"data": []interface{}{},
				"expr": ".active",
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 0 {
					t.Errorf("expected 0 items, got %d", len(arr))
				}
			},
		},
		{
			name: "error on missing data parameter",
			inputs: map[string]interface{}{
				"expr": ".active",
			},
			wantErr:     true,
			wantErrType: ErrorTypeValidation,
		},
		{
			name: "error on missing expr parameter",
			inputs: map[string]interface{}{
				"data": []interface{}{},
			},
			wantErr:     true,
			wantErrType: ErrorTypeValidation,
		},
		{
			name: "error on null data",
			inputs: map[string]interface{}{
				"data": nil,
				"expr": ".active",
			},
			wantErr:     true,
			wantErrType: ErrorTypeEmptyInput,
		},
		{
			name: "error on non-array data",
			inputs: map[string]interface{}{
				"data": "not an array",
				"expr": ".active",
			},
			wantErr:     true,
			wantErrType: ErrorTypeTypeError,
		},
		{
			name: "error on non-string expr",
			inputs: map[string]interface{}{
				"data": []interface{}{},
				"expr": 123,
			},
			wantErr:     true,
			wantErrType: ErrorTypeTypeError,
		},
		{
			name: "error on empty expr",
			inputs: map[string]interface{}{
				"data": []interface{}{},
				"expr": "",
			},
			wantErr:     true,
			wantErrType: ErrorTypeValidation,
		},
		{
			name: "error on invalid jq expression",
			inputs: map[string]interface{}{
				"data": []interface{}{map[string]interface{}{"x": 1}},
				"expr": "..invalid..",
			},
			wantErr:     true,
			wantErrType: ErrorTypeExpressionError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := action.filter(ctx, tt.inputs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				opErr, ok := err.(*OperationError)
				if !ok {
					t.Errorf("expected OperationError, got %T", err)
					return
				}
				if opErr.ErrorType != tt.wantErrType {
					t.Errorf("expected error type %s, got %s", tt.wantErrType, opErr.ErrorType)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestMap(t *testing.T) {
	action, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name        string
		inputs      map[string]interface{}
		wantErr     bool
		wantErrType ErrorType
		checkResult func(t *testing.T, result *Result)
	}{
		{
			name: "map to field value",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"name": "Alice", "age": float64(30)},
					map[string]interface{}{"name": "Bob", "age": float64(25)},
				},
				"expr": ".name",
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 2 {
					t.Errorf("expected 2 items, got %d", len(arr))
					return
				}
				if arr[0] != "Alice" || arr[1] != "Bob" {
					t.Errorf("expected [Alice, Bob], got %v", arr)
				}
			},
		},
		{
			name: "map to object shape",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"id": float64(1), "title": "Task 1", "extra": "ignored"},
					map[string]interface{}{"id": float64(2), "title": "Task 2", "extra": "ignored"},
				},
				"expr": `{id: .id, title: .title}`,
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 2 {
					t.Errorf("expected 2 items, got %d", len(arr))
					return
				}
				// Check first object
				obj, ok := arr[0].(map[string]interface{})
				if !ok {
					t.Errorf("expected object, got %T", arr[0])
					return
				}
				if obj["id"] != float64(1) || obj["title"] != "Task 1" {
					t.Errorf("unexpected object values: %v", obj)
				}
				if _, hasExtra := obj["extra"]; hasExtra {
					t.Errorf("unexpected extra field in mapped object")
				}
			},
		},
		{
			name: "map with calculation",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"count": float64(5)},
					map[string]interface{}{"count": float64(10)},
				},
				"expr": ".count * 2",
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 2 {
					t.Errorf("expected 2 items, got %d", len(arr))
					return
				}
				if arr[0] != float64(10) || arr[1] != float64(20) {
					t.Errorf("expected [10, 20], got %v", arr)
				}
			},
		},
		{
			name: "map to string concatenation",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"first": "John", "last": "Doe"},
					map[string]interface{}{"first": "Jane", "last": "Smith"},
				},
				"expr": `.first + " " + .last`,
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 2 {
					t.Errorf("expected 2 items, got %d", len(arr))
					return
				}
				if arr[0] != "John Doe" || arr[1] != "Jane Smith" {
					t.Errorf("expected [John Doe, Jane Smith], got %v", arr)
				}
			},
		},
		{
			name: "empty array",
			inputs: map[string]interface{}{
				"data": []interface{}{},
				"expr": ".name",
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				arr, ok := result.Response.([]interface{})
				if !ok {
					t.Errorf("expected array response, got %T", result.Response)
					return
				}
				if len(arr) != 0 {
					t.Errorf("expected 0 items, got %d", len(arr))
				}
			},
		},
		{
			name: "error on missing data parameter",
			inputs: map[string]interface{}{
				"expr": ".name",
			},
			wantErr:     true,
			wantErrType: ErrorTypeValidation,
		},
		{
			name: "error on missing expr parameter",
			inputs: map[string]interface{}{
				"data": []interface{}{},
			},
			wantErr:     true,
			wantErrType: ErrorTypeValidation,
		},
		{
			name: "error on null data",
			inputs: map[string]interface{}{
				"data": nil,
				"expr": ".name",
			},
			wantErr:     true,
			wantErrType: ErrorTypeEmptyInput,
		},
		{
			name: "error on non-array data",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"x": 1},
				"expr": ".name",
			},
			wantErr:     true,
			wantErrType: ErrorTypeTypeError,
		},
		{
			name: "error on non-string expr",
			inputs: map[string]interface{}{
				"data": []interface{}{},
				"expr": 123,
			},
			wantErr:     true,
			wantErrType: ErrorTypeTypeError,
		},
		{
			name: "error on empty expr",
			inputs: map[string]interface{}{
				"data": []interface{}{},
				"expr": "",
			},
			wantErr:     true,
			wantErrType: ErrorTypeValidation,
		},
		{
			name: "error on invalid jq expression",
			inputs: map[string]interface{}{
				"data": []interface{}{map[string]interface{}{"x": 1}},
				"expr": "..invalid..",
			},
			wantErr:     true,
			wantErrType: ErrorTypeExpressionError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := action.mapArray(ctx, tt.inputs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				opErr, ok := err.(*OperationError)
				if !ok {
					t.Errorf("expected OperationError, got %T", err)
					return
				}
				if opErr.ErrorType != tt.wantErrType {
					t.Errorf("expected error type %s, got %s", tt.wantErrType, opErr.ErrorType)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}
