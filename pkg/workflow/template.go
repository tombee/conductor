package workflow

import (
	"bytes"
	"fmt"
	"text/template"
)

// TemplateContext holds the data available for template variable resolution.
// It provides access to workflow inputs, step outputs, and environment variables.
type TemplateContext struct {
	// Workflow inputs accessible as {{.input_name}}
	Inputs map[string]interface{}

	// Step outputs accessible as {{.steps.step_id.response}}
	Steps map[string]map[string]interface{}

	// Environment variables accessible as {{.env.VAR_NAME}}
	Env map[string]string

	// Tool results accessible as {{.tools.tool_name}}
	Tools map[string]interface{}

	// Loop context accessible as {{.loop.iteration}}, {{.loop.max_iterations}}, {{.loop.history}}
	Loop map[string]interface{}
}

// NewTemplateContext creates a new template context with empty maps.
func NewTemplateContext() *TemplateContext {
	return &TemplateContext{
		Inputs: make(map[string]interface{}),
		Steps:  make(map[string]map[string]interface{}),
		Env:    make(map[string]string),
		Tools:  make(map[string]interface{}),
	}
}

// SetInput adds an input variable to the context.
func (tc *TemplateContext) SetInput(name string, value interface{}) {
	tc.Inputs[name] = value
}

// SetStepOutput adds a step's output to the context.
func (tc *TemplateContext) SetStepOutput(stepID string, output map[string]interface{}) {
	tc.Steps[stepID] = output
}

// SetToolResult adds a tool's result to the context.
func (tc *TemplateContext) SetToolResult(toolName string, result interface{}) {
	tc.Tools[toolName] = result
}

// SetLoopContext sets the loop context for loop step execution.
// This provides access to loop.iteration, loop.max_iterations, and loop.history.
func (tc *TemplateContext) SetLoopContext(iteration, maxIterations int, history interface{}) {
	tc.Loop = map[string]interface{}{
		"iteration":      iteration,
		"max_iterations": maxIterations,
		"history":        history,
	}
}

// ToMap converts the context to a flat map for template execution.
// Input variables are available both at the top level ({{.input_name}}) and
// under "inputs" ({{.inputs.input_name}}) for compatibility with workflow definitions.
// Step outputs are under "steps": {{.steps.step_id.response}}
// Environment variables are under "env": {{.env.VAR_NAME}}
// Tool results are under "tools": {{.tools.tool_name}}
// Loop context is under "loop": {{.loop.iteration}}, {{.loop.max_iterations}}, {{.loop.history}}
func (tc *TemplateContext) ToMap() map[string]interface{} {
	data := make(map[string]interface{})

	// Add all inputs at top level for simple access {{.input_name}}
	for k, v := range tc.Inputs {
		data[k] = v
	}

	// Also add inputs under "inputs" key for explicit access {{.inputs.input_name}}
	data["inputs"] = tc.Inputs

	// Add steps under "steps" key
	data["steps"] = tc.Steps

	// Add environment variables under "env" key
	data["env"] = tc.Env

	// Add tool results under "tools" key
	data["tools"] = tc.Tools

	// Add loop context under "loop" key if present
	if tc.Loop != nil {
		data["loop"] = tc.Loop
	}

	return data
}

// ResolveTemplate executes a Go template string with the given context.
// Returns the resolved string or an error if template execution fails.
func ResolveTemplate(templateStr string, ctx *TemplateContext) (string, error) {
	if ctx == nil {
		ctx = NewTemplateContext()
	}

	tmpl, err := template.New("workflow").
		Funcs(TemplateFuncMap()).
		Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx.ToMap()); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ResolveInputs resolves all string values in the inputs map using the template context.
func ResolveInputs(inputs map[string]interface{}, ctx *TemplateContext) (map[string]interface{}, error) {
	resolved := make(map[string]interface{})

	for key, value := range inputs {
		resolvedVal, err := resolveValue(value, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve input %q: %w", key, err)
		}
		resolved[key] = resolvedVal
	}

	return resolved, nil
}

