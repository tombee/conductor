package transform

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
		{
			name:    "empty config gets defaults applied",
			config:  &Config{},
			wantErr: false,
		},
		{
			name: "custom config",
			config: &Config{
				MaxInputSize:  5 * 1024 * 1024,
				MaxArrayItems: 5000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if conn == nil {
					t.Error("New() returned nil connector")
				}
				if conn.Name() != "transform" {
					t.Errorf("Name() = %v, want transform", conn.Name())
				}
			}
		})
	}
}

func TestTransformConnector_Execute_UnknownOperation(t *testing.T) {
	conn, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = conn.Execute(context.Background(), "unknown_op", map[string]interface{}{})
	if err == nil {
		t.Error("Execute() expected error for unknown operation, got nil")
	}

	opErr, ok := err.(*OperationError)
	if !ok {
		t.Errorf("Execute() error type = %T, want *OperationError", err)
	}
	if ok && opErr.ErrorType != ErrorTypeValidation {
		t.Errorf("Execute() error type = %v, want %v", opErr.ErrorType, ErrorTypeValidation)
	}
}

func TestTransformConnector_Execute_NotImplemented(t *testing.T) {
	conn, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Operations that are not yet implemented
	operations := []string{
		// All operations are now implemented
	}

	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			_, err := conn.Execute(context.Background(), op, map[string]interface{}{})
			if err == nil {
				t.Errorf("Execute(%s) expected not implemented error, got nil", op)
			}

			opErr, ok := err.(*OperationError)
			if !ok {
				t.Errorf("Execute(%s) error type = %T, want *OperationError", op, err)
			}
			if ok && opErr.ErrorType != ErrorTypeInternal {
				t.Errorf("Execute(%s) error type = %v, want %v", op, opErr.ErrorType, ErrorTypeInternal)
			}
		})
	}
}
