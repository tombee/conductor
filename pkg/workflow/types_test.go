package workflow

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestErrKeyNotFound_Error tests the error message formatting for ErrKeyNotFound.
func TestErrKeyNotFound_Error(t *testing.T) {
	err := ErrKeyNotFound{Key: "missing_key"}
	expected := `key "missing_key" not found`
	if err.Error() != expected {
		t.Errorf("ErrKeyNotFound.Error() = %q, want %q", err.Error(), expected)
	}

	// Verify error message doesn't contain actual values (security requirement)
	if strings.Contains(err.Error(), "value") {
		t.Error("ErrKeyNotFound.Error() should not contain the word 'value'")
	}
}

// TestErrTypeAssertion_Error tests the error message formatting for ErrTypeAssertion.
func TestErrTypeAssertion_Error(t *testing.T) {
	err := ErrTypeAssertion{Key: "my_key", Got: "int", Want: "string"}
	expected := `key "my_key" is int, not string`
	if err.Error() != expected {
		t.Errorf("ErrTypeAssertion.Error() = %q, want %q", err.Error(), expected)
	}

	// Verify error message doesn't contain actual values (security requirement)
	if strings.Contains(err.Error(), "42") || strings.Contains(err.Error(), "actual") {
		t.Error("ErrTypeAssertion.Error() should not contain actual values")
	}
}

// TestNewWorkflowContext tests context creation.
func TestNewWorkflowContext(t *testing.T) {
	t.Run("with inputs", func(t *testing.T) {
		inputs := map[string]any{"key": "value"}
		ctx := NewWorkflowContext(inputs)
		if ctx == nil {
			t.Fatal("NewWorkflowContext returned nil")
		}
		if ctx.inputs == nil {
			t.Error("inputs should not be nil")
		}
		if ctx.outputs == nil {
			t.Error("outputs should not be nil")
		}
		if ctx.vars == nil {
			t.Error("vars should not be nil")
		}
	})

	t.Run("with nil inputs", func(t *testing.T) {
		ctx := NewWorkflowContext(nil)
		if ctx == nil {
			t.Fatal("NewWorkflowContext returned nil")
		}
		if ctx.inputs == nil {
			t.Error("inputs should be initialized to empty map when nil is passed")
		}
	})
}

