// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package assert

import (
	"testing"
)

func TestEvaluator_BasicComparisons(t *testing.T) {
	eval := New()

	tests := []struct {
		name       string
		expression string
		context    map[string]interface{}
		wantPassed bool
	}{
		{
			name:       "equality - pass",
			expression: "status_code == 200",
			context:    map[string]interface{}{"status_code": 200},
			wantPassed: true,
		},
		{
			name:       "equality - fail",
			expression: "status_code == 200",
			context:    map[string]interface{}{"status_code": 404},
			wantPassed: false,
		},
		{
			name:       "inequality - pass",
			expression: "status_code != 500",
			context:    map[string]interface{}{"status_code": 200},
			wantPassed: true,
		},
		{
			name:       "greater than - pass",
			expression: "total > 0",
			context:    map[string]interface{}{"total": 5},
			wantPassed: true,
		},
		{
			name:       "greater than - fail",
			expression: "total > 10",
			context:    map[string]interface{}{"total": 5},
			wantPassed: false,
		},
		{
			name:       "less than - pass",
			expression: "total < 100",
			context:    map[string]interface{}{"total": 50},
			wantPassed: true,
		},
		{
			name:       "greater or equal - pass",
			expression: "score >= 80",
			context:    map[string]interface{}{"score": 80},
			wantPassed: true,
		},
		{
			name:       "less or equal - pass",
			expression: "score <= 100",
			context:    map[string]interface{}{"score": 95},
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.Evaluate(tt.expression, tt.context)
			if result.Error != nil {
				t.Fatalf("Unexpected error: %v", result.Error)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("Expected Passed=%v, got %v", tt.wantPassed, result.Passed)
			}
		})
	}
}

func TestEvaluator_StringOperators(t *testing.T) {
	eval := New()

	tests := []struct {
		name       string
		expression string
		context    map[string]interface{}
		wantPassed bool
		wantError  bool
	}{
		{
			name:       "has (contains) - pass",
			expression: `has(body, "success")`,
			context:    map[string]interface{}{"body": "Operation was successful"},
			wantPassed: true,
		},
		{
			name:       "has (contains) - fail",
			expression: `has(body, "error")`,
			context:    map[string]interface{}{"body": "Operation was successful"},
			wantPassed: false,
		},
		{
			name:       "match (regex) - pass",
			expression: `match(id, "^[A-Z]{3}-\\d+$")`,
			context:    map[string]interface{}{"id": "ABC-123"},
			wantPassed: true,
		},
		{
			name:       "match (regex) - fail",
			expression: `match(id, "^[A-Z]{3}-\\d+$")`,
			context:    map[string]interface{}{"id": "invalid"},
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.Evaluate(tt.expression, tt.context)
			if tt.wantError {
				if result.Error == nil {
					t.Errorf("Expected error, got none")
				}
				return
			}
			if result.Error != nil {
				t.Fatalf("Unexpected error: %v", result.Error)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("Expected Passed=%v, got %v", tt.wantPassed, result.Passed)
			}
		})
	}
}

func TestEvaluator_CollectionOperators(t *testing.T) {
	eval := New()

	tests := []struct {
		name       string
		expression string
		context    map[string]interface{}
		wantPassed bool
	}{
		{
			name:       "includes (in) - pass",
			expression: `includes(status, [200, 201, 202])`,
			context:    map[string]interface{}{"status": 200},
			wantPassed: true,
		},
		{
			name:       "includes (in) - fail",
			expression: `includes(status, [200, 201, 202])`,
			context:    map[string]interface{}{"status": 404},
			wantPassed: false,
		},
		{
			name:       "notIn - pass",
			expression: `notIn(status, [400, 500])`,
			context:    map[string]interface{}{"status": 200},
			wantPassed: true,
		},
		{
			name:       "notIn - fail",
			expression: `notIn(status, [400, 500])`,
			context:    map[string]interface{}{"status": 400},
			wantPassed: false,
		},
		{
			name:       "has array - pass",
			expression: `has(items, "test")`,
			context:    map[string]interface{}{"items": []interface{}{"test", "other"}},
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.Evaluate(tt.expression, tt.context)
			if result.Error != nil {
				t.Fatalf("Unexpected error: %v", result.Error)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("Expected Passed=%v, got %v", tt.wantPassed, result.Passed)
			}
		})
	}
}

