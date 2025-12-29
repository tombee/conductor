package utility

import (
	"context"
	"testing"
)

// newTestAction creates an action with deterministic random for testing.
func newTestAction(seed int64) *UtilityAction {
	cfg := &Config{
		RandomSeed:          &seed,
		MaxArraySize:        100,
		MaxIDLength:         256,
		DefaultNanoidLength: 21,
	}
	action, _ := New(cfg)
	return action
}

func TestRandomInt(t *testing.T) {
	action := newTestAction(42)
	ctx := context.Background()

	t.Run("basic range", func(t *testing.T) {
		result, err := action.Execute(ctx, "random_int", map[string]interface{}{
			"min": 1,
			"max": 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		val, ok := result.Response.(int64)
		if !ok {
			t.Fatalf("expected int64, got %T", result.Response)
		}
		if val < 1 || val > 10 {
			t.Errorf("value %d not in range [1, 10]", val)
		}
	})

	t.Run("min equals max", func(t *testing.T) {
		result, err := action.Execute(ctx, "random_int", map[string]interface{}{
			"min": 5,
			"max": 5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		val := result.Response.(int64)
		if val != 5 {
			t.Errorf("expected 5, got %d", val)
		}
	})

	t.Run("min greater than max returns error", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_int", map[string]interface{}{
			"min": 10,
			"max": 1,
		})
		if err == nil {
			t.Fatal("expected error when min > max")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeRange {
			t.Errorf("expected ErrorTypeRange, got %v", opErr.ErrorType)
		}
	})

	t.Run("missing min parameter", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_int", map[string]interface{}{
			"max": 10,
		})
		if err == nil {
			t.Fatal("expected error when min is missing")
		}
	})

	t.Run("missing max parameter", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_int", map[string]interface{}{
			"min": 1,
		})
		if err == nil {
			t.Fatal("expected error when max is missing")
		}
	})
}

func TestRandomChoose(t *testing.T) {
	action := newTestAction(42)
	ctx := context.Background()

	t.Run("basic selection", func(t *testing.T) {
		items := []interface{}{"a", "b", "c", "d"}
		result, err := action.Execute(ctx, "random_choose", map[string]interface{}{
			"items": items,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		chosen := result.Response.(string)
		found := false
		for _, item := range items {
			if item == chosen {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("chosen item %q not in original items", chosen)
		}
	})

	t.Run("single item", func(t *testing.T) {
		result, err := action.Execute(ctx, "random_choose", map[string]interface{}{
			"items": []interface{}{"only_one"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Response != "only_one" {
			t.Errorf("expected 'only_one', got %v", result.Response)
		}
	})

	t.Run("empty items returns error", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_choose", map[string]interface{}{
			"items": []interface{}{},
		})
		if err == nil {
			t.Fatal("expected error for empty items")
		}

		opErr := err.(*OperationError)
		if opErr.ErrorType != ErrorTypeEmpty {
			t.Errorf("expected ErrorTypeEmpty, got %v", opErr.ErrorType)
		}
	})

	t.Run("missing items parameter", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_choose", map[string]interface{}{})
		if err == nil {
			t.Fatal("expected error when items is missing")
		}
	})
}

func TestRandomWeighted(t *testing.T) {
	action := newTestAction(42)
	ctx := context.Background()

	t.Run("basic weighted selection", func(t *testing.T) {
		result, err := action.Execute(ctx, "random_weighted", map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"value": "common", "weight": 90},
				map[string]interface{}{"value": "rare", "weight": 10},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		chosen := result.Response.(string)
		if chosen != "common" && chosen != "rare" {
			t.Errorf("unexpected result: %v", chosen)
		}
	})

	t.Run("single weight 100%", func(t *testing.T) {
		result, err := action.Execute(ctx, "random_weighted", map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"value": "only", "weight": 100},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Response != "only" {
			t.Errorf("expected 'only', got %v", result.Response)
		}
	})

	t.Run("zero total weight returns error", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_weighted", map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"value": "a", "weight": 0},
				map[string]interface{}{"value": "b", "weight": 0},
			},
		})
		if err == nil {
			t.Fatal("expected error for zero total weight")
		}
	})

	t.Run("negative weight returns error", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_weighted", map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"value": "a", "weight": -10},
			},
		})
		if err == nil {
			t.Fatal("expected error for negative weight")
		}
	})

	t.Run("missing value field returns error", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_weighted", map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"weight": 10},
			},
		})
		if err == nil {
			t.Fatal("expected error for missing value field")
		}
	})
}