// resolveValue recursively resolves template variables in a value.
func resolveValue(value interface{}, ctx *TemplateContext) (interface{}, error) {
	switch v := value.(type) {
	case string:
		// Check if this is a pure template reference (preserves type)
		if isPureTemplateRef(v) {
			rawVal, ok := extractRawValue(v, ctx)
			if ok {
				return rawVal, nil
			}
		}
		// Graceful degradation: if template resolution fails, keep original value
		resolved, err := resolveOrKeep(v, ctx)
		if err != nil {
			return v, nil
		}
		return resolved, nil
	case map[string]interface{}:
		resolved := make(map[string]interface{})
		for k, val := range v {
			resolvedVal, err := resolveValue(val, ctx)
			if err != nil {
				return nil, fmt.Errorf("in field %q: %w", k, err)
			}
			resolved[k] = resolvedVal
		}
		return resolved, nil
	case []interface{}:
		resolved := make([]interface{}, len(v))
		for i, val := range v {
			resolvedVal, err := resolveValue(val, ctx)
			if err != nil {
				return nil, fmt.Errorf("at index %d: %w", i, err)
			}
			resolved[i] = resolvedVal
		}
		return resolved, nil
	default:
		return value, nil
	}
}

// resolveOrKeep tries to resolve a string as a template, returns error if template syntax is present but fails.
func resolveOrKeep(s string, ctx *TemplateContext) (string, error) {
	// Check if string contains template syntax
	if !containsTemplateSyntax(s) {
		return s, nil
	}

	result, err := ResolveTemplate(s, ctx)
	if err != nil {
		return "", fmt.Errorf("template error in %q: %w", truncateForError(s), err)
	}

	// If template produced "<no value>", it means variable was undefined
	if result == "<no value>" {
		return "", fmt.Errorf("undefined template variable in %q", truncateForError(s))
	}

	return result, nil
}

// truncateForError truncates a string for inclusion in error messages.
func truncateForError(s string) string {
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

// containsTemplateSyntax checks if a string contains Go template syntax.
func containsTemplateSyntax(s string) bool {
	// Simple check for {{ and }}
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '{' && s[i+1] == '{' {
			return true
		}
	}
	return false
}

// isPureTemplateRef checks if a string is exactly a single template reference
// like "{{.steps.foo.response}}" with no surrounding text.
func isPureTemplateRef(s string) bool {
	s = trimWhitespace(s)
	if len(s) < 5 { // Minimum: {{.x}}
		return false
	}
	if s[:2] != "{{" || s[len(s)-2:] != "}}" {
		return false
	}
	// Check there's no other {{ in the middle
	inner := s[2 : len(s)-2]
	for i := 0; i < len(inner)-1; i++ {
		if inner[i] == '{' && inner[i+1] == '{' {
			return false
		}
		if inner[i] == '}' && inner[i+1] == '}' {
			return false
		}
	}
	return true
}

// trimWhitespace removes leading and trailing whitespace
func trimWhitespace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// extractRawValue extracts the raw value from a pure template reference.
// It parses paths like "{{.steps.foo.response}}" and navigates the context.
func extractRawValue(s string, ctx *TemplateContext) (interface{}, bool) {
	s = trimWhitespace(s)
	inner := trimWhitespace(s[2 : len(s)-2]) // Remove {{ and }}

	// Must start with a dot
	if len(inner) == 0 || inner[0] != '.' {
		return nil, false
	}
	inner = inner[1:] // Remove leading dot

	// Split the path
	parts := splitPath(inner)
	if len(parts) == 0 {
		return nil, false
	}

	data := ctx.ToMap()
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}

	return current, true
}

// splitPath splits a template path like "steps.foo.response" into parts.
func splitPath(path string) []string {
	var parts []string
	var current string

	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(path[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	return parts
}
