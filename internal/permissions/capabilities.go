package permissions

import (
	"fmt"
)

// ProviderCapability represents a specific permission enforcement capability.
type ProviderCapability string

const (
	// CapPathRead indicates the provider can enforce path read permissions
	CapPathRead ProviderCapability = "paths.read"

	// CapPathWrite indicates the provider can enforce path write permissions
	CapPathWrite ProviderCapability = "paths.write"

	// CapNetwork indicates the provider can enforce network permissions
	CapNetwork ProviderCapability = "network"

	// CapSecrets indicates the provider can enforce secret access permissions
	CapSecrets ProviderCapability = "secrets"

	// CapTools indicates the provider can enforce tool filtering
	CapTools ProviderCapability = "tools"

	// CapShell indicates the provider can enforce shell command restrictions
	CapShell ProviderCapability = "shell"

	// CapEnv indicates the provider can enforce environment variable restrictions
	CapEnv ProviderCapability = "env"
)

// ProviderCapabilities describes which permission controls a provider can enforce.
type ProviderCapabilities struct {
	// ProviderName is the name of the provider (e.g., "anthropic", "openai")
	ProviderName string

	// Capabilities is the set of permission types this provider can enforce
	Capabilities map[ProviderCapability]bool

	// Notes provides additional context about enforcement limitations
	Notes string
}

// CanEnforce returns true if the provider can enforce the given capability.
func (pc *ProviderCapabilities) CanEnforce(cap ProviderCapability) bool {
	if pc.Capabilities == nil {
		return false
	}
	return pc.Capabilities[cap]
}

// CanEnforceAll returns true if the provider can enforce all the given capabilities.
func (pc *ProviderCapabilities) CanEnforceAll(caps []ProviderCapability) bool {
	for _, cap := range caps {
		if !pc.CanEnforce(cap) {
			return false
		}
	}
	return true
}

// GetUnenforceable returns the list of capabilities that cannot be enforced.
func (pc *ProviderCapabilities) GetUnenforceable(caps []ProviderCapability) []ProviderCapability {
	var unenforceable []ProviderCapability
	for _, cap := range caps {
		if !pc.CanEnforce(cap) {
			unenforceable = append(unenforceable, cap)
		}
	}
	return unenforceable
}

// CapabilityRegistry holds provider capabilities.
var CapabilityRegistry = map[string]*ProviderCapabilities{
	"anthropic": {
		ProviderName: "anthropic",
		Capabilities: map[ProviderCapability]bool{
			CapTools: true,
		},
		Notes: "Anthropic Claude supports tool filtering via the tools array in API requests. Path, network, secret, shell, and env controls must be enforced at the connector level.",
	},
	"openai": {
		ProviderName: "openai",
		Capabilities: map[ProviderCapability]bool{
			CapTools: true,
		},
		Notes: "OpenAI supports tool filtering via the tools array in API requests. Path, network, secret, shell, and env controls must be enforced at the connector level.",
	},
	"ollama": {
		ProviderName: "ollama",
		Capabilities: map[ProviderCapability]bool{
			CapTools: true,
		},
		Notes: "Ollama supports tool filtering via the tools array in API requests. Path, network, secret, shell, and env controls must be enforced at the connector level.",
	},
	"aider": {
		ProviderName: "aider",
		Capabilities: map[ProviderCapability]bool{
			CapPathRead:  true,
			CapPathWrite: true,
		},
		Notes: "Aider CLI agent has limited permission enforcement. Path controls are partially supported through file scope. Network, secret, shell, and env controls must be enforced externally.",
	},
	"codex-cli": {
		ProviderName: "codex-cli",
		Capabilities: map[ProviderCapability]bool{
			CapTools: true,
		},
		Notes: "Codex CLI supports tool filtering. Path, network, secret, shell, and env controls must be enforced at the connector level.",
	},
	"cursor": {
		ProviderName: "cursor",
		Capabilities: map[ProviderCapability]bool{
			CapPathRead:  true,
			CapPathWrite: true,
		},
		Notes: "Cursor IDE has partial path control through workspace restrictions. Other permissions must be enforced externally.",
	},
	"gemini-cli": {
		ProviderName: "gemini-cli",
		Capabilities: map[ProviderCapability]bool{
			CapTools: true,
		},
		Notes: "Gemini CLI supports tool filtering via function calling. Path, network, secret, shell, and env controls must be enforced at the connector level.",
	},
	"claude-code": {
		ProviderName: "claude-code",
		Capabilities: map[ProviderCapability]bool{
			CapPathWrite: true,
			CapNetwork:   true,
			CapSecrets:   true,
			CapTools:     true,
			CapShell:     true,
			CapEnv:       false, // Claude Code doesn't have full env var control
		},
		Notes: "Claude Code CLI agent can enforce most permissions through its built-in tools. Path read is partial (can read via tools but not fully controlled). Environment variable control is limited.",
	},
}

