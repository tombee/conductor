package transform

import (
	"context"
	"testing"
)

func TestTransformAction_Pick(t *testing.T) {
	conn, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name       string
		inputs     map[string]interface{}
		wantErr    bool
		errType    ErrorType
		wantKeys   []string
		wantValues map[string]interface{}
	}{
		{
			name: "pick existing keys",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"name":     "John",
					"age":      30,
					"email":    "john@example.com",
					"password": "secret",
				},
				"keys": []interface{}{"name", "email"},
			},
			wantErr:  false,
			wantKeys: []string{"name", "email"},
			wantValues: map[string]interface{}{
				"name":  "John",
				"email": "john@example.com",
			},
		},
		{
			name: "pick with some missing keys",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"name": "John",
					"age":  30,
				},
				"keys": []interface{}{"name", "email", "phone"},
			},
			wantErr:  false,
			wantKeys: []string{"name"},
			wantValues: map[string]interface{}{
				"name": "John",
			},
		},
		{
			name: "pick all missing keys returns empty object",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"name": "John",
					"age":  30,
				},
				"keys": []interface{}{"email", "phone"},
			},
			wantErr:    false,
			wantKeys:   []string{},
			wantValues: map[string]interface{}{},
		},
		{
			name: "pick with empty keys array",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"name": "John",
					"age":  30,
				},
				"keys": []interface{}{},
			},
			wantErr:    false,
			wantKeys:   []string{},
			wantValues: map[string]interface{}{},
		},
		{
			name: "pick with nested values",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"user": map[string]interface{}{
						"id":   1,
						"name": "John",
					},
					"active": true,
					"meta":   "internal",
				},
				"keys": []interface{}{"user", "active"},
			},
			wantErr:  false,
			wantKeys: []string{"user", "active"},
		},
		{
			name:    "missing data parameter",
			inputs:  map[string]interface{}{"keys": []interface{}{"name"}},
			wantErr: true,
			errType: ErrorTypeValidation,
		},
		{
			name: "missing keys parameter",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"name": "John"},
			},
			wantErr: true,
			errType: ErrorTypeValidation,
		},
		{
			name: "null data",
			inputs: map[string]interface{}{
				"data": nil,
				"keys": []interface{}{"name"},
			},
			wantErr: true,
			errType: ErrorTypeEmptyInput,
		},
		{
			name: "non-object data",
			inputs: map[string]interface{}{
				"data": []interface{}{"a", "b"},
				"keys": []interface{}{"name"},
			},
			wantErr: true,
			errType: ErrorTypeTypeError,
		},
		{
			name: "non-array keys",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"name": "John"},
				"keys": "name",
			},
			wantErr: true,
			errType: ErrorTypeTypeError,
		},
		{
			name: "non-string key in array",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"name": "John"},
				"keys": []interface{}{"name", 123},
			},
			wantErr: true,
			errType: ErrorTypeTypeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conn.Execute(context.Background(), "pick", tt.inputs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("pick() expected error, got nil")
					return
				}
				opErr, ok := err.(*OperationError)
				if !ok {
					t.Errorf("pick() error type = %T, want *OperationError", err)
					return
				}
				if opErr.ErrorType != tt.errType {
					t.Errorf("pick() error type = %v, want %v", opErr.ErrorType, tt.errType)
				}
				return
			}

			if err != nil {
				t.Errorf("pick() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Error("pick() returned nil result")
				return
			}

			response, ok := result.Response.(map[string]interface{})
			if !ok {
				t.Errorf("pick() response type = %T, want map[string]interface{}", result.Response)
				return
			}

			// Check that only expected keys are present
			if len(response) != len(tt.wantKeys) {
				t.Errorf("pick() response has %d keys, want %d", len(response), len(tt.wantKeys))
			}

			for _, key := range tt.wantKeys {
				if _, exists := response[key]; !exists {
					t.Errorf("pick() response missing expected key %q", key)
				}
			}

			// Check specific values if provided
			for key, wantValue := range tt.wantValues {
				if gotValue, exists := response[key]; !exists {
					t.Errorf("pick() response missing key %q", key)
				} else if gotValue != wantValue {
					t.Errorf("pick() response[%q] = %v, want %v", key, gotValue, wantValue)
				}
			}

			// Check metadata
			if result.Metadata == nil {
				t.Error("pick() metadata is nil")
				return
			}
			if _, ok := result.Metadata["keys_requested"]; !ok {
				t.Error("pick() metadata missing keys_requested")
			}
			if _, ok := result.Metadata["keys_found"]; !ok {
				t.Error("pick() metadata missing keys_found")
			}
		})
	}
}