func TestRandomSample(t *testing.T) {
	action := newTestAction(42)
	ctx := context.Background()

	t.Run("sample subset", func(t *testing.T) {
		result, err := action.Execute(ctx, "random_sample", map[string]interface{}{
			"items": []interface{}{"a", "b", "c", "d", "e"},
			"count": 3,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sampled := result.Response.([]interface{})
		if len(sampled) != 3 {
			t.Errorf("expected 3 items, got %d", len(sampled))
		}

		// Check no duplicates
		seen := make(map[interface{}]bool)
		for _, item := range sampled {
			if seen[item] {
				t.Errorf("duplicate item in sample: %v", item)
			}
			seen[item] = true
		}
	})

	t.Run("sample all", func(t *testing.T) {
		result, err := action.Execute(ctx, "random_sample", map[string]interface{}{
			"items": []interface{}{"a", "b", "c"},
			"count": 3,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sampled := result.Response.([]interface{})
		if len(sampled) != 3 {
			t.Errorf("expected 3 items, got %d", len(sampled))
		}
	})

	t.Run("count exceeds items returns error", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_sample", map[string]interface{}{
			"items": []interface{}{"a", "b"},
			"count": 5,
		})
		if err == nil {
			t.Fatal("expected error when count > items length")
		}

		opErr := err.(*OperationError)
		if opErr.ErrorType != ErrorTypeRange {
			t.Errorf("expected ErrorTypeRange, got %v", opErr.ErrorType)
		}
	})

	t.Run("count zero returns error", func(t *testing.T) {
		_, err := action.Execute(ctx, "random_sample", map[string]interface{}{
			"items": []interface{}{"a", "b"},
			"count": 0,
		})
		if err == nil {
			t.Fatal("expected error when count is 0")
		}
	})
}

func TestRandomShuffle(t *testing.T) {
	action := newTestAction(42)
	ctx := context.Background()

	t.Run("shuffle array", func(t *testing.T) {
		original := []interface{}{"a", "b", "c", "d", "e"}
		result, err := action.Execute(ctx, "random_shuffle", map[string]interface{}{
			"items": original,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		shuffled := result.Response.([]interface{})
		if len(shuffled) != len(original) {
			t.Errorf("expected %d items, got %d", len(original), len(shuffled))
		}

		// Check all elements are present
		origSet := make(map[interface{}]bool)
		for _, item := range original {
			origSet[item] = true
		}
		for _, item := range shuffled {
			if !origSet[item] {
				t.Errorf("shuffled contains unexpected item: %v", item)
			}
		}
	})

	t.Run("shuffle empty array", func(t *testing.T) {
		result, err := action.Execute(ctx, "random_shuffle", map[string]interface{}{
			"items": []interface{}{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		shuffled := result.Response.([]interface{})
		if len(shuffled) != 0 {
			t.Errorf("expected 0 items, got %d", len(shuffled))
		}
	})

	t.Run("shuffle single item", func(t *testing.T) {
		result, err := action.Execute(ctx, "random_shuffle", map[string]interface{}{
			"items": []interface{}{"only"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		shuffled := result.Response.([]interface{})
		if len(shuffled) != 1 || shuffled[0] != "only" {
			t.Errorf("expected ['only'], got %v", shuffled)
		}
	})
}

func TestDeterministicRandomness(t *testing.T) {
	// Two actions with same seed should produce same results
	uc1 := newTestAction(12345)
	uc2 := newTestAction(12345)
	ctx := context.Background()

	inputs := map[string]interface{}{
		"min": 1,
		"max": 1000000,
	}

	result1, _ := uc1.Execute(ctx, "random_int", inputs)
	result2, _ := uc2.Execute(ctx, "random_int", inputs)

	if result1.Response != result2.Response {
		t.Errorf("deterministic sources should produce same results: %v vs %v",
			result1.Response, result2.Response)
	}
}