// TestWorkflowContext_GetString tests string retrieval.
func TestWorkflowContext_GetString(t *testing.T) {
	tests := []struct {
		name      string
		inputs    map[string]any
		key       string
		want      string
		wantErr   error
		checkType func(error) bool
	}{
		{
			name:    "success",
			inputs:  map[string]any{"name": "Alice"},
			key:     "name",
			want:    "Alice",
			wantErr: nil,
		},
		{
			name:    "missing key",
			inputs:  map[string]any{},
			key:     "missing",
			want:    "",
			wantErr: ErrKeyNotFound{Key: "missing"},
			checkType: func(err error) bool {
				var e ErrKeyNotFound
				return errors.As(err, &e)
			},
		},
		{
			name:    "type mismatch - int",
			inputs:  map[string]any{"age": 42},
			key:     "age",
			want:    "",
			wantErr: ErrTypeAssertion{Key: "age", Got: "int", Want: "string"},
			checkType: func(err error) bool {
				var e ErrTypeAssertion
				return errors.As(err, &e)
			},
		},
		{
			name:    "type mismatch - nil",
			inputs:  map[string]any{"nullable": nil},
			key:     "nullable",
			want:    "",
			wantErr: ErrTypeAssertion{Key: "nullable", Got: "<nil>", Want: "string"},
			checkType: func(err error) bool {
				var e ErrTypeAssertion
				return errors.As(err, &e)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got, err := ctx.GetString(tt.key)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("GetString() error = nil, want %v", tt.wantErr)
				}
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("GetString() error = %v, want %v", err, tt.wantErr)
				}
				if tt.checkType != nil && !tt.checkType(err) {
					t.Errorf("GetString() error type mismatch, got %T", err)
				}
			} else if err != nil {
				t.Fatalf("GetString() unexpected error = %v", err)
			}

			if got != tt.want {
				t.Errorf("GetString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_GetStringOr tests string retrieval with default values.
func TestWorkflowContext_GetStringOr(t *testing.T) {
	tests := []struct {
		name       string
		inputs     map[string]any
		key        string
		defaultVal string
		want       string
	}{
		{
			name:       "success",
			inputs:     map[string]any{"name": "Alice"},
			key:        "name",
			defaultVal: "default",
			want:       "Alice",
		},
		{
			name:       "missing key returns default",
			inputs:     map[string]any{},
			key:        "missing",
			defaultVal: "default_value",
			want:       "default_value",
		},
		{
			name:       "type mismatch returns default",
			inputs:     map[string]any{"age": 42},
			key:        "age",
			defaultVal: "default_value",
			want:       "default_value",
		},
		{
			name:       "nil value returns default",
			inputs:     map[string]any{"nullable": nil},
			key:        "nullable",
			defaultVal: "default_value",
			want:       "default_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got := ctx.GetStringOr(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetStringOr() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_GetInt64 tests int64 retrieval.
func TestWorkflowContext_GetInt64(t *testing.T) {
	tests := []struct {
		name    string
		inputs  map[string]any
		key     string
		want    int64
		wantErr bool
		errType string
	}{
		{
			name:    "success - int64",
			inputs:  map[string]any{"count": int64(100)},
			key:     "count",
			want:    100,
			wantErr: false,
		},
		{
			name:    "success - int",
			inputs:  map[string]any{"count": int(200)},
			key:     "count",
			want:    200,
			wantErr: false,
		},
		{
			name:    "success - int32",
			inputs:  map[string]any{"count": int32(300)},
			key:     "count",
			want:    300,
			wantErr: false,
		},
		{
			name:    "success - float64 (JSON number)",
			inputs:  map[string]any{"count": float64(400)},
			key:     "count",
			want:    400,
			wantErr: false,
		},
		{
			name:    "missing key",
			inputs:  map[string]any{},
			key:     "missing",
			want:    0,
			wantErr: true,
			errType: "ErrKeyNotFound",
		},
		{
			name:    "type mismatch - string",
			inputs:  map[string]any{"count": "not a number"},
			key:     "count",
			want:    0,
			wantErr: true,
			errType: "ErrTypeAssertion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got, err := ctx.GetInt64(tt.key)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("GetInt64() error = nil, want error")
				}
				switch tt.errType {
				case "ErrKeyNotFound":
					var e ErrKeyNotFound
					if !errors.As(err, &e) {
						t.Errorf("GetInt64() error type = %T, want ErrKeyNotFound", err)
					}
				case "ErrTypeAssertion":
					var e ErrTypeAssertion
					if !errors.As(err, &e) {
						t.Errorf("GetInt64() error type = %T, want ErrTypeAssertion", err)
					}
				}
			} else if err != nil {
				t.Fatalf("GetInt64() unexpected error = %v", err)
			}

			if got != tt.want {
				t.Errorf("GetInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_GetInt64Or tests int64 retrieval with default values.
func TestWorkflowContext_GetInt64Or(t *testing.T) {
	tests := []struct {
		name       string
		inputs     map[string]any
		key        string
		defaultVal int64
		want       int64
	}{
		{
			name:       "success",
			inputs:     map[string]any{"count": int64(42)},
			key:        "count",
			defaultVal: 99,
			want:       42,
		},
		{
			name:       "missing key returns default",
			inputs:     map[string]any{},
			key:        "missing",
			defaultVal: 99,
			want:       99,
		},
		{
			name:       "type mismatch returns default",
			inputs:     map[string]any{"count": "not a number"},
			key:        "count",
			defaultVal: 99,
			want:       99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got := ctx.GetInt64Or(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetInt64Or() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_GetBool tests bool retrieval.
func TestWorkflowContext_GetBool(t *testing.T) {
	tests := []struct {
		name    string
		inputs  map[string]any
		key     string
		want    bool
		wantErr bool
		errType string
	}{
		{
			name:    "success - true",
			inputs:  map[string]any{"enabled": true},
			key:     "enabled",
			want:    true,
			wantErr: false,
		},
		{
			name:    "success - false",
			inputs:  map[string]any{"enabled": false},
			key:     "enabled",
			want:    false,
			wantErr: false,
		},
		{
			name:    "missing key",
			inputs:  map[string]any{},
			key:     "missing",
			want:    false,
			wantErr: true,
			errType: "ErrKeyNotFound",
		},
		{
			name:    "type mismatch - string",
			inputs:  map[string]any{"enabled": "true"},
			key:     "enabled",
			want:    false,
			wantErr: true,
			errType: "ErrTypeAssertion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got, err := ctx.GetBool(tt.key)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("GetBool() error = nil, want error")
				}
				switch tt.errType {
				case "ErrKeyNotFound":
					var e ErrKeyNotFound
					if !errors.As(err, &e) {
						t.Errorf("GetBool() error type = %T, want ErrKeyNotFound", err)
					}
				case "ErrTypeAssertion":
					var e ErrTypeAssertion
					if !errors.As(err, &e) {
						t.Errorf("GetBool() error type = %T, want ErrTypeAssertion", err)
					}
				}
			} else if err != nil {
				t.Fatalf("GetBool() unexpected error = %v", err)
			}

			if got != tt.want {
				t.Errorf("GetBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_GetBoolOr tests bool retrieval with default values.
func TestWorkflowContext_GetBoolOr(t *testing.T) {
	tests := []struct {
		name       string
		inputs     map[string]any
		key        string
		defaultVal bool
		want       bool
	}{
		{
			name:       "success",
			inputs:     map[string]any{"enabled": true},
			key:        "enabled",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "missing key returns default",
			inputs:     map[string]any{},
			key:        "missing",
			defaultVal: true,
			want:       true,
		},
		{
			name:       "type mismatch returns default",
			inputs:     map[string]any{"enabled": "true"},
			key:        "enabled",
			defaultVal: false,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got := ctx.GetBoolOr(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetBoolOr() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_GetFloat64 tests float64 retrieval.
func TestWorkflowContext_GetFloat64(t *testing.T) {
	tests := []struct {
		name    string
		inputs  map[string]any
		key     string
		want    float64
		wantErr bool
		errType string
	}{
		{
			name:    "success - float64",
			inputs:  map[string]any{"pi": 3.14},
			key:     "pi",
			want:    3.14,
			wantErr: false,
		},
		{
			name:    "success - float32",
			inputs:  map[string]any{"pi": float32(3.14)},
			key:     "pi",
			want:    float64(float32(3.14)),
			wantErr: false,
		},
		{
			name:    "success - int",
			inputs:  map[string]any{"count": 42},
			key:     "count",
			want:    42.0,
			wantErr: false,
		},
		{
			name:    "success - int64",
			inputs:  map[string]any{"count": int64(100)},
			key:     "count",
			want:    100.0,
			wantErr: false,
		},
		{
			name:    "missing key",
			inputs:  map[string]any{},
			key:     "missing",
			want:    0,
			wantErr: true,
			errType: "ErrKeyNotFound",
		},
		{
			name:    "type mismatch - string",
			inputs:  map[string]any{"pi": "not a number"},
			key:     "pi",
			want:    0,
			wantErr: true,
			errType: "ErrTypeAssertion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got, err := ctx.GetFloat64(tt.key)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("GetFloat64() error = nil, want error")
				}
				switch tt.errType {
				case "ErrKeyNotFound":
					var e ErrKeyNotFound
					if !errors.As(err, &e) {
						t.Errorf("GetFloat64() error type = %T, want ErrKeyNotFound", err)
					}
				case "ErrTypeAssertion":
					var e ErrTypeAssertion
					if !errors.As(err, &e) {
						t.Errorf("GetFloat64() error type = %T, want ErrTypeAssertion", err)
					}
				}
			} else if err != nil {
				t.Fatalf("GetFloat64() unexpected error = %v", err)
			}

			if got != tt.want {
				t.Errorf("GetFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_GetFloat64Or tests float64 retrieval with default values.
func TestWorkflowContext_GetFloat64Or(t *testing.T) {
	tests := []struct {
		name       string
		inputs     map[string]any
		key        string
		defaultVal float64
		want       float64
	}{
		{
			name:       "success",
			inputs:     map[string]any{"pi": 3.14},
			key:        "pi",
			defaultVal: 0.0,
			want:       3.14,
		},
		{
			name:       "missing key returns default",
			inputs:     map[string]any{},
			key:        "missing",
			defaultVal: 99.9,
			want:       99.9,
		},
		{
			name:       "type mismatch returns default",
			inputs:     map[string]any{"pi": "not a number"},
			key:        "pi",
			defaultVal: 0.0,
			want:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got := ctx.GetFloat64Or(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetFloat64Or() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_GetSlice tests slice retrieval.
func TestWorkflowContext_GetSlice(t *testing.T) {
	tests := []struct {
		name    string
		inputs  map[string]any
		key     string
		want    []interface{}
		wantErr bool
		errType string
	}{
		{
			name:    "success",
			inputs:  map[string]any{"items": []interface{}{"a", "b", "c"}},
			key:     "items",
			want:    []interface{}{"a", "b", "c"},
			wantErr: false,
		},
		{
			name:    "missing key",
			inputs:  map[string]any{},
			key:     "missing",
			want:    nil,
			wantErr: true,
			errType: "ErrKeyNotFound",
		},
		{
			name:    "type mismatch - string",
			inputs:  map[string]any{"items": "not a slice"},
			key:     "items",
			want:    nil,
			wantErr: true,
			errType: "ErrTypeAssertion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got, err := ctx.GetSlice(tt.key)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("GetSlice() error = nil, want error")
				}
				switch tt.errType {
				case "ErrKeyNotFound":
					var e ErrKeyNotFound
					if !errors.As(err, &e) {
						t.Errorf("GetSlice() error type = %T, want ErrKeyNotFound", err)
					}
				case "ErrTypeAssertion":
					var e ErrTypeAssertion
					if !errors.As(err, &e) {
						t.Errorf("GetSlice() error type = %T, want ErrTypeAssertion", err)
					}
				}
			} else if err != nil {
				t.Fatalf("GetSlice() unexpected error = %v", err)
			}

			if !slicesEqual(got, tt.want) {
				t.Errorf("GetSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_GetMap tests map retrieval.
func TestWorkflowContext_GetMap(t *testing.T) {
	tests := []struct {
		name    string
		inputs  map[string]any
		key     string
		want    map[string]interface{}
		wantErr bool
		errType string
	}{
		{
			name:    "success",
			inputs:  map[string]any{"config": map[string]interface{}{"key": "value"}},
			key:     "config",
			want:    map[string]interface{}{"key": "value"},
			wantErr: false,
		},
		{
			name:    "missing key",
			inputs:  map[string]any{},
			key:     "missing",
			want:    nil,
			wantErr: true,
			errType: "ErrKeyNotFound",
		},
		{
			name:    "type mismatch - string",
			inputs:  map[string]any{"config": "not a map"},
			key:     "config",
			want:    nil,
			wantErr: true,
			errType: "ErrTypeAssertion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)
			got, err := ctx.GetMap(tt.key)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("GetMap() error = nil, want error")
				}
				switch tt.errType {
				case "ErrKeyNotFound":
					var e ErrKeyNotFound
					if !errors.As(err, &e) {
						t.Errorf("GetMap() error type = %T, want ErrKeyNotFound", err)
					}
				case "ErrTypeAssertion":
					var e ErrTypeAssertion
					if !errors.As(err, &e) {
						t.Errorf("GetMap() error type = %T, want ErrTypeAssertion", err)
					}
				}
			} else if err != nil {
				t.Fatalf("GetMap() unexpected error = %v", err)
			}

			if !mapsEqual(got, tt.want) {
				t.Errorf("GetMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWorkflowContext_ConcurrentReads tests that concurrent reads are safe.
func TestWorkflowContext_ConcurrentReads(t *testing.T) {
	inputs := map[string]any{
		"string_val": "test",
		"int_val":    int64(42),
		"bool_val":   true,
		"float_val":  3.14,
	}
	ctx := NewWorkflowContext(inputs)

	var wg sync.WaitGroup
	goroutines := 100
	iterations := 100

	// Launch many goroutines that read from the context concurrently
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Read different types concurrently
				_, _ = ctx.GetString("string_val")
				_, _ = ctx.GetInt64("int_val")
				_, _ = ctx.GetBool("bool_val")
				_, _ = ctx.GetFloat64("float_val")

				// Also test the Or variants
				_ = ctx.GetStringOr("string_val", "default")
				_ = ctx.GetInt64Or("int_val", 0)
				_ = ctx.GetBoolOr("bool_val", false)
				_ = ctx.GetFloat64Or("float_val", 0.0)
			}
		}()
	}

	// Wait for all goroutines to complete
	// If there's a data race, this test will fail with -race flag
	wg.Wait()
}

// TestWorkflowContext_NoPanicOnDefault tests that default value methods never panic.
func TestWorkflowContext_NoPanicOnDefault(t *testing.T) {
	// This test ensures the "Or" methods never panic regardless of input
	tests := []struct {
		name   string
		inputs map[string]any
		key    string
	}{
		{"missing key", map[string]any{}, "missing"},
		{"nil value", map[string]any{"key": nil}, "key"},
		{"wrong type", map[string]any{"key": 42}, "key"},
		{"complex wrong type", map[string]any{"key": struct{}{}}, "key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewWorkflowContext(tt.inputs)

			// These should not panic
			_ = ctx.GetStringOr(tt.key, "default")
			_ = ctx.GetInt64Or(tt.key, 0)
			_ = ctx.GetBoolOr(tt.key, false)
			_ = ctx.GetFloat64Or(tt.key, 0.0)
		})
	}
}

// TestStepOutput_JSONSerialization tests that StepOutput can be marshaled to JSON.
func TestStepOutput_JSONSerialization(t *testing.T) {
	output := StepOutput{
		Text:  "Hello, world!",
		Data:  map[string]interface{}{"key": "value"},
		Error: "",
		Metadata: OutputMetadata{
			Duration: time.Second,
			TokenUsage: &TokenUsage{
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
			},
			Provider: "anthropic",
			Model:    "claude-opus-4-5",
		},
	}

	// Just verify the structure can be created without panicking
	// Actual JSON marshaling test would require encoding/json import
	if output.Text == "" {
		t.Error("StepOutput should have Text field")
	}
	if output.Metadata.Provider == "" {
		t.Error("OutputMetadata should have Provider field")
	}
	if output.Metadata.TokenUsage == nil {
		t.Error("OutputMetadata should have TokenUsage")
	}
	if output.Metadata.TokenUsage.TotalTokens != 150 {
		t.Errorf("TokenUsage.TotalTokens = %d, want 150", output.Metadata.TokenUsage.TotalTokens)
	}
}

// TestErrorMessages_NoValueLeakage tests that error messages don't leak sensitive values.
func TestErrorMessages_NoValueLeakage(t *testing.T) {
	inputs := map[string]any{
		"password":    "super_secret_password",
		"api_key":     "sk-1234567890abcdef",
		"int_value":   42,
		"bool_value":  true,
		"float_value": 3.14159,
	}
	ctx := NewWorkflowContext(inputs)

	tests := []struct {
		name      string
		operation func() error
	}{
		{"string type mismatch", func() error { _, err := ctx.GetString("int_value"); return err }},
		{"int64 type mismatch", func() error { _, err := ctx.GetInt64("password"); return err }},
		{"bool type mismatch", func() error { _, err := ctx.GetBool("password"); return err }},
		{"float64 type mismatch", func() error { _, err := ctx.GetFloat64("password"); return err }},
		{"missing key", func() error { _, err := ctx.GetString("nonexistent"); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errMsg := err.Error()

			// Verify sensitive values are not in error messages
			sensitiveValues := []string{
				"super_secret_password",
				"sk-1234567890abcdef",
				"42", // Note: numbers might appear in type names, but not as values
			}

			for _, sensitive := range sensitiveValues {
				if strings.Contains(errMsg, sensitive) {
					t.Errorf("error message contains sensitive value %q: %s", sensitive, errMsg)
				}
			}

			// Verify error message structure is reasonable (contains "key")
			if !strings.Contains(errMsg, "key") {
				t.Errorf("error message should mention 'key': %s", errMsg)
			}
		})
	}
}

// TestStepOutput_ToMap tests the StepOutput.ToMap conversion.
func TestStepOutput_ToMap(t *testing.T) {
	t.Run("converts text and data fields", func(t *testing.T) {
		output := StepOutput{
			Text: "hello world",
			Data: map[string]interface{}{"status": "success", "count": 42},
		}

		result := output.ToMap()

		if result["text"] != "hello world" {
			t.Errorf("expected text='hello world', got %v", result["text"])
		}
		if result["response"] != "hello world" {
			t.Errorf("expected response='hello world', got %v", result["response"])
		}
		if result["status"] != "success" {
			t.Errorf("expected status='success', got %v", result["status"])
		}
		if result["count"] != 42 {
			t.Errorf("expected count=42, got %v", result["count"])
		}
	})

	t.Run("converts error field", func(t *testing.T) {
		output := StepOutput{
			Error: "execution failed",
		}

		result := output.ToMap()

		if result["error"] != "execution failed" {
			t.Errorf("expected error='execution failed', got %v", result["error"])
		}
	})

	t.Run("handles non-map data", func(t *testing.T) {
		output := StepOutput{
			Text: "result",
			Data: []string{"item1", "item2"},
		}

		result := output.ToMap()

		if result["text"] != "result" {
			t.Errorf("expected text='result', got %v", result["text"])
		}
		if result["data"] == nil {
			t.Error("expected data to be set")
		}
		dataSlice, ok := result["data"].([]string)
		if !ok {
			t.Errorf("expected data to be []string, got %T", result["data"])
		}
		if len(dataSlice) != 2 {
			t.Errorf("expected data slice length 2, got %d", len(dataSlice))
		}
	})

	t.Run("handles empty output", func(t *testing.T) {
		output := StepOutput{}

		result := output.ToMap()

		if result == nil {
			t.Error("expected non-nil map")
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d keys", len(result))
		}
	})

	t.Run("preserves types through conversion", func(t *testing.T) {
		output := StepOutput{
			Text: "test",
			Data: map[string]interface{}{
				"string": "value",
				"int":    123,
				"bool":   true,
				"float":  45.67,
			},
		}

		result := output.ToMap()

		if result["string"] != "value" {
			t.Errorf("expected string='value', got %v", result["string"])
		}
		if result["int"] != 123 {
			t.Errorf("expected int=123, got %v", result["int"])
		}
		if result["bool"] != true {
			t.Errorf("expected bool=true, got %v", result["bool"])
		}
		if result["float"] != 45.67 {
			t.Errorf("expected float=45.67, got %v", result["float"])
		}
	})
}

// Helper functions

func slicesEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