func TestEvaluator_PipeOperators(t *testing.T) {
	eval := New()

	tests := []struct {
		name       string
		expression string
		context    map[string]interface{}
		wantPassed bool
		wantError  bool
	}{
		{
			name:       "len pipe - pass",
			expression: `len(items) > 0`,
			context:    map[string]interface{}{"items": []interface{}{1, 2, 3}},
			wantPassed: true,
		},
		{
			name:       "len pipe - fail",
			expression: `len(items) > 0`,
			context:    map[string]interface{}{"items": []interface{}{}},
			wantPassed: false,
		},
		{
			name:       "lowercase pipe - pass",
			expression: `lowercase(name) == "alice"`,
			context:    map[string]interface{}{"name": "ALICE"},
			wantPassed: true,
		},
		{
			name:       "uppercase pipe - pass",
			expression: `uppercase(name) == "BOB"`,
			context:    map[string]interface{}{"name": "bob"},
			wantPassed: true,
		},
		{
			name:       "round pipe - pass",
			expression: `round(score) == 95`,
			context:    map[string]interface{}{"score": 94.7},
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.Evaluate(tt.expression, tt.context)
			if tt.wantError {
				if result.Error == nil {
					t.Errorf("Expected error, got none")
				}
				return
			}
			if result.Error != nil {
				t.Fatalf("Unexpected error: %v", result.Error)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("Expected Passed=%v, got %v", tt.wantPassed, result.Passed)
			}
		})
	}
}

func TestEvaluator_NestedAccess(t *testing.T) {
	eval := New()

	tests := []struct {
		name       string
		expression string
		context    map[string]interface{}
		wantPassed bool
	}{
		{
			name:       "nested map access - pass",
			expression: `body.data.id == 123`,
			context: map[string]interface{}{
				"body": map[string]interface{}{
					"data": map[string]interface{}{
						"id": 123,
					},
				},
			},
			wantPassed: true,
		},
		{
			name:       "nested array access - pass",
			expression: `body.items[0].name == "test"`,
			context: map[string]interface{}{
				"body": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{"name": "test"},
					},
				},
			},
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.Evaluate(tt.expression, tt.context)
			if result.Error != nil {
				t.Fatalf("Unexpected error: %v", result.Error)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("Expected Passed=%v, got %v", tt.wantPassed, result.Passed)
			}
		})
	}
}

func TestEvaluator_EmptyExpression(t *testing.T) {
	eval := New()

	result := eval.Evaluate("", map[string]interface{}{})
	if result.Error != nil {
		t.Fatalf("Unexpected error: %v", result.Error)
	}
	if !result.Passed {
		t.Errorf("Empty expression should pass")
	}
}

func TestEvaluator_InvalidExpression(t *testing.T) {
	eval := New()

	tests := []struct {
		name       string
		expression string
		context    map[string]interface{}
	}{
		{
			name:       "syntax error",
			expression: "status_code = 200",
			context:    map[string]interface{}{"status_code": 200},
		},
		{
			name:       "non-boolean result",
			expression: "status_code",
			context:    map[string]interface{}{"status_code": 200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.Evaluate(tt.expression, tt.context)
			if result.Error == nil {
				t.Errorf("Expected error, got none")
			}
			if result.Passed {
				t.Errorf("Invalid expression should not pass")
			}
		})
	}
}

func TestEvaluator_Cache(t *testing.T) {
	eval := New()

	expression := "status_code == 200"
	context := map[string]interface{}{"status_code": 200}

	// First evaluation should compile and cache
	result1 := eval.Evaluate(expression, context)
	if result1.Error != nil {
		t.Fatalf("Unexpected error: %v", result1.Error)
	}

	cacheSize := eval.CacheSize()
	if cacheSize != 1 {
		t.Errorf("Expected cache size 1, got %d", cacheSize)
	}

	// Second evaluation should use cache
	result2 := eval.Evaluate(expression, context)
	if result2.Error != nil {
		t.Fatalf("Unexpected error: %v", result2.Error)
	}

	// Cache size should still be 1
	cacheSize = eval.CacheSize()
	if cacheSize != 1 {
		t.Errorf("Expected cache size 1 after second evaluation, got %d", cacheSize)
	}

	// Clear cache
	eval.ClearCache()
	cacheSize = eval.CacheSize()
	if cacheSize != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cacheSize)
	}
}
