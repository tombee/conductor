package utility

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		uc, err := New(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if uc == nil {
			t.Fatal("expected action, got nil")
		}
		if uc.Name() != "utility" {
			t.Errorf("expected name 'utility', got %q", uc.Name())
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		seed := int64(42)
		cfg := &Config{
			RandomSeed:          &seed,
			MaxArraySize:        100,
			MaxIDLength:         64,
			DefaultNanoidLength: 10,
		}
		uc, err := New(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if uc == nil {
			t.Fatal("expected action, got nil")
		}
	})
}

func TestExecuteUnknownOperation(t *testing.T) {
	uc, _ := New(nil)
	_, err := uc.Execute(context.Background(), "unknown_op", nil)
	if err == nil {
		t.Fatal("expected error for unknown operation")
	}

	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("expected OperationError, got %T", err)
	}
	if opErr.ErrorType != ErrorTypeValidation {
		t.Errorf("expected ErrorTypeValidation, got %v", opErr.ErrorType)
	}
}

func TestOperations(t *testing.T) {
	uc, _ := New(nil)
	ops := uc.Operations()
	expected := []string{
		"random_int", "random_choose", "random_weighted", "random_sample", "random_shuffle",
		"id_uuid", "id_nanoid", "id_custom",
		"math_clamp", "math_round", "math_min", "math_max",
		"timestamp", "sleep",
	}

	if len(ops) != len(expected) {
		t.Errorf("expected %d operations, got %d", len(expected), len(ops))
	}

	opSet := make(map[string]bool)
	for _, op := range ops {
		opSet[op] = true
	}

	for _, exp := range expected {
		if !opSet[exp] {
			t.Errorf("missing expected operation: %s", exp)
		}
	}
}
