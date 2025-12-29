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

// Package binding implements the binding resolution system.
//
// This package provides the BindingResolver, which connects workflow requirements
// to runtime values by following a strict resolution order:
//
//  1. Profile binding (explicit configuration)
//  2. Inline workflow definition (backward compatibility)
//  3. Environment variable (if inherit_env enabled)
//  4. Default value (if defined in workflow)
//  5. Error: required binding missing
//
// The resolver integrates with the secret provider registry to resolve
// secret references (${VAR}, env:VAR, file:/path) into actual values.
package binding

import (
	"context"
	"fmt"
	"strings"

	"github.com/tombee/conductor/pkg/profile"
	"github.com/tombee/conductor/pkg/workflow"
)

// Resolver implements the binding resolution system.
//
// It resolves workflow requirements into concrete runtime values by following
// the resolution order: profile > inline > env > default > error.
//
// The resolver uses the secret provider registry to resolve secret references
// and maintains a runtime context for audit logging and error reporting.
type Resolver struct {
	// secretRegistry routes secret references to appropriate providers
	secretRegistry profile.SecretProviderRegistry

	// inheritEnv controls whether environment variables can be accessed
	// This is set from the profile's inherit_env configuration
	inheritEnv profile.InheritEnvConfig
}

// NewResolver creates a new binding resolver.
//
// The secret registry must be configured with the appropriate providers
// (env, file, etc.) before creating the resolver.
func NewResolver(secretRegistry profile.SecretProviderRegistry, inheritEnv profile.InheritEnvConfig) *Resolver {
	return &Resolver{
		secretRegistry: secretRegistry,
		inheritEnv:     inheritEnv,
	}
}

// ResolutionContext contains the context needed for binding resolution.
type ResolutionContext struct {
	// Profile contains the runtime bindings from the selected profile
	Profile *profile.Profile

	// Workflow contains the workflow definition with inline bindings
	Workflow *workflow.Definition

	// RunID identifies the run for audit logging
	RunID string

	// Workspace identifies the workspace context
	Workspace string
}

// ResolvedBinding represents a successfully resolved binding.
type ResolvedBinding struct {
	// ConnectorBindings maps integration names to resolved configurations
	ConnectorBindings map[string]ResolvedConnectorBinding

	// MCPServerBindings maps MCP server names to resolved configurations
	MCPServerBindings map[string]ResolvedMCPServerBinding
}

// ResolvedConnectorBinding contains resolved integration configuration.
type ResolvedConnectorBinding struct {
	// Auth contains resolved authentication credentials (secrets resolved to values)
	Auth ResolvedAuthBinding

	// BaseURL is the integration base URL (optional)
	BaseURL string

	// Headers are additional HTTP headers (optional)
	Headers map[string]string

	// Source indicates where this binding came from (for debugging)
	Source BindingSource
}

// ResolvedAuthBinding contains resolved authentication credentials.
type ResolvedAuthBinding struct {
	// Token for bearer authentication (resolved from secret reference)
	Token string

	// Username for basic authentication
	Username string

	// Password for basic authentication (resolved from secret reference)
	Password string

	// Header name for API key authentication
	Header string

	// Value for API key authentication (resolved from secret reference)
	Value string
}

// ResolvedMCPServerBinding contains resolved MCP server configuration.
type ResolvedMCPServerBinding struct {
	// Command is the executable to run
	Command string

	// Args are command-line arguments
	Args []string

	// Env are environment variables (secrets resolved to values)
	Env map[string]string

	// Timeout is the default timeout for tool calls in seconds
	Timeout int

	// Source indicates where this binding came from (for debugging)
	Source BindingSource
}

// BindingSource indicates where a binding value originated.
type BindingSource string

const (
	// SourceProfile indicates the binding came from a profile
	SourceProfile BindingSource = "profile"

	// SourceInline indicates the binding came from inline workflow definition
	SourceInline BindingSource = "inline"

	// SourceEnvironment indicates the binding came from environment variables
	SourceEnvironment BindingSource = "environment"

	// SourceDefault indicates the binding came from a default value
	SourceDefault BindingSource = "default"
)

