// Package providers registers all built-in LLM provider factories.
//
// Import this package to register all provider factories with the global registry:
//
//	import _ "github.com/tombee/conductor/pkg/llm/providers"
//
// This registers factories but does not instantiate providers.
// Call llm.Activate() to instantiate providers based on configuration.
package providers

import (
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

func init() {
	// Register all built-in provider factories.
	// Factories are registered at import time but not instantiated.
	// Call llm.Activate() to instantiate based on config.

	// Claude Code - CLI-based provider using the Claude CLI
	llm.RegisterFactory("claude-code", claudecode.NewWithCredentials)

	// Anthropic - API-based provider for Claude models
	llm.RegisterFactory("anthropic", NewAnthropicWithCredentials)
}
