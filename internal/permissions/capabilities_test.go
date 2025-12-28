package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestProviderCapabilities(t *testing.T) {
	t.Run("CanEnforce", func(t *testing.T) {
		caps := &ProviderCapabilities{
			ProviderName: "test",
			Capabilities: map[ProviderCapability]bool{
				CapTools:    true,
				CapPathRead: true,
			},
		}

		assert.True(t, caps.CanEnforce(CapTools))
		assert.True(t, caps.CanEnforce(CapPathRead))
		assert.False(t, caps.CanEnforce(CapNetwork))
		assert.False(t, caps.CanEnforce(CapShell))
	})

	t.Run("CanEnforceAll", func(t *testing.T) {
		caps := &ProviderCapabilities{
			ProviderName: "test",
			Capabilities: map[ProviderCapability]bool{
				CapTools:    true,
				CapPathRead: true,
			},
		}

		assert.True(t, caps.CanEnforceAll([]ProviderCapability{CapTools, CapPathRead}))
		assert.False(t, caps.CanEnforceAll([]ProviderCapability{CapTools, CapNetwork}))
		assert.False(t, caps.CanEnforceAll([]ProviderCapability{CapNetwork, CapShell}))
		assert.True(t, caps.CanEnforceAll([]ProviderCapability{}))
	})

	t.Run("GetUnenforceable", func(t *testing.T) {
		caps := &ProviderCapabilities{
			ProviderName: "test",
			Capabilities: map[ProviderCapability]bool{
				CapTools: true,
			},
		}

		unenforceable := caps.GetUnenforceable([]ProviderCapability{
			CapTools,
			CapPathRead,
			CapNetwork,
		})

		assert.Len(t, unenforceable, 2)
		assert.Contains(t, unenforceable, CapPathRead)
		assert.Contains(t, unenforceable, CapNetwork)
	})
}

func TestCapabilityRegistry(t *testing.T) {
	t.Run("anthropic capabilities", func(t *testing.T) {
		caps := GetProviderCapabilities("anthropic")
		assert.NotNil(t, caps)
		assert.Equal(t, "anthropic", caps.ProviderName)
		assert.True(t, caps.CanEnforce(CapTools))
		assert.False(t, caps.CanEnforce(CapPathRead))
		assert.False(t, caps.CanEnforce(CapNetwork))
	})

	t.Run("openai capabilities", func(t *testing.T) {
		caps := GetProviderCapabilities("openai")
		assert.NotNil(t, caps)
		assert.Equal(t, "openai", caps.ProviderName)
		assert.True(t, caps.CanEnforce(CapTools))
		assert.False(t, caps.CanEnforce(CapPathRead))
	})

	t.Run("ollama capabilities", func(t *testing.T) {
		caps := GetProviderCapabilities("ollama")
		assert.NotNil(t, caps)
		assert.Equal(t, "ollama", caps.ProviderName)
		assert.True(t, caps.CanEnforce(CapTools))
		assert.False(t, caps.CanEnforce(CapShell))
	})

	t.Run("claude-code capabilities", func(t *testing.T) {
		caps := GetProviderCapabilities("claude-code")
		assert.NotNil(t, caps)
		assert.Equal(t, "claude-code", caps.ProviderName)
		assert.False(t, caps.CanEnforce(CapPathRead)) // Path read is partial, not full
		assert.True(t, caps.CanEnforce(CapPathWrite))
		assert.True(t, caps.CanEnforce(CapNetwork))
		assert.True(t, caps.CanEnforce(CapSecrets))
		assert.True(t, caps.CanEnforce(CapTools))
		assert.True(t, caps.CanEnforce(CapShell))
		assert.False(t, caps.CanEnforce(CapEnv))
	})

	t.Run("unknown provider", func(t *testing.T) {
		caps := GetProviderCapabilities("unknown")
		assert.Nil(t, caps)
	})
}