// Resolve resolves all bindings for a workflow run.
//
// This is the main entry point for binding resolution. It:
// 1. Validates workflow requirements against profile bindings
// 2. Resolves all secret references to actual values
// 3. Returns resolved bindings or an error if required bindings are missing
//
// Resolution follows the precedence order:
//   profile > inline > environment > default > error
func (r *Resolver) Resolve(ctx context.Context, resCtx *ResolutionContext) (*ResolvedBinding, error) {
	if resCtx == nil {
		return nil, fmt.Errorf("resolution context is nil")
	}

	if resCtx.Workflow == nil {
		return nil, fmt.Errorf("workflow definition is nil")
	}

	resolved := &ResolvedBinding{
		ConnectorBindings: make(map[string]ResolvedConnectorBinding),
		MCPServerBindings: make(map[string]ResolvedMCPServerBinding),
	}

	// If workflow has no requirements, check for inline definitions
	if resCtx.Workflow.Requires == nil {
		// Use inline definitions if present (backward compatibility)
		return r.resolveInlineBindings(ctx, resCtx, resolved)
	}

	// Resolve integration requirements
	if err := r.resolveConnectorRequirements(ctx, resCtx, resolved); err != nil {
		return nil, err
	}

	// Resolve MCP server requirements
	if err := r.resolveMCPServerRequirements(ctx, resCtx, resolved); err != nil {
		return nil, err
	}

	return resolved, nil
}

// resolveConnectorRequirements resolves all integration requirements.
func (r *Resolver) resolveConnectorRequirements(ctx context.Context, resCtx *ResolutionContext, resolved *ResolvedBinding) error {
	if resCtx.Workflow.Requires == nil || resCtx.Workflow.Requires.Integrations == nil {
		return nil
	}

	for _, req := range resCtx.Workflow.Requires.Integrations {
		binding, source, err := r.resolveConnectorBinding(ctx, resCtx, req.Name)
		if err != nil {
			if req.Optional {
				// Optional integration missing - skip with warning
				// TODO: Add warning logging
				continue
			}
			return profile.NewBindingError(
				profile.ErrorCategoryNotFound,
				fmt.Sprintf("connectors.%s", req.Name),
				resCtx.Profile.Name,
				fmt.Sprintf("workflow requires integration %q but no binding found", req.Name),
			)
		}

		// Resolve secrets in the binding
		resolvedBinding, err := r.resolveConnectorSecrets(ctx, resCtx, binding)
		if err != nil {
			return fmt.Errorf("failed to resolve secrets for integration %q: %w", req.Name, err)
		}

		resolvedBinding.Source = source
		resolved.ConnectorBindings[req.Name] = *resolvedBinding
	}

	return nil
}

// resolveConnectorBinding finds the integration binding following resolution order.
func (r *Resolver) resolveConnectorBinding(ctx context.Context, resCtx *ResolutionContext, name string) (*profile.IntegrationBinding, BindingSource, error) {
	// 1. Check profile binding (highest priority)
	if resCtx.Profile != nil && resCtx.Profile.Bindings.Integrations != nil {
		if binding, exists := resCtx.Profile.Bindings.Integrations[name]; exists {
			return &binding, SourceProfile, nil
		}
	}

	// 2. Check inline workflow definition (backward compatibility)
	if resCtx.Workflow.Integrations != nil {
		if connDef, exists := resCtx.Workflow.Integrations[name]; exists {
			// Convert IntegrationDefinition to ConnectorBinding
			binding := &profile.IntegrationBinding{
				BaseURL: connDef.BaseURL,
				Headers: connDef.Headers,
			}
			if connDef.Auth != nil {
				binding.Auth = profile.AuthBinding{
					Token:    connDef.Auth.Token,
					Username: connDef.Auth.Username,
					Password: connDef.Auth.Password,
					Header:   connDef.Auth.Header,
					Value:    connDef.Auth.Value,
				}
			}
			return binding, SourceInline, nil
		}
	}

	// 3. Environment variable access (if inherit_env enabled)
	// For connectors, environment-based auth is handled through secret resolution
	// No direct environment binding for integration configuration

	// 4. No default value support for connectors

	// 5. Not found
	return nil, "", fmt.Errorf("integration %q not found", name)
}

