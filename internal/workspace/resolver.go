package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tombee/conductor/pkg/profile"
	"github.com/tombee/conductor/pkg/workflow"
)

// BindingResolver resolves workflow integration requirements to workspace integrations.
type BindingResolver struct {
	storage        Storage
	secretRegistry profile.SecretProviderRegistry
}

// NewBindingResolver creates a new binding resolver.
// The secretRegistry is used to resolve secret references in integration auth fields.
func NewBindingResolver(storage Storage, secretRegistry profile.SecretProviderRegistry) *BindingResolver {
	return &BindingResolver{
		storage:        storage,
		secretRegistry: secretRegistry,
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

// ResolveSecrets resolves secret references in an integration's auth configuration.
// This is called at binding time (not at configuration time) to resolve secrets
// from their references (env:, file:, keychain:, ${VAR}) to actual values.
//
// Resolution occurs for:
//   - AuthConfig.Token (for token auth)
//   - AuthConfig.Password (for basic auth)
//   - AuthConfig.APIKeyValue (for api-key auth)
//
// Returns a new Integration with resolved secrets (original is not modified).
func (r *BindingResolver) ResolveSecrets(ctx context.Context, integration *Integration) (*Integration, error) {
	if r.secretRegistry == nil {
		// No secret registry configured - return integration as-is
		// This can happen in test scenarios or when secrets aren't needed
		return integration, nil
	}

	// Create a copy to avoid modifying the original
	resolved := *integration
	resolved.Auth = integration.Auth

	// Resolve token if present
	if resolved.Auth.Token != "" {
		value, err := r.resolveSecretValue(ctx, resolved.Auth.Token, integration.Name, "token")
		if err != nil {
			return nil, err
		}
		resolved.Auth.Token = value
	}

	// Resolve password if present
	if resolved.Auth.Password != "" {
		value, err := r.resolveSecretValue(ctx, resolved.Auth.Password, integration.Name, "password")
		if err != nil {
			return nil, err
		}
		resolved.Auth.Password = value
	}

	// Resolve API key value if present
	if resolved.Auth.APIKeyValue != "" {
		value, err := r.resolveSecretValue(ctx, resolved.Auth.APIKeyValue, integration.Name, "api_key_value")
		if err != nil {
			return nil, err
		}
		resolved.Auth.APIKeyValue = value
	}

	return &resolved, nil
}

// resolveSecretValue resolves a single secret value.
// If the value is a secret reference (${VAR}, env:, file:, keychain:), it's resolved.
// Otherwise, the value is returned as-is (literal value).
func (r *BindingResolver) resolveSecretValue(ctx context.Context, value, integrationName, fieldName string) (string, error) {
	if !isSecretReference(value) {
		// Not a reference, return as-is (already a literal value)
		return value, nil
	}

	// Resolve the reference
	resolved, err := r.secretRegistry.Resolve(ctx, value)
	if err != nil {
		return "", &SecretResolutionError{
			IntegrationName: integrationName,
			Field:           fieldName,
			Reference:       value,
			Cause:           err,
		}
	}

	return resolved, nil
}

// isSecretReference checks if a value looks like a secret reference.
// Returns true for: ${VAR}, env:VAR, file:/path, keychain:name
func isSecretReference(value string) bool {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return true // ${VAR} format
	}
	if strings.HasPrefix(value, "env:") {
		return true // env:VAR format
	}
	if strings.HasPrefix(value, "file:") {
		return true // file:/path format
	}
	if strings.HasPrefix(value, "keychain:") {
		return true // keychain:name format
	}
	return false
}

// SecretResolutionError represents a failure to resolve a secret reference.
type SecretResolutionError struct {
	IntegrationName string
	Field           string
	Reference       string
	Cause           error
}

func (e *SecretResolutionError) Error() string {
	// Extract the error category from the underlying cause if it's a SecretResolutionError
	var secretErr *profile.SecretResolutionError
	category := "UNKNOWN"
	suggestion := ""

	if errors.As(e.Cause, &secretErr) {
		category = string(secretErr.Category)

		// Provide helpful suggestions based on error category
		switch secretErr.Category {
		case profile.ErrorCategoryNotFound:
			if strings.HasPrefix(e.Reference, "env:") || strings.HasPrefix(e.Reference, "${") {
				// Extract var name
				varName := e.Reference
				if strings.HasPrefix(varName, "env:") {
					varName = varName[4:]
				} else if strings.HasPrefix(varName, "${") && strings.HasSuffix(varName, "}") {
					varName = varName[2 : len(varName)-1]
				}
				suggestion = fmt.Sprintf("\n\nTo fix: export %s=<your-value>", varName)
			} else if strings.HasPrefix(e.Reference, "file:") {
				path := e.Reference[5:]
				suggestion = fmt.Sprintf("\n\nTo fix: Ensure file exists at %s", path)
			} else if strings.HasPrefix(e.Reference, "keychain:") {
				key := e.Reference[9:]
				suggestion = fmt.Sprintf("\n\nTo fix: Add secret to keychain with key %q", key)
			}

		case profile.ErrorCategoryAccessDenied:
			suggestion = "\n\nTo fix: Check permissions or unlock the keychain/secret service"
		}
	}

	return fmt.Sprintf(
		"Error: Failed to resolve secret for integration '%s'.\n\n"+
			"Field: %s\n"+
			"Reference: %s\n"+
			"Cause: %s%s",
		e.IntegrationName,
		e.Field,
		e.Reference,
		category,
		suggestion,
	)
}

func (e *SecretResolutionError) Unwrap() error {
	return e.Cause
}