func TestValidateEnforcement(t *testing.T) {
	t.Run("no permissions - all enforceable", func(t *testing.T) {
		result := ValidateEnforcement("anthropic", nil)
		assert.True(t, result.AllEnforceable)
		assert.Empty(t, result.Unenforceable)
		assert.Empty(t, result.Warnings)
	})

	t.Run("unknown provider", func(t *testing.T) {
		permCtx := &PermissionContext{
			Tools: &workflow.ToolPermissions{
				Allowed: []string{"file.*"},
			},
		}

		result := ValidateEnforcement("unknown-provider", permCtx)
		assert.False(t, result.AllEnforceable)
		assert.Len(t, result.Warnings, 1)
		assert.Contains(t, result.Warnings[0], "not registered")
	})

	t.Run("anthropic - tools only (enforceable)", func(t *testing.T) {
		permCtx := &PermissionContext{
			Tools: &workflow.ToolPermissions{
				Allowed: []string{"file.*"},
			},
		}

		result := ValidateEnforcement("anthropic", permCtx)
		assert.True(t, result.AllEnforceable)
		assert.Empty(t, result.Unenforceable)
		assert.Empty(t, result.Warnings)
	})

	t.Run("anthropic - paths not enforceable", func(t *testing.T) {
		permCtx := &PermissionContext{
			Paths: &workflow.PathPermissions{
				Read: []string{"src/**"},
			},
		}

		result := ValidateEnforcement("anthropic", permCtx)
		assert.False(t, result.AllEnforceable)
		assert.Contains(t, result.Unenforceable, CapPathRead)
		assert.Len(t, result.Warnings, 1)
		assert.Contains(t, result.Warnings[0], "cannot enforce paths.read")
	})

	t.Run("anthropic - mixed enforceable and unenforceable", func(t *testing.T) {
		permCtx := &PermissionContext{
			Tools: &workflow.ToolPermissions{
				Allowed: []string{"file.*"},
			},
			Paths: &workflow.PathPermissions{
				Read: []string{"src/**"},
			},
			Network: &workflow.NetworkPermissions{
				AllowedHosts: []string{"api.example.com"},
			},
		}

		result := ValidateEnforcement("anthropic", permCtx)
		assert.False(t, result.AllEnforceable)
		assert.Contains(t, result.Unenforceable, CapPathRead)
		assert.Contains(t, result.Unenforceable, CapNetwork)
		assert.NotContains(t, result.Unenforceable, CapTools)
		assert.Len(t, result.Warnings, 2)
	})

	t.Run("claude-code - most permissions enforceable", func(t *testing.T) {
		permCtx := &PermissionContext{
			Paths: &workflow.PathPermissions{
				Read:  []string{"src/**"},
				Write: []string{"out/**"},
			},
			Network: &workflow.NetworkPermissions{
				AllowedHosts: []string{"api.example.com"},
			},
			Tools: &workflow.ToolPermissions{
				Allowed: []string{"file.*"},
			},
			Shell: &workflow.ShellPermissions{
				Enabled: boolPtr(true),
			},
		}

		result := ValidateEnforcement("claude-code", permCtx)
		// Path read is partial (not fully controlled), so it's unenforceable
		assert.False(t, result.AllEnforceable)
		assert.Contains(t, result.Unenforceable, CapPathRead)
		assert.Len(t, result.Warnings, 1)
		assert.Contains(t, result.Warnings[0], "cannot enforce paths.read")
	})

	t.Run("claude-code - env not enforceable", func(t *testing.T) {
		permCtx := &PermissionContext{
			Env: &workflow.EnvPermissions{
				Inherit: false,
			},
		}

		result := ValidateEnforcement("claude-code", permCtx)
		assert.False(t, result.AllEnforceable)
		assert.Contains(t, result.Unenforceable, CapEnv)
		assert.Len(t, result.Warnings, 1)
		assert.Contains(t, result.Warnings[0], "cannot enforce env")
	})

	t.Run("openai - only tools enforceable", func(t *testing.T) {
		permCtx := &PermissionContext{
			Tools: &workflow.ToolPermissions{
				Allowed: []string{"*"},
				Blocked: []string{"shell.*"},
			},
			Secrets: &workflow.SecretPermissions{
				Allowed: []string{"API_*"},
			},
		}

		result := ValidateEnforcement("openai", permCtx)
		assert.False(t, result.AllEnforceable)
		assert.Contains(t, result.Unenforceable, CapSecrets)
		assert.NotContains(t, result.Unenforceable, CapTools)
		assert.Len(t, result.Warnings, 1)
	})

	t.Run("empty permission contexts are ignored", func(t *testing.T) {
		permCtx := &PermissionContext{
			Paths: &workflow.PathPermissions{
				Read:  []string{}, // Empty - should be ignored
				Write: []string{},
			},
			Network: &workflow.NetworkPermissions{
				AllowedHosts: []string{}, // Empty - should be ignored
			},
		}

		result := ValidateEnforcement("anthropic", permCtx)
		assert.True(t, result.AllEnforceable)
		assert.Empty(t, result.Unenforceable)
	})
}

func TestAllCapabilities(t *testing.T) {
	caps := AllCapabilities()
	assert.Len(t, caps, 7)
	assert.Contains(t, caps, CapPathRead)
	assert.Contains(t, caps, CapPathWrite)
	assert.Contains(t, caps, CapNetwork)
	assert.Contains(t, caps, CapSecrets)
	assert.Contains(t, caps, CapTools)
	assert.Contains(t, caps, CapShell)
	assert.Contains(t, caps, CapEnv)
}
