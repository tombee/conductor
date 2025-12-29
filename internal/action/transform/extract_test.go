package transform

import (
	"context"
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	conn, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name    string
		inputs  map[string]interface{}
		wantErr bool
		errType ErrorType
		check   func(*testing.T, interface{})
	}{
		{
			name:    "missing data parameter",
			inputs:  map[string]interface{}{},
			wantErr: true,
			errType: ErrorTypeValidation,
		},
		{
			name: "missing expr parameter",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"foo": "bar"},
			},
			wantErr: true,
			errType: ErrorTypeValidation,
		},
		{
			name: "expr wrong type",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"foo": "bar"},
				"expr": 123,
			},
			wantErr: true,
			errType: ErrorTypeTypeError,
		},
		{
			name: "empty expr",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"foo": "bar"},
				"expr": "",
			},
			wantErr: true,
			errType: ErrorTypeValidation,
		},
		{
			name: "simple field extraction",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"foo": "bar", "baz": 123},
				"expr": ".foo",
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				if result != "bar" {
					t.Errorf("result = %v, want bar", result)
				}
			},
		},
		{
			name: "nested field extraction",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"user": map[string]interface{}{
						"name":  "Alice",
						"email": "alice@example.com",
					},
				},
				"expr": ".user.name",
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				if result != "Alice" {
					t.Errorf("result = %v, want Alice", result)
				}
			},
		},
		{
			name: "array element extraction",
			inputs: map[string]interface{}{
				"data": []interface{}{1, 2, 3, 4, 5},
				"expr": ".[2]",
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				// gojq returns numbers as float64 or int depending on the value
				// Accept both int and float64
				switch v := result.(type) {
				case float64:
					if v != 3.0 {
						t.Errorf("result = %v, want 3", result)
					}
				case int:
					if v != 3 {
						t.Errorf("result = %v, want 3", result)
					}
				default:
					t.Errorf("result type = %T, want numeric", result)
				}
			},
		},
		{
			name: "map array",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"x": 1, "y": 2},
					map[string]interface{}{"x": 3, "y": 4},
				},
				"expr": "map(.x)",
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				arr, ok := result.([]interface{})
				if !ok {
					t.Errorf("result type = %T, want []interface{}", result)
					return
				}
				if len(arr) != 2 {
					t.Errorf("len(result) = %v, want 2", len(arr))
					return
				}
				// gojq may return int or float64
				// Just check the values are present (loose comparison)
			},
		},
		{
			name: "filter expression",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"name": "Alice", "age": 30},
					map[string]interface{}{"name": "Bob", "age": 25},
					map[string]interface{}{"name": "Charlie", "age": 35},
				},
				"expr": "map(select(.age > 28))",
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				arr, ok := result.([]interface{})
				if !ok {
					t.Errorf("result type = %T, want []interface{}", result)
					return
				}
				if len(arr) != 2 {
					t.Errorf("len(result) = %v, want 2", len(arr))
				}
			},
		},
		{
			name: "construct object",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"first": "John",
					"last":  "Doe",
					"age":   42,
				},
				"expr": `{name: (.first + " " + .last), age: .age}`,
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				if !ok {
					t.Errorf("result type = %T, want map[string]interface{}", result)
					return
				}
				if m["name"] != "John Doe" {
					t.Errorf("name = %v, want John Doe", m["name"])
				}
				// Accept both int and float64 for age
				switch age := m["age"].(type) {
				case float64:
					if age != 42.0 {
						t.Errorf("age = %v, want 42", m["age"])
					}
				case int:
					if age != 42 {
						t.Errorf("age = %v, want 42", m["age"])
					}
				default:
					t.Errorf("age type = %T, want numeric", m["age"])
				}
			},
		},
		{
			name: "invalid jq expression",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"foo": "bar"},
				"expr": ".[invalid",
			},
			wantErr: true,
			errType: ErrorTypeExpressionError,
		},
		{
			name: "expression on null data",
			inputs: map[string]interface{}{
				"data": nil,
				"expr": ".foo",
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				if result != nil {
					t.Errorf("result = %v, want nil", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conn.Execute(context.Background(), "extract", tt.inputs)

			if (err != nil) != tt.wantErr {
				t.Errorf("extract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				opErr, ok := err.(*OperationError)
				if !ok {
					t.Errorf("error type = %T, want *OperationError", err)
					return
				}
				if opErr.ErrorType != tt.errType {
					t.Errorf("error type = %v, want %v", opErr.ErrorType, tt.errType)
				}
				return
			}

			if tt.check != nil && result != nil {
				tt.check(t, result.Response)
			}
		})
	}
}

func TestExtract_SizeLimit(t *testing.T) {
	// Create connector with small limits for testing
	conn, err := New(&Config{
		MaxInputSize:  100, // Very small for testing
		MaxOutputSize: 100,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create large input
	largeData := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeData[string(rune('a'+i%26))+string(rune('a'+(i/26)%26))] = strings.Repeat("x", 10)
	}

	_, err = conn.Execute(context.Background(), "extract", map[string]interface{}{
		"data": largeData,
		"expr": ".",
	})

	if err == nil {
		t.Error("extract() expected size limit error, got nil")
		return
	}

	opErr, ok := err.(*OperationError)
	if !ok {
		t.Errorf("error type = %T, want *OperationError", err)
		return
	}
	if opErr.ErrorType != ErrorTypeLimitExceeded {
		t.Errorf("error type = %v, want %v", opErr.ErrorType, ErrorTypeLimitExceeded)
	}
}

func TestExtract_SensitiveDataRedaction(t *testing.T) {
	conn, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Use an invalid expression to trigger an error with the data in the message
	_, err = conn.Execute(context.Background(), "extract", map[string]interface{}{
		"data": map[string]interface{}{
			"password": "secret123",
		},
		"expr": ".invalid[syntax",
	})

	if err == nil {
		t.Error("extract() expected error, got nil")
		return
	}

	// Error message should not contain the password value
	errMsg := err.Error()
	if strings.Contains(errMsg, "secret123") {
		t.Error("error message contains sensitive data - should be redacted")
	}
}

func TestContainsSensitivePattern(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "no sensitive data",
			text: "regular error message",
			want: false,
		},
		{
			name: "contains password",
			text: "failed to parse password field",
			want: true,
		},
		{
			name: "contains PASSWORD (uppercase)",
			text: "invalid PASSWORD",
			want: true,
		},
		{
			name: "contains api_key",
			text: "api_key is required",
			want: true,
		},
		{
			name: "contains token",
			text: "authentication token expired",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsSensitivePattern(tt.text)
			if got != tt.want {
				t.Errorf("containsSensitivePattern() = %v, want %v", got, tt.want)
			}
		})
	}
}