// GetProviderCapabilities returns the capabilities for a provider.
// Returns nil if the provider is not registered.
func GetProviderCapabilities(providerName string) *ProviderCapabilities {
	return CapabilityRegistry[providerName]
}

// ValidationResult contains the result of permission enforcement validation.
type ValidationResult struct {
	// Provider name
	Provider string

	// AllEnforceable is true if all requested permissions can be enforced
	AllEnforceable bool

	// Unenforceable lists permissions that cannot be enforced by this provider
	Unenforceable []ProviderCapability

	// Warnings contains human-readable warnings about unenforceable permissions
	Warnings []string
}

// ValidateEnforcement checks if a provider can enforce the given permission configuration.
// Returns a validation result indicating which permissions cannot be enforced.
func ValidateEnforcement(providerName string, permCtx *PermissionContext) *ValidationResult {
	result := &ValidationResult{
		Provider:       providerName,
		AllEnforceable: true,
		Unenforceable:  []ProviderCapability{},
		Warnings:       []string{},
	}

	// Get provider capabilities
	providerCaps := GetProviderCapabilities(providerName)
	if providerCaps == nil {
		// Unknown provider - assume no enforcement capabilities
		result.AllEnforceable = false
		result.Warnings = append(result.Warnings, fmt.Sprintf("Provider '%s' is not registered in the capability registry", providerName))
		return result
	}

	// If no permissions are set, nothing to validate
	if permCtx == nil {
		return result
	}

	// Check each permission type
	requiredCaps := []ProviderCapability{}

	if permCtx.Paths != nil && (len(permCtx.Paths.Read) > 0 || len(permCtx.Paths.Write) > 0) {
		if len(permCtx.Paths.Read) > 0 {
			requiredCaps = append(requiredCaps, CapPathRead)
		}
		if len(permCtx.Paths.Write) > 0 {
			requiredCaps = append(requiredCaps, CapPathWrite)
		}
	}

	if permCtx.Network != nil && (len(permCtx.Network.AllowedHosts) > 0 || len(permCtx.Network.BlockedHosts) > 0) {
		requiredCaps = append(requiredCaps, CapNetwork)
	}

	if permCtx.Secrets != nil && len(permCtx.Secrets.Allowed) > 0 {
		requiredCaps = append(requiredCaps, CapSecrets)
	}

	if permCtx.Tools != nil && (len(permCtx.Tools.Allowed) > 0 || len(permCtx.Tools.Blocked) > 0) {
		requiredCaps = append(requiredCaps, CapTools)
	}

	if permCtx.Shell != nil && (permCtx.Shell.Enabled != nil || len(permCtx.Shell.AllowedCommands) > 0) {
		requiredCaps = append(requiredCaps, CapShell)
	}

	if permCtx.Env != nil {
		requiredCaps = append(requiredCaps, CapEnv)
	}

	// Check which capabilities are unenforceable
	unenforceable := providerCaps.GetUnenforceable(requiredCaps)
	if len(unenforceable) > 0 {
		result.AllEnforceable = false
		result.Unenforceable = unenforceable

		for _, cap := range unenforceable {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Provider '%s' cannot enforce %s permissions - these will be logged but not blocked",
				providerName, cap,
			))
		}
	}

	return result
}

// AllCapabilities returns a list of all defined capabilities.
func AllCapabilities() []ProviderCapability {
	return []ProviderCapability{
		CapPathRead,
		CapPathWrite,
		CapNetwork,
		CapSecrets,
		CapTools,
		CapShell,
		CapEnv,
	}
}
