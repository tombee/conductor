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
		switch v := value.(type) {
		case string:
			resolved[key] = resolveOrKeep(v, ctx)
		default:
			resolved[key] = value
		}
	}

	return resolved, nil
}

// resolveOrKeep tries to resolve a string as a template, returns original if no template syntax.
func resolveOrKeep(s string, ctx *TemplateContext) string {
	// Check if string contains template syntax
	if !containsTemplateSyntax(s) {
		return s
	}

	result, err := ResolveTemplate(s, ctx)
	if err != nil {
		// Return original string if template fails (might not be a template)
		return s
	}

	// If template produced "<no value>", it means variable was undefined
	// Return original string to preserve the template for debugging
	if result == "<no value>" {
		return s
	}

	return result
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