// resolveConnectorSecrets resolves all secret references in an integration binding.
func (r *Resolver) resolveConnectorSecrets(ctx context.Context, resCtx *ResolutionContext, binding *profile.IntegrationBinding) (*ResolvedConnectorBinding, error) {
	resolved := &ResolvedConnectorBinding{
		BaseURL: binding.BaseURL,
		Headers: binding.Headers,
	}

	// Resolve auth secrets
	if binding.Auth.Token != "" {
		token, err := r.resolveSecret(ctx, binding.Auth.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve auth token: %w", err)
		}
		resolved.Auth.Token = token
	}

	if binding.Auth.Username != "" {
		// Username may or may not be a secret - try to resolve, fall back to literal
		username, err := r.resolveSecret(ctx, binding.Auth.Username)
		if err != nil {
			// If it's not a secret reference, use as literal value
			username = binding.Auth.Username
		}
		resolved.Auth.Username = username
	}

	if binding.Auth.Password != "" {
		password, err := r.resolveSecret(ctx, binding.Auth.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve auth password: %w", err)
		}
		resolved.Auth.Password = password
	}

	if binding.Auth.Header != "" {
		// Header name is not a secret
		resolved.Auth.Header = binding.Auth.Header
	}

	if binding.Auth.Value != "" {
		value, err := r.resolveSecret(ctx, binding.Auth.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve auth value: %w", err)
		}
		resolved.Auth.Value = value
	}

	return resolved, nil
}

// resolveMCPServerRequirements resolves all MCP server requirements.
func (r *Resolver) resolveMCPServerRequirements(ctx context.Context, resCtx *ResolutionContext, resolved *ResolvedBinding) error {
	if resCtx.Workflow.Requires == nil || resCtx.Workflow.Requires.MCPServers == nil {
		return nil
	}

	for _, req := range resCtx.Workflow.Requires.MCPServers {
		binding, source, err := r.resolveMCPServerBinding(ctx, resCtx, req.Name)
		if err != nil {
			return profile.NewBindingError(
				profile.ErrorCategoryNotFound,
				fmt.Sprintf("mcp_servers.%s", req.Name),
				resCtx.Profile.Name,
				fmt.Sprintf("workflow requires MCP server %q but no binding found", req.Name),
			)
		}

		// Resolve secrets in the binding
		resolvedBinding, err := r.resolveMCPServerSecrets(ctx, resCtx, binding)
		if err != nil {
			return fmt.Errorf("failed to resolve secrets for MCP server %q: %w", req.Name, err)
		}

		resolvedBinding.Source = source
		resolved.MCPServerBindings[req.Name] = *resolvedBinding
	}

	return nil
}

// resolveMCPServerBinding finds the MCP server binding following resolution order.
func (r *Resolver) resolveMCPServerBinding(ctx context.Context, resCtx *ResolutionContext, name string) (*profile.MCPServerBinding, BindingSource, error) {
	// 1. Check profile binding (highest priority)
	if resCtx.Profile != nil && resCtx.Profile.Bindings.MCPServers != nil {
		if binding, exists := resCtx.Profile.Bindings.MCPServers[name]; exists {
			return &binding, SourceProfile, nil
		}
	}

	// 2. Check inline workflow definition (backward compatibility)
	if resCtx.Workflow.MCPServers != nil {
		for _, mcpCfg := range resCtx.Workflow.MCPServers {
			if mcpCfg.Name == name {
				// Convert MCPServerConfig to MCPServerBinding
				// MCPServerConfig.Env is []string in "KEY=value" format
				// MCPServerBinding.Env is map[string]string
				envMap := make(map[string]string)
				for _, envPair := range mcpCfg.Env {
					// Parse KEY=value format
					parts := strings.SplitN(envPair, "=", 2)
					if len(parts) == 2 {
						envMap[parts[0]] = parts[1]
					}
				}

				binding := &profile.MCPServerBinding{
					Command: mcpCfg.Command,
					Args:    mcpCfg.Args,
					Env:     envMap,
					Timeout: mcpCfg.Timeout,
				}
				return binding, SourceInline, nil
			}
		}
	}

	// 3. Environment variable access (if inherit_env enabled)
	// No direct environment binding for MCP server configuration

	// 4. No default value support for MCP servers

	// 5. Not found
	return nil, "", fmt.Errorf("MCP server %q not found", name)
}

