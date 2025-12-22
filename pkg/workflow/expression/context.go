package expression

// BuildContext creates an expression evaluation context from workflow context.
//
// The workflow context typically contains:
//   - "_templateContext": *TemplateContext (internal)
//   - "inputs": workflow input values
//   - "steps": map of step results
//   - "loop": loop context (iteration, max_iterations, history) for loop steps
//
// This function extracts the relevant fields into a flat map structure
// suitable for expression evaluation:
//
//	{
//	    "inputs": {"name": "value", ...},
//	    "steps": {
//	        "step_id": {"content": "...", "status": "success"},
//	        ...
//	    },
//	    "loop": {
//	        "iteration": 0,
//	        "max_iterations": 10,
//	        "history": [...]
//	    }
//	}
func BuildContext(workflowContext map[string]interface{}) map[string]interface{} {
	ctx := make(map[string]interface{})

	// Extract inputs
	if inputs, ok := workflowContext["inputs"]; ok {
		ctx["inputs"] = inputs
	} else {
		ctx["inputs"] = make(map[string]interface{})
	}

	// Extract step results
	if steps, ok := workflowContext["steps"]; ok {
		ctx["steps"] = steps
	} else {
		ctx["steps"] = make(map[string]interface{})
	}

	// Extract loop context (for loop steps)
	if loop, ok := workflowContext["loop"]; ok {
		ctx["loop"] = loop
	}

	// Also expose at top level for convenience (allows both $.inputs.x and inputs.x)
	// This matches the JSONPath-style references used in workflow YAML
	if inputs, ok := ctx["inputs"].(map[string]interface{}); ok {
		for k, v := range inputs {
			if _, exists := ctx[k]; !exists {
				ctx[k] = v
			}
		}
	}

	return ctx
}

// BuildContextFromInputsAndSteps creates an expression context from separate inputs and steps maps.
// This is useful when you have the components separately rather than a combined workflow context.
func BuildContextFromInputsAndSteps(inputs, steps map[string]interface{}) map[string]interface{} {
	ctx := make(map[string]interface{})

	if inputs != nil {
		ctx["inputs"] = inputs
	} else {
		ctx["inputs"] = make(map[string]interface{})
	}

	if steps != nil {
		ctx["steps"] = steps
	} else {
		ctx["steps"] = make(map[string]interface{})
	}

	return ctx
}

// StepOutputConverter defines the interface for converting step outputs to maps.
// This interface breaks the circular dependency between expression and workflow packages.
type StepOutputConverter interface {
	ToMap() map[string]interface{}
}

// BuildContextFromTypedOutputs creates an expression context from typed inputs and step outputs.
// This function converts type-safe step outputs to untyped maps for expr-lang evaluation.
// The expression layer remains untyped per architectural decision to maintain compatibility
// with the expr library and allow flexible expression evaluation.
//
// Parameters:
//   - inputs: Raw workflow inputs (already untyped)
//   - stepOutputs: Map of step IDs to StepOutput converters
//
// Note: Type safety is enforced at the input boundary (workflow inputs) but expressions
// operate on untyped data. The caller (workflow package) handles conversion.
func BuildContextFromTypedOutputs(inputs map[string]any, stepOutputs map[string]StepOutputConverter) map[string]interface{} {
	steps := make(map[string]interface{})

	for stepID, converter := range stepOutputs {
		if converter != nil {
			steps[stepID] = converter.ToMap()
		}
	}

	return BuildContextFromInputsAndSteps(inputs, steps)
}
