package workflow

import (
	"fmt"
	"strings"

	"github.com/tombee/conductor/pkg/errors"
)

// ValidateExpressionInjection checks for template expressions in expr fields.
// This prevents injection attacks where user-controlled template variables
// could be used to inject malicious jq expressions.
func ValidateExpressionInjection(step *StepDefinition) error {
	// Only check steps that use expr parameter
	if step.Inputs == nil {
		return nil
	}

	expr, ok := step.Inputs["expr"]
	if !ok {
		return nil
	}

	exprStr, ok := expr.(string)
	if !ok {
		return nil // Type validation happens elsewhere
	}

	// Check if expr contains template expression markers
	if strings.Contains(exprStr, "{{") && strings.Contains(exprStr, "}}") {
		return &errors.ValidationError{
			Field:      "expr",
			Message:    "expression injection risk: expr field cannot contain template expressions",
			Suggestion: "use static expressions only; pass dynamic values through inputs instead",
		}
	}

	return nil
}

// ValidateNestedForeach checks for foreach inside foreach constructs.
// Nested foreach is not supported as it creates complex iteration semantics
// and potential performance issues.
func ValidateNestedForeach(step *StepDefinition, inForeachContext bool) error {
	// Check if this step has foreach
	if step.Foreach != "" {
		if inForeachContext {
			return &errors.ValidationError{
				Field:      "foreach",
				Message:    fmt.Sprintf("nested foreach not supported: step '%s' uses foreach inside foreach context", step.ID),
				Suggestion: "flatten nested iterations or use a separate workflow step",
			}
		}
		// This step starts a foreach context
		inForeachContext = true
	}

	// Recursively check nested steps (only relevant for parallel steps)
	if step.Type == StepTypeParallel {
		for _, nestedStep := range step.Steps {
			if err := ValidateNestedForeach(&nestedStep, inForeachContext); err != nil {
				return err
			}
		}
	}

	return nil
}
