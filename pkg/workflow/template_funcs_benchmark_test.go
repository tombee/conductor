package workflow

import (
	"testing"
)

// Benchmark tests to verify NFR1: < 1 microsecond overhead per function call

func BenchmarkMathFunctions(b *testing.B) {
	b.Run("add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = add(2, 3, 4)
		}
	})

	b.Run("sub", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = sub(10, 3)
		}
	})

	b.Run("mul", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = mul(2, 3, 4)
		}
	})

	b.Run("div", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = div(10, 2)
		}
	})

	b.Run("divf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = divf(10, 4)
		}
	})

	b.Run("mod", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = mod(10, 3)
		}
	})

	b.Run("min", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = min(3, 1, 4, 1, 5)
		}
	})

	b.Run("max", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = max(3, 1, 4, 1, 5)
		}
	})
}

func BenchmarkJsonFunctions(b *testing.B) {
	testData := map[string]interface{}{
		"name":  "Thorin",
		"level": 3,
		"items": []string{"sword", "shield", "axe"},
	}

	jsonStr := `{"name":"Thorin","level":3,"items":["sword","shield","axe"]}`

	b.Run("toJson", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = toJson(testData)
		}
	})

	b.Run("toJsonPretty", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = toJsonPretty(testData)
		}
	})

	b.Run("fromJson", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = fromJson(jsonStr)
		}
	})
}

func BenchmarkStringFunctions(b *testing.B) {
	testArray := []string{"a", "b", "c", "d", "e"}

	b.Run("joinFunc", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = joinFunc(testArray, ", ")
		}
	})

	b.Run("titleCase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = titleCase("hello world")
		}
	})
}

func BenchmarkCollectionFunctions(b *testing.B) {
	testArray := []int{1, 2, 3, 4, 5}
	testMap := map[string]int{"a": 1, "b": 2, "c": 3}
	testObjects := []map[string]interface{}{
		{"name": "Thorin", "level": 3},
		{"name": "Gandalf", "level": 10},
		{"name": "Bilbo", "level": 1},
	}

	b.Run("first", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = first(testArray)
		}
	})

	b.Run("last", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = last(testArray)
		}
	})

	b.Run("keys", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = keys(testMap)
		}
	})

	b.Run("values", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = values(testMap)
		}
	})

	b.Run("hasKey", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = hasKey(testMap, "a")
		}
	})

	b.Run("pluck", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = pluck(testObjects, "name")
		}
	})
}

func BenchmarkDefaultFunctions(b *testing.B) {
	b.Run("defaultFunc", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = defaultFunc("default", "value")
		}
	})

	b.Run("coalesce", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = coalesce(nil, "", "first", "second")
		}
	})
}

func BenchmarkTypeConversion(b *testing.B) {
	b.Run("toInt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = toInt("42")
		}
	})

	b.Run("toFloat", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = toFloat("3.14")
		}
	})

	b.Run("toString", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = toString(42)
		}
	})

	b.Run("toBool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = toBool("true")
		}
	})
}

func BenchmarkTemplateIntegration(b *testing.B) {
	ctx := NewTemplateContext()
	ctx.SetInput("roll", 14)
	ctx.SetInput("modifier", 5)
	ctx.SetStepOutput("character", map[string]interface{}{
		"name":  "Thorin",
		"level": 3,
	})

	template := "Attack roll: {{add .roll .modifier}} (rolled {{.roll}} + {{.modifier}})"

	b.Run("full template resolution", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = ResolveTemplate(template, ctx)
		}
	})
}
