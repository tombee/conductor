package utility

import (
	"context"
	"math"
	"testing"
)

func TestMathClamp(t *testing.T) {
	uc, _ := New(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{"value in range", 50, 0, 100, 50},
		{"value below min", -10, 0, 100, 0},
		{"value above max", 150, 0, 100, 100},
		{"value equals min", 0, 0, 100, 0},
		{"value equals max", 100, 0, 100, 100},
		{"negative range", -50, -100, -10, -50},
		{"value below negative range", -150, -100, -10, -100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := uc.Execute(ctx, "math_clamp", map[string]interface{}{
				"value": tc.value,
				"min":   tc.min,
				"max":   tc.max,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := result.Response.(float64)
			if got != tc.expected {
				t.Errorf("expected %f, got %f", tc.expected, got)
			}
		})
	}

	t.Run("min greater than max returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_clamp", map[string]interface{}{
			"value": 50,
			"min":   100,
			"max":   0,
		})
		if err == nil {
			t.Fatal("expected error when min > max")
		}
	})

	t.Run("NaN value returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_clamp", map[string]interface{}{
			"value": math.NaN(),
			"min":   0,
			"max":   100,
		})
		if err == nil {
			t.Fatal("expected error for NaN value")
		}
	})

	t.Run("missing parameters returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_clamp", map[string]interface{}{
			"value": 50,
		})
		if err == nil {
			t.Fatal("expected error for missing min/max")
		}
	})
}

func TestMathRound(t *testing.T) {
	uc, _ := New(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		value    float64
		decimals int
		expected float64
	}{
		{"round to integer", 3.7, 0, 4},
		{"round down to integer", 3.2, 0, 3},
		{"round to 1 decimal", 3.14159, 1, 3.1},
		{"round to 2 decimals", 3.14159, 2, 3.14},
		{"round to 3 decimals", 3.14159, 3, 3.142},
		{"round negative", -2.7, 0, -3},
		{"round 0.5 up", 0.5, 0, 1}, // Go rounds ties up (away from zero is implementation dependent)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := uc.Execute(ctx, "math_round", map[string]interface{}{
				"value":    tc.value,
				"decimals": tc.decimals,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := result.Response.(float64)
			if got != tc.expected {
				t.Errorf("expected %f, got %f", tc.expected, got)
			}
		})
	}

	t.Run("default decimals is 0", func(t *testing.T) {
		result, err := uc.Execute(ctx, "math_round", map[string]interface{}{
			"value": 3.7,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := result.Response.(float64)
		if got != 4 {
			t.Errorf("expected 4, got %f", got)
		}
	})

	t.Run("negative decimals returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_round", map[string]interface{}{
			"value":    3.14,
			"decimals": -1,
		})
		if err == nil {
			t.Fatal("expected error for negative decimals")
		}
	})

	t.Run("exceeds max decimals returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_round", map[string]interface{}{
			"value":    3.14,
			"decimals": 20,
		})
		if err == nil {
			t.Fatal("expected error for exceeding max decimals")
		}
	})

	t.Run("NaN returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_round", map[string]interface{}{
			"value": math.NaN(),
		})
		if err == nil {
			t.Fatal("expected error for NaN")
		}
	})

	t.Run("Infinity returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_round", map[string]interface{}{
			"value": math.Inf(1),
		})
		if err == nil {
			t.Fatal("expected error for Infinity")
		}
	})
}

func TestMathMin(t *testing.T) {
	uc, _ := New(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		values   []interface{}
		expected float64
	}{
		{"single value", []interface{}{5}, 5},
		{"two values", []interface{}{10, 5}, 5},
		{"multiple values", []interface{}{10, 5, 8, 3, 12}, 3},
		{"negative values", []interface{}{-5, -10, -3}, -10},
		{"mixed values", []interface{}{-5, 0, 10}, -5},
		{"decimals", []interface{}{3.14, 2.71, 1.41}, 1.41},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := uc.Execute(ctx, "math_min", map[string]interface{}{
				"values": tc.values,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := result.Response.(float64)
			if got != tc.expected {
				t.Errorf("expected %f, got %f", tc.expected, got)
			}
		})
	}

	t.Run("empty values returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_min", map[string]interface{}{
			"values": []interface{}{},
		})
		if err == nil {
			t.Fatal("expected error for empty values")
		}
	})

	t.Run("NaN in values returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_min", map[string]interface{}{
			"values": []interface{}{1, math.NaN(), 3},
		})
		if err == nil {
			t.Fatal("expected error for NaN in values")
		}
	})

	t.Run("non-numeric value returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_min", map[string]interface{}{
			"values": []interface{}{1, "not a number", 3},
		})
		if err == nil {
			t.Fatal("expected error for non-numeric value")
		}
	})
}

func TestMathMax(t *testing.T) {
	uc, _ := New(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		values   []interface{}
		expected float64
	}{
		{"single value", []interface{}{5}, 5},
		{"two values", []interface{}{10, 5}, 10},
		{"multiple values", []interface{}{10, 5, 8, 3, 12}, 12},
		{"negative values", []interface{}{-5, -10, -3}, -3},
		{"mixed values", []interface{}{-5, 0, 10}, 10},
		{"decimals", []interface{}{3.14, 2.71, 1.41}, 3.14},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := uc.Execute(ctx, "math_max", map[string]interface{}{
				"values": tc.values,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := result.Response.(float64)
			if got != tc.expected {
				t.Errorf("expected %f, got %f", tc.expected, got)
			}
		})
	}

	t.Run("empty values returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_max", map[string]interface{}{
			"values": []interface{}{},
		})
		if err == nil {
			t.Fatal("expected error for empty values")
		}
	})

	t.Run("NaN in values returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "math_max", map[string]interface{}{
			"values": []interface{}{1, math.NaN(), 3},
		})
		if err == nil {
			t.Fatal("expected error for NaN in values")
		}
	})
}