func TestTransformAction_Omit(t *testing.T) {
	conn, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name       string
		inputs     map[string]interface{}
		wantErr    bool
		errType    ErrorType
		wantKeys   []string
		wantValues map[string]interface{}
	}{
		{
			name: "omit existing keys",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"name":     "John",
					"age":      30,
					"password": "secret",
				},
				"keys": []interface{}{"password"},
			},
			wantErr:  false,
			wantKeys: []string{"name", "age"},
			wantValues: map[string]interface{}{
				"name": "John",
				"age":  30,
			},
		},
		{
			name: "omit multiple keys",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"name":     "John",
					"age":      30,
					"email":    "john@example.com",
					"password": "secret",
					"token":    "abc123",
				},
				"keys": []interface{}{"password", "token"},
			},
			wantErr:  false,
			wantKeys: []string{"name", "age", "email"},
		},
		{
			name: "omit non-existing keys (no-op)",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"name": "John",
					"age":  30,
				},
				"keys": []interface{}{"password", "token"},
			},
			wantErr:  false,
			wantKeys: []string{"name", "age"},
			wantValues: map[string]interface{}{
				"name": "John",
				"age":  30,
			},
		},
		{
			name: "omit with empty keys array returns all",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"name": "John",
					"age":  30,
				},
				"keys": []interface{}{},
			},
			wantErr:  false,
			wantKeys: []string{"name", "age"},
		},
		{
			name: "omit all keys returns empty object",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{
					"name": "John",
					"age":  30,
				},
				"keys": []interface{}{"name", "age"},
			},
			wantErr:    false,
			wantKeys:   []string{},
			wantValues: map[string]interface{}{},
		},
		{
			name:    "missing data parameter",
			inputs:  map[string]interface{}{"keys": []interface{}{"name"}},
			wantErr: true,
			errType: ErrorTypeValidation,
		},
		{
			name: "missing keys parameter",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"name": "John"},
			},
			wantErr: true,
			errType: ErrorTypeValidation,
		},
		{
			name: "null data",
			inputs: map[string]interface{}{
				"data": nil,
				"keys": []interface{}{"name"},
			},
			wantErr: true,
			errType: ErrorTypeEmptyInput,
		},
		{
			name: "non-object data",
			inputs: map[string]interface{}{
				"data": "not an object",
				"keys": []interface{}{"name"},
			},
			wantErr: true,
			errType: ErrorTypeTypeError,
		},
		{
			name: "non-array keys",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"name": "John"},
				"keys": "password",
			},
			wantErr: true,
			errType: ErrorTypeTypeError,
		},
		{
			name: "non-string key in array",
			inputs: map[string]interface{}{
				"data": map[string]interface{}{"name": "John"},
				"keys": []interface{}{"name", 456},
			},
			wantErr: true,
			errType: ErrorTypeTypeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conn.Execute(context.Background(), "omit", tt.inputs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("omit() expected error, got nil")
					return
				}
				opErr, ok := err.(*OperationError)
				if !ok {
					t.Errorf("omit() error type = %T, want *OperationError", err)
					return
				}
				if opErr.ErrorType != tt.errType {
					t.Errorf("omit() error type = %v, want %v", opErr.ErrorType, tt.errType)
				}
				return
			}

			if err != nil {
				t.Errorf("omit() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Error("omit() returned nil result")
				return
			}

			response, ok := result.Response.(map[string]interface{})
			if !ok {
				t.Errorf("omit() response type = %T, want map[string]interface{}", result.Response)
				return
			}

			// Check that only expected keys are present
			if len(response) != len(tt.wantKeys) {
				t.Errorf("omit() response has %d keys, want %d", len(response), len(tt.wantKeys))
			}

			for _, key := range tt.wantKeys {
				if _, exists := response[key]; !exists {
					t.Errorf("omit() response missing expected key %q", key)
				}
			}

			// Check specific values if provided
			for key, wantValue := range tt.wantValues {
				if gotValue, exists := response[key]; !exists {
					t.Errorf("omit() response missing key %q", key)
				} else if gotValue != wantValue {
					t.Errorf("omit() response[%q] = %v, want %v", key, gotValue, wantValue)
				}
			}

			// Check metadata
			if result.Metadata == nil {
				t.Error("omit() metadata is nil")
				return
			}
			if _, ok := result.Metadata["keys_to_omit"]; !ok {
				t.Error("omit() metadata missing keys_to_omit")
			}
			if _, ok := result.Metadata["keys_omitted"]; !ok {
				t.Error("omit() metadata missing keys_omitted")
			}
			if _, ok := result.Metadata["keys_retained"]; !ok {
				t.Error("omit() metadata missing keys_retained")
			}
		})
	}
}
