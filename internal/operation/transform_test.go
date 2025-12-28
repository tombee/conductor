package operation

import (
	"testing"
)

func TestTransformResponse(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		response   interface{}
		want       interface{}
		wantError  bool
	}{
		{
			name:       "no transform",
			expression: "",
			response:   map[string]interface{}{"id": 123, "name": "test"},
			want:       map[string]interface{}{"id": 123, "name": "test"},
			wantError:  false,
		},
		{
			name:       "extract field",
			expression: ".id",
			response:   map[string]interface{}{"id": float64(123), "name": "test"},
			want:       float64(123),
			wantError:  false,
		},
		{
			name:       "extract nested field",
			expression: ".user.email",
			response:   map[string]interface{}{"user": map[string]interface{}{"email": "test@example.com"}},
			want:       "test@example.com",
			wantError:  false,
		},
		{
			name:       "array first element",
			expression: ".[0]",
			response:   []interface{}{"first", "second", "third"},
			want:       "first",
			wantError:  false,
		},
		{
			name:       "select fields",
			expression: "{id, name}",
			response:   map[string]interface{}{"id": float64(123), "name": "test", "extra": "ignore"},
			want:       map[string]interface{}{"id": float64(123), "name": "test"},
			wantError:  false,
		},
		{
			name:       "map array",
			expression: "[.[] | {name}]",
			response:   []interface{}{map[string]interface{}{"name": "a", "x": 1}, map[string]interface{}{"name": "b", "x": 2}},
			want:       []interface{}{map[string]interface{}{"name": "a"}, map[string]interface{}{"name": "b"}},
			wantError:  false,
		},
		{
			name:       "invalid expression",
			expression: ".{invalid}",
			response:   map[string]interface{}{"id": 123},
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformResponse(tt.expression, tt.response)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Compare results (use string representation for simplicity)
			gotStr := toString(got)
			wantStr := toString(tt.want)

			if gotStr != wantStr {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestValidateTransformExpression(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		wantError  bool
	}{
		{
			name:       "valid expression",
			expression: ".id",
			wantError:  false,
		},
		{
			name:       "valid complex expression",
			expression: "[.[] | select(.active) | {id, name}]",
			wantError:  false,
		},
		{
			name:       "empty expression",
			expression: "",
			wantError:  false,
		},
		{
			name:       "invalid syntax",
			expression: ".{invalid",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransformExpression(tt.expression)

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// toString converts an interface{} to a string for comparison.
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return string(rune(int(val)))
	case map[string]interface{}:
		return "map"
	case []interface{}:
		return "array"
	default:
		return "unknown"
	}
}
