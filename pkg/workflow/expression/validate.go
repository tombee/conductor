package expression

import (
	"fmt"
	"regexp"
	"strings"
)

// Step reference patterns to extract step IDs from expressions
var (
	// Matches {{.steps.id.field}} or {{steps.id.field}}
	templateStepPattern = regexp.MustCompile(`\{\{\.?steps\.([^.}\s]+)`)
	// Matches steps.id.field (expr-lang syntax)
	// Allow alphanumeric, underscore, and hyphen in step IDs
	exprStepPattern = regexp.MustCompile(`\bsteps\.([a-zA-Z_][a-zA-Z0-9_-]*)`)
)

// ValidateStepReferences validates that all step IDs referenced in an expression exist.
// It extracts step IDs from both template syntax ({{.steps.id}}) and expr-lang syntax (steps.id).
// Returns an error if any referenced step ID is not in the known steps list.
//
// Parameters:
//   - expression: The expression to validate
//   - knownStepIDs: List of valid step IDs in the workflow
//
// Example:
//
//	err := ValidateStepReferences(`steps.check.status == "success"`, []string{"check", "build"})
//	// Returns nil (check exists)
//
//	err := ValidateStepReferences(`steps.missing.status == "success"`, []string{"check", "build"})
//	// Returns error (missing not in known steps)
func ValidateStepReferences(expression string, knownStepIDs []string) error {
	if expression == "" {
		return nil
	}

	// Extract all referenced step IDs
	referencedSteps := extractStepReferences(expression)
	if len(referencedSteps) == 0 {
		return nil // No step references to validate
	}

	// Create a set of known step IDs for fast lookup
	knownSteps := make(map[string]bool)
	for _, id := range knownStepIDs {
		knownSteps[id] = true
	}

	// Check each referenced step
	var invalidSteps []string
	for _, stepID := range referencedSteps {
		if !knownSteps[stepID] {
			invalidSteps = append(invalidSteps, stepID)
		}
	}

	if len(invalidSteps) > 0 {
		return fmt.Errorf(
			"expression references unknown step(s): %s (known steps: %s)",
			strings.Join(invalidSteps, ", "),
			strings.Join(knownStepIDs, ", "),
		)
	}

	return nil
}

// extractStepReferences extracts all unique step IDs referenced in an expression.
// It checks both template syntax ({{.steps.id}}) and expr-lang syntax (steps.id).
func extractStepReferences(expression string) []string {
	stepSet := make(map[string]bool)

	// Extract from template patterns: {{.steps.id}} or {{steps.id}}
	templateMatches := templateStepPattern.FindAllStringSubmatch(expression, -1)
	for _, match := range templateMatches {
		if len(match) > 1 {
			stepSet[match[1]] = true
		}
	}

	// Extract from expr-lang patterns: steps.id
	exprMatches := exprStepPattern.FindAllStringSubmatch(expression, -1)
	for _, match := range exprMatches {
		if len(match) > 1 {
			stepSet[match[1]] = true
		}
	}

	// Convert set to slice
	steps := make([]string, 0, len(stepSet))
	for step := range stepSet {
		steps = append(steps, step)
	}

	return steps
}
