package workflow

import (
	"fmt"
	"regexp"
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

// PlaintextCredentialPattern represents a pattern for detecting plaintext credentials in workflows.
type PlaintextCredentialPattern struct {
	Name    string
	Pattern *regexp.Regexp
}

var (
	// workflowCredentialPatterns contains patterns for detecting embedded credentials in workflow definitions.
	// These patterns warn users to use the `requires` section and profiles instead of embedding credentials.
	workflowCredentialPatterns = []PlaintextCredentialPattern{
		{
			Name:    "GitHub Token",
			Pattern: regexp.MustCompile(`\b(ghp_|gho_|ghu_|ghs_|ghr_)[a-zA-Z0-9]{36,}\b`),
		},
		{
			Name:    "Anthropic API Key",
			Pattern: regexp.MustCompile(`\bsk-ant-[a-zA-Z0-9-]{95,}\b`),
		},
		{
			Name:    "OpenAI API Key",
			Pattern: regexp.MustCompile(`\bsk-[a-zA-Z0-9]{20,}\b`),
		},
		{
			Name:    "AWS Access Key",
			Pattern: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		},
		{
			Name:    "Slack Token",
			Pattern: regexp.MustCompile(`\b(xoxb-|xoxp-|xoxa-|xoxr-)[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{24,}\b`),
		},
	}
)

// DetectEmbeddedCredentials checks the workflow definition for embedded plaintext credentials.
// Returns warnings about found credentials. This is a non-blocking warning - workflows with
// embedded credentials are still valid, but users are warned to use profiles instead (SPEC-130).
func DetectEmbeddedCredentials(def *Definition) []string {
	var warnings []string

	// Helper to check a string value for credentials
	checkValue := func(location, value string) {
		for _, pattern := range workflowCredentialPatterns {
			if pattern.Pattern.MatchString(value) {
				warnings = append(warnings, fmt.Sprintf(
					"%s contains %s - consider using `requires` section with profile bindings instead of embedding credentials",
					location, pattern.Name,
				))
			}
		}
	}

	// Check connectors for embedded auth
	for name, connector := range def.Connectors {
		if connector.Auth != nil {
			// Check all auth fields
			if connector.Auth.Token != "" {
				checkValue(fmt.Sprintf("connectors.%s.auth.token", name), connector.Auth.Token)
			}
			if connector.Auth.Username != "" {
				checkValue(fmt.Sprintf("connectors.%s.auth.username", name), connector.Auth.Username)
			}
			if connector.Auth.Password != "" {
				checkValue(fmt.Sprintf("connectors.%s.auth.password", name), connector.Auth.Password)
			}
			if connector.Auth.Value != "" {
				checkValue(fmt.Sprintf("connectors.%s.auth.value", name), connector.Auth.Value)
			}
		}
	}

	// Check MCP servers for embedded credentials in env
	for i, server := range def.MCPServers {
		for _, envVar := range server.Env {
			// Env vars are in "KEY=value" format
			checkValue(fmt.Sprintf("mcp_servers[%d].env", i), envVar)
		}
	}

	return warnings
}