// resolveMCPServerSecrets resolves all secret references in an MCP server binding.
func (r *Resolver) resolveMCPServerSecrets(ctx context.Context, resCtx *ResolutionContext, binding *profile.MCPServerBinding) (*ResolvedMCPServerBinding, error) {
	resolved := &ResolvedMCPServerBinding{
		Command: binding.Command,
		Args:    binding.Args,
		Env:     make(map[string]string),
		Timeout: binding.Timeout,
	}

	// Resolve environment variable secrets
	for key, value := range binding.Env {
		resolvedValue, err := r.resolveSecret(ctx, value)
		if err != nil {
			// If it's not a secret reference, use as literal value
			resolvedValue = value
		}
		resolved.Env[key] = resolvedValue
	}

	return resolved, nil
}

// resolveInlineBindings handles workflows without requires section (backward compatibility).
func (r *Resolver) resolveInlineBindings(ctx context.Context, resCtx *ResolutionContext, resolved *ResolvedBinding) (*ResolvedBinding, error) {
	// Process inline connectors
	for name, connDef := range resCtx.Workflow.Integrations {
		binding := &profile.IntegrationBinding{
			BaseURL: connDef.BaseURL,
			Headers: connDef.Headers,
		}
		if connDef.Auth != nil {
			binding.Auth = profile.AuthBinding{
				Token:    connDef.Auth.Token,
				Username: connDef.Auth.Username,
				Password: connDef.Auth.Password,
				Header:   connDef.Auth.Header,
				Value:    connDef.Auth.Value,
			}
		}

		resolvedBinding, err := r.resolveConnectorSecrets(ctx, resCtx, binding)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve inline integration %q: %w", name, err)
		}

		resolvedBinding.Source = SourceInline
		resolved.ConnectorBindings[name] = *resolvedBinding
	}

	// Process inline MCP servers
	for _, mcpCfg := range resCtx.Workflow.MCPServers {
		// Convert []string env to map[string]string
		envMap := make(map[string]string)
		for _, envPair := range mcpCfg.Env {
			// Parse KEY=value format
			parts := strings.SplitN(envPair, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}

		binding := &profile.MCPServerBinding{
			Command: mcpCfg.Command,
			Args:    mcpCfg.Args,
			Env:     envMap,
			Timeout: mcpCfg.Timeout,
		}

		resolvedBinding, err := r.resolveMCPServerSecrets(ctx, resCtx, binding)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve inline MCP server %q: %w", mcpCfg.Name, err)
		}

		resolvedBinding.Source = SourceInline
		resolved.MCPServerBindings[mcpCfg.Name] = *resolvedBinding
	}

	return resolved, nil
}

// resolveSecret resolves a single secret reference to its value.
//
// This method:
// 1. Detects if the value is a secret reference (${VAR}, env:VAR, file:/path)
// 2. Routes to the appropriate secret provider via the registry
// 3. Returns the resolved value or an error
//
// Non-secret values (plain strings) are returned as-is for backward compatibility.
func (r *Resolver) resolveSecret(ctx context.Context, reference string) (string, error) {
	if reference == "" {
		return "", nil
	}

	// Check if this looks like a secret reference
	if !isSecretReference(reference) {
		// Not a secret reference - return as literal value
		return reference, nil
	}

	// Resolve through secret registry
	value, err := r.secretRegistry.Resolve(ctx, reference)
	if err != nil {
		return "", err
	}

	return value, nil
}

// isSecretReference checks if a value looks like a secret reference.
//
// Secret references match one of these patterns:
//   - ${VAR_NAME}        (legacy environment variable syntax)
//   - env:VAR_NAME       (explicit environment variable)
//   - file:/path/to/file (file-based secret)
//   - vault:path#field   (future: Vault secret)
func isSecretReference(value string) bool {
	// Check for ${VAR} syntax
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return true
	}

	// Check for scheme:reference syntax
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 {
			scheme := parts[0]
			// Known secret provider schemes
			knownSchemes := []string{"env", "file", "vault", "1password", "aws-secrets"}
			for _, known := range knownSchemes {
				if scheme == known {
					return true
				}
			}
		}
	}

	return false
}
