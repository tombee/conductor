package utility

import (
	"context"
	"regexp"
	"testing"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestIDUUID(t *testing.T) {
	uc, _ := New(nil)
	ctx := context.Background()

	t.Run("generates valid UUID v4", func(t *testing.T) {
		result, err := uc.Execute(ctx, "id_uuid", map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		uuid := result.Response.(string)
		if !uuidPattern.MatchString(uuid) {
			t.Errorf("invalid UUID v4 format: %s", uuid)
		}
	})

	t.Run("generates unique UUIDs", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			result, err := uc.Execute(ctx, "id_uuid", map[string]interface{}{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			uuid := result.Response.(string)
			if seen[uuid] {
				t.Errorf("duplicate UUID generated: %s", uuid)
			}
			seen[uuid] = true
		}
	})
}

func TestIDNanoid(t *testing.T) {
	uc, _ := New(nil)
	ctx := context.Background()

	t.Run("generates default length ID", func(t *testing.T) {
		result, err := uc.Execute(ctx, "id_nanoid", map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		id := result.Response.(string)
		if len(id) != 21 { // default length
			t.Errorf("expected length 21, got %d", len(id))
		}
	})

	t.Run("generates custom length ID", func(t *testing.T) {
		result, err := uc.Execute(ctx, "id_nanoid", map[string]interface{}{
			"length": 12,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		id := result.Response.(string)
		if len(id) != 12 {
			t.Errorf("expected length 12, got %d", len(id))
		}
	})

	t.Run("uses URL-safe alphabet", func(t *testing.T) {
		urlSafe := regexp.MustCompile(`^[0-9A-Za-z_-]+$`)

		result, err := uc.Execute(ctx, "id_nanoid", map[string]interface{}{
			"length": 100,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		id := result.Response.(string)
		if !urlSafe.MatchString(id) {
			t.Errorf("ID contains non-URL-safe characters: %s", id)
		}
	})

	t.Run("zero length returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "id_nanoid", map[string]interface{}{
			"length": 0,
		})
		if err == nil {
			t.Fatal("expected error for zero length")
		}
	})

	t.Run("negative length returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "id_nanoid", map[string]interface{}{
			"length": -5,
		})
		if err == nil {
			t.Fatal("expected error for negative length")
		}
	})

	t.Run("exceeds max length returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "id_nanoid", map[string]interface{}{
			"length": 300, // default max is 256
		})
		if err == nil {
			t.Fatal("expected error for exceeding max length")
		}
	})
}

func TestIDCustom(t *testing.T) {
	uc, _ := New(nil)
	ctx := context.Background()

	t.Run("generates ID with custom alphabet", func(t *testing.T) {
		result, err := uc.Execute(ctx, "id_custom", map[string]interface{}{
			"length":   8,
			"alphabet": "ABCD",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		id := result.Response.(string)
		if len(id) != 8 {
			t.Errorf("expected length 8, got %d", len(id))
		}

		for _, ch := range id {
			if ch != 'A' && ch != 'B' && ch != 'C' && ch != 'D' {
				t.Errorf("ID contains character not in alphabet: %c", ch)
			}
		}
	})

	t.Run("numeric only alphabet", func(t *testing.T) {
		result, err := uc.Execute(ctx, "id_custom", map[string]interface{}{
			"length":   6,
			"alphabet": "0123456789",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		id := result.Response.(string)
		numericOnly := regexp.MustCompile(`^[0-9]+$`)
		if !numericOnly.MatchString(id) {
			t.Errorf("ID contains non-numeric characters: %s", id)
		}
	})

	t.Run("missing length returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "id_custom", map[string]interface{}{
			"alphabet": "ABC",
		})
		if err == nil {
			t.Fatal("expected error for missing length")
		}
	})

	t.Run("missing alphabet returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "id_custom", map[string]interface{}{
			"length": 8,
		})
		if err == nil {
			t.Fatal("expected error for missing alphabet")
		}
	})

	t.Run("empty alphabet returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "id_custom", map[string]interface{}{
			"length":   8,
			"alphabet": "",
		})
		if err == nil {
			t.Fatal("expected error for empty alphabet")
		}
	})

	t.Run("unsafe characters in alphabet returns error", func(t *testing.T) {
		unsafeChars := []string{"<", ">", "\"", "'", "&", "\\", "`"}

		for _, ch := range unsafeChars {
			_, err := uc.Execute(ctx, "id_custom", map[string]interface{}{
				"length":   8,
				"alphabet": "ABC" + ch,
			})
			if err == nil {
				t.Errorf("expected error for unsafe character: %s", ch)
			}
		}
	})

	t.Run("non-printable characters in alphabet returns error", func(t *testing.T) {
		_, err := uc.Execute(ctx, "id_custom", map[string]interface{}{
			"length":   8,
			"alphabet": "ABC\x00DEF",
		})
		if err == nil {
			t.Fatal("expected error for non-printable characters")
		}
	})
}
