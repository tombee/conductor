package workspace

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/pkg/workflow"
)

// BindingResolver resolves workflow integration requirements to workspace integrations.
type BindingResolver struct {
	storage Storage
}

// NewBindingResolver creates a new binding resolver.
func NewBindingResolver(storage Storage) *BindingResolver {
	return &BindingResolver{
		storage: storage,
	}
}

// ResolvedBinding represents a resolved integration binding.
type ResolvedBinding struct {
	// Requirement is the original requirement (type and alias)
	Requirement workflow.ParsedIntegrationRequirement

	// Integration is the workspace integration that satisfies this requirement
	Integration *Integration

	// BindingMethod indicates how the binding was resolved (auto or explicit)
	BindingMethod BindingMethod
}

// BindingMethod indicates how a binding was resolved.
type BindingMethod string

const (
	// BindingMethodAuto indicates the binding was automatically resolved
	BindingMethodAuto BindingMethod = "auto"

	// BindingMethodExplicit indicates the binding was explicitly specified
	BindingMethodExplicit BindingMethod = "explicit"
)

// ResolveBindings resolves workflow integration requirements to workspace integrations.
// explicitBindings is a map from requirement identifier (type or alias) to integration name.
// For simple requirements (no alias), the identifier is the type (e.g., "github").
// For aliased requirements, the identifier is the alias (e.g., "source").
func (r *BindingResolver) ResolveBindings(
	ctx context.Context,
	workspaceName string,
	wf *workflow.Definition,
	explicitBindings map[string]string,
) (map[string]*ResolvedBinding, error) {
	if wf.Requires == nil || len(wf.Requires.Integrations) == 0 {
		// No requirements, return empty bindings
		return make(map[string]*ResolvedBinding), nil
	}

	bindings := make(map[string]*ResolvedBinding)

	for _, reqStr := range wf.Requires.Integrations {
		parsed := workflow.ParseIntegrationRequirement(reqStr)

		// Determine the binding identifier (type or alias)
		identifier := parsed.Type
		if parsed.Alias != "" {
			identifier = parsed.Alias
		}

		// Check for explicit binding
		if integrationName, ok := explicitBindings[identifier]; ok {
			// Explicit binding specified
			integration, err := r.storage.GetIntegration(ctx, workspaceName, integrationName)
			if err != nil {
				return nil, &BindingError{
					Requirement: parsed,
					Identifier:  identifier,
					Reason:      fmt.Sprintf("explicit binding to integration '%s' failed: %v", integrationName, err),
				}
			}

			// Verify the integration type matches the requirement
			if integration.Type != parsed.Type {
				return nil, &BindingError{
					Requirement: parsed,
					Identifier:  identifier,
					Reason: fmt.Sprintf(
						"integration '%s' has type '%s', but requirement needs type '%s'",
						integrationName,
						integration.Type,
						parsed.Type,
					),
				}
			}

			bindings[identifier] = &ResolvedBinding{
				Requirement:   parsed,
				Integration:   integration,
				BindingMethod: BindingMethodExplicit,
			}
			continue
		}

		// No explicit binding, try auto-binding
		// For aliased requirements, auto-binding is not allowed
		if parsed.Alias != "" {
			return nil, &BindingError{
				Requirement: parsed,
				Identifier:  identifier,
				Reason: fmt.Sprintf(
					"aliased requirement '%s as %s' requires explicit binding via --bind-integration %s=<name>",
					parsed.Type,
					parsed.Alias,
					parsed.Alias,
				),
			}
		}

		// Auto-bind: find integrations of the required type
		integrations, err := r.storage.ListIntegrations(ctx, workspaceName)
		if err != nil {
			return nil, fmt.Errorf("failed to list integrations: %w", err)
		}

		// Filter by type
		var matches []*Integration
		for _, integration := range integrations {
			if integration.Type == parsed.Type {
				matches = append(matches, integration)
			}
		}

		if len(matches) == 0 {
			// No integration of required type
			return nil, &NoIntegrationError{
				Type:      parsed.Type,
				Workspace: workspaceName,
			}
		}

		if len(matches) > 1 {
			// Multiple integrations of type, explicit binding required
			var names []string
			for _, m := range matches {
				names = append(names, m.Name)
			}
			return nil, &MultipleIntegrationsError{
				Type:           parsed.Type,
				Workspace:      workspaceName,
				IntegrationNames: names,
			}
		}

		// Exactly one match, auto-bind
		bindings[identifier] = &ResolvedBinding{
			Requirement:   parsed,
			Integration:   matches[0],
			BindingMethod: BindingMethodAuto,
		}
	}

	return bindings, nil
}

// BindingError represents a binding resolution error.
type BindingError struct {
	Requirement workflow.ParsedIntegrationRequirement
	Identifier  string
	Reason      string
}

func (e *BindingError) Error() string {
	return fmt.Sprintf("binding error for '%s': %s", e.Identifier, e.Reason)
}

// NoIntegrationError indicates no integration of the required type was found.
type NoIntegrationError struct {
	Type      string
	Workspace string
}

func (e *NoIntegrationError) Error() string {
	return fmt.Sprintf(
		"Error: Workflow requires '%s' integration but none configured.\n\n"+
			"To fix: conductor integrations add %s --token '${%s_TOKEN}'",
		e.Type,
		e.Type,
		toUpperSnakeCase(e.Type),
	)
}

// MultipleIntegrationsError indicates multiple integrations of the required type exist.
type MultipleIntegrationsError struct {
	Type             string
	Workspace        string
	IntegrationNames []string
}

func (e *MultipleIntegrationsError) Error() string {
	return fmt.Sprintf(
		"Error: Workflow requires '%s' but %d integrations found: %v.\n\n"+
			"To fix: conductor run workflow.yaml --bind-integration %s=<name>",
		e.Type,
		len(e.IntegrationNames),
		e.IntegrationNames,
		e.Type,
	)
}

// toUpperSnakeCase converts a string to UPPER_SNAKE_CASE.
// Example: "github" -> "GITHUB", "github-enterprise" -> "GITHUB_ENTERPRISE"
func toUpperSnakeCase(s string) string {
	result := ""
	for i, ch := range s {
		if ch >= 'a' && ch <= 'z' {
			result += string(ch - 32) // Convert to uppercase
		} else if ch >= 'A' && ch <= 'Z' {
			result += string(ch)
		} else if ch == '-' || ch == ' ' {
			result += "_"
		} else if ch >= '0' && ch <= '9' {
			result += string(ch)
		} else if i > 0 && (ch == '_' || ch == '.') {
			result += "_"
		}
	}
	return result
}
