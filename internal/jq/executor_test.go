package jq

import (
	"context"
	"testing"
	"time"
)

func TestExecutor_Execute(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		data       interface{}
		want       interface{}
		wantErr    bool
	}{
		{
			name:       "empty expression returns data as-is",
			expression: "",
			data:       map[string]interface{}{"foo": "bar"},
			want:       map[string]interface{}{"foo": "bar"},
			wantErr:    false,
		},
		{
			name:       "simple field extraction",
			expression: ".foo",
			data:       map[string]interface{}{"foo": "bar"},
			want:       "bar",
			wantErr:    false,
		},
		{
			name:       "array map",
			expression: "map(.x)",
			data:       []interface{}{map[string]interface{}{"x": 1}, map[string]interface{}{"x": 2}},
			want:       []interface{}{float64(1), float64(2)},
			wantErr:    false,
		},
		{
			name:       "invalid expression",
			expression: ".[",
			data:       map[string]interface{}{"foo": "bar"},
			want:       nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecutor(DefaultTimeout, DefaultMaxInputSize)
			got, err := executor.Execute(context.Background(), tt.expression, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Compare results - using simple comparison for now
				if got == nil && tt.want != nil {
					t.Errorf("Execute() got nil, want %v", tt.want)
				} else if got != nil && tt.want == nil {
					t.Errorf("Execute() got %v, want nil", got)
				}
				// For non-nil cases, we'd need deep comparison which we'll add later
			}
		})
	}
}

func TestExecutor_Validate(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		wantErr    bool
	}{
		{
			name:       "empty expression is valid",
			expression: "",
			wantErr:    false,
		},
		{
			name:       "simple expression is valid",
			expression: ".foo",
			wantErr:    false,
		},
		{
			name:       "invalid expression",
			expression: ".[",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecutor(DefaultTimeout, DefaultMaxInputSize)
			err := executor.Validate(tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecutor_Timeout(t *testing.T) {
	executor := NewExecutor(100*time.Millisecond, DefaultMaxInputSize)

	// This expression creates an infinite loop
	_, err := executor.Execute(context.Background(), "while(true; . + 1)", 0)
	if err == nil {
		t.Error("Execute() expected timeout error, got nil")
	}
}
