package workflow

import (
	"testing"
)

func TestTemplateContext_SetInput(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetInput("name", "John")
	ctx.SetInput("age", 30)

	if ctx.Inputs["name"] != "John" {
		t.Errorf("expected input 'name' to be 'John', got %v", ctx.Inputs["name"])
	}
	if ctx.Inputs["age"] != 30 {
		t.Errorf("expected input 'age' to be 30, got %v", ctx.Inputs["age"])
	}
}

func TestTemplateContext_SetStepOutput(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetStepOutput("step1", map[string]interface{}{
		"response": "Hello, world!",
		"status":   "success",
	})

	if ctx.Steps["step1"]["response"] != "Hello, world!" {
		t.Errorf("expected step1 response to be 'Hello, world!', got %v", ctx.Steps["step1"]["response"])
	}
	if ctx.Steps["step1"]["status"] != "success" {
		t.Errorf("expected step1 status to be 'success', got %v", ctx.Steps["step1"]["status"])
	}
}

func TestTemplateContext_ToMap(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetInput("input1", "value1")
	ctx.SetInput("input2", 42)
	ctx.SetStepOutput("step1", map[string]interface{}{
		"response": "output1",
	})

	data := ctx.ToMap()

	// Check inputs are at top level
	if data["input1"] != "value1" {
		t.Errorf("expected input1 at top level, got %v", data["input1"])
	}
	if data["input2"] != 42 {
		t.Errorf("expected input2 at top level, got %v", data["input2"])
	}

	// Check steps are under "steps" key
	steps, ok := data["steps"].(map[string]map[string]interface{})
	if !ok {
		t.Fatalf("expected steps to be map[string]map[string]interface{}, got %T", data["steps"])
	}
	if steps["step1"]["response"] != "output1" {
		t.Errorf("expected step1 response, got %v", steps["step1"]["response"])
	}
}

func TestResolveTemplate_SimpleInput(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetInput("name", "Alice")

	result, err := ResolveTemplate("Hello, {{.name}}!", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Hello, Alice!"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveTemplate_StepOutput(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetStepOutput("security", map[string]interface{}{
		"response": "No issues found",
	})

	result, err := ResolveTemplate("Review: {{.steps.security.response}}", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Review: No issues found"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveTemplate_MultipleVariables(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetInput("user", "Bob")
	ctx.SetInput("task", "code review")
	ctx.SetStepOutput("step1", map[string]interface{}{
		"response": "Complete",
	})

	template := "User: {{.user}}, Task: {{.task}}, Status: {{.steps.step1.response}}"
	result, err := ResolveTemplate(template, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "User: Bob, Task: code review, Status: Complete"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveTemplate_NoTemplateSyntax(t *testing.T) {
	ctx := NewTemplateContext()

	result, err := ResolveTemplate("Just a plain string", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Just a plain string"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveTemplate_UndefinedVariable(t *testing.T) {
	ctx := NewTemplateContext()

	// Undefined variable produces "<no value>" in Go templates (not an error)
	result, err := ResolveTemplate("Hello, {{.undefined}}!", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Go template engine replaces undefined variables with "<no value>"
	expected := "Hello, <no value>!"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveTemplate_InvalidSyntax(t *testing.T) {
	ctx := NewTemplateContext()

	// Invalid template syntax should produce error
	_, err := ResolveTemplate("Hello, {{.name}!", ctx)
	if err == nil {
		t.Error("expected error for invalid syntax, got nil")
	}
}

func TestResolveTemplate_NilContext(t *testing.T) {
	// Nil context should be handled gracefully
	result, err := ResolveTemplate("Hello, world!", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Hello, world!"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveTemplate_ComplexStepChain(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetInput("diff", "func main() { ... }")
	ctx.SetStepOutput("security", map[string]interface{}{
		"response": "Security review: No critical issues",
	})
	ctx.SetStepOutput("summary", map[string]interface{}{
		"response": "All checks passed",
	})

	template := "Input: {{.diff}}\nSecurity: {{.steps.security.response}}\nSummary: {{.steps.summary.response}}"
	result, err := ResolveTemplate(template, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Input: func main() { ... }\nSecurity: Security review: No critical issues\nSummary: All checks passed"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveInputs_StringValues(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetInput("user", "Alice")

	inputs := map[string]interface{}{
		"greeting": "Hello, {{.user}}!",
		"number":   42,
		"bool":     true,
	}

	resolved, err := ResolveInputs(inputs, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// String with template should be resolved
	if resolved["greeting"] != "Hello, Alice!" {
		t.Errorf("expected greeting to be resolved, got %v", resolved["greeting"])
	}

	// Non-string values should pass through
	if resolved["number"] != 42 {
		t.Errorf("expected number to pass through, got %v", resolved["number"])
	}
	if resolved["bool"] != true {
		t.Errorf("expected bool to pass through, got %v", resolved["bool"])
	}
}

func TestResolveInputs_NoTemplateSyntax(t *testing.T) {
	ctx := NewTemplateContext()

	inputs := map[string]interface{}{
		"plain":  "just a string",
		"number": 123,
	}

	resolved, err := ResolveInputs(inputs, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Plain string should pass through unchanged
	if resolved["plain"] != "just a string" {
		t.Errorf("expected plain string to pass through, got %v", resolved["plain"])
	}
}

func TestResolveInputs_TemplateError(t *testing.T) {
	ctx := NewTemplateContext()

	inputs := map[string]interface{}{
		// Invalid template syntax should be kept as-is (graceful degradation)
		"invalid": "{{.undefined}}",
	}

	resolved, err := ResolveInputs(inputs, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When template fails, original string should be kept
	if resolved["invalid"] != "{{.undefined}}" {
		t.Errorf("expected original string on error, got %v", resolved["invalid"])
	}
}

func TestContainsTemplateSyntax(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "simple template",
			input:    "{{.variable}}",
			expected: true,
		},
		{
			name:     "template in string",
			input:    "Hello, {{.name}}!",
			expected: true,
		},
		{
			name:     "no template",
			input:    "plain string",
			expected: false,
		},
		{
			name:     "single brace",
			input:    "{not a template}",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "single character",
			input:    "{",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsTemplateSyntax(tt.input)
			if result != tt.expected {
				t.Errorf("containsTemplateSyntax(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveOrKeep(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetInput("name", "Bob")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "template is resolved",
			input:    "Hello, {{.name}}!",
			expected: "Hello, Bob!",
		},
		{
			name:     "plain string is kept",
			input:    "Hello, world!",
			expected: "Hello, world!",
		},
		{
			name:     "invalid template is kept",
			input:    "{{.undefined}}",
			expected: "{{.undefined}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveOrKeep(tt.input, ctx)
			if result != tt.expected {
				t.Errorf("resolveOrKeep(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
