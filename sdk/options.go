package sdk

import (
	"fmt"
	"log/slog"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers"
)

// Option is a functional option for SDK construction.
type Option func(*SDK) error

// WithProvider registers a custom LLM provider.
// The provider must implement the llm.Provider interface.
// Returns an error if the provider is nil or the name is empty.
//
// Example:
//
//	provider := &MyCustomProvider{}
//	s, err := sdk.New(sdk.WithProvider("custom", provider))
func WithProvider(name string, provider llm.Provider) Option {
	return func(s *SDK) error {
		if provider == nil {
			return fmt.Errorf("provider cannot be nil")
		}
		if name == "" {
			return fmt.Errorf("provider name cannot be empty")
		}

		s.providers.Register(provider)
		return nil
	}
}

// WithAnthropicProvider creates an Anthropic provider from an API key.
// The API key is validated on first use, not at registration time.
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY")),
//	)
func WithAnthropicProvider(apiKey string) Option {
	return func(s *SDK) error {
		if apiKey == "" {
			return fmt.Errorf("Anthropic API key cannot be empty")
		}

		provider, err := providers.NewAnthropicProvider(apiKey)
		if err != nil {
			return fmt.Errorf("create Anthropic provider: %w", err)
		}

		s.providers.Register(provider)
		return nil
	}
}

// WithOllamaProvider creates an Ollama provider with a custom base URL.
// If baseURL is empty, defaults to http://localhost:11434.
// Ollama is a local LLM provider that doesn't require API keys.
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithOllamaProvider("http://localhost:11434"),
//	)
func WithOllamaProvider(baseURL string) Option {
	return func(s *SDK) error {
		provider, err := providers.NewOllamaProvider(baseURL)
		if err != nil {
			return fmt.Errorf("create Ollama provider: %w", err)
		}

		s.providers.Register(provider)
		return nil
	}
}

// WithLogger sets a custom structured logger.
// If not set, logs go to slog.Default().
//
// Example:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
//		Level: slog.LevelInfo,
//	}))
//	s, err := sdk.New(sdk.WithLogger(logger))
func WithLogger(logger *slog.Logger) Option {
	return func(s *SDK) error {
		if logger == nil {
			return fmt.Errorf("logger cannot be nil")
		}
		s.logger = logger
		return nil
	}
}

// WithTokenLimit sets a maximum token limit for workflow execution.
// The limit is enforced per workflow run. Set to 0 for no limit.
// Can be overridden per-run with WithRunTokenLimit().
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithAnthropicProvider(apiKey),
//		sdk.WithTokenLimit(100000), // 100k tokens max per workflow
//	)
func WithTokenLimit(maxTokens int) Option {
	return func(s *SDK) error {
		if maxTokens < 0 {
			return fmt.Errorf("token limit cannot be negative: %d", maxTokens)
		}
		s.tokenLimit = maxTokens
		return nil
	}
}

// WithStore configures workflow state persistence.
// Implement a custom Store to persist run history, build audit logs,
// or export execution data to monitoring systems.
//
// Example:
//
//	store := &PostgresStore{db: db}
//	s, err := sdk.New(sdk.WithStore(store))
func WithStore(store Store) Option {
	return func(s *SDK) error {
		if store == nil {
			return fmt.Errorf("store cannot be nil")
		}
		s.store = store
		return nil
	}
}

// WithInMemoryStore uses in-memory storage for workflow runs (default).
// The in-memory store limits history to 1000 most recent runs.
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithAnthropicProvider(apiKey),
//		sdk.WithInMemoryStore(), // Explicit default
//	)
func WithInMemoryStore() Option {
	return func(s *SDK) error {
		s.store = newInMemoryStore()
		return nil
	}
}

// WithTool registers a custom tool at SDK construction time.
// Tools can also be registered later with SDK.RegisterTool().
//
// Example:
//
//	tool := sdk.FuncTool("get_weather", "Get weather", schema, fn)
//	s, err := sdk.New(sdk.WithTool("get_weather", tool))
func WithTool(name string, tool Tool) Option {
	return func(s *SDK) error {
		if name == "" {
			return fmt.Errorf("tool name cannot be empty")
		}
		if tool == nil {
			return fmt.Errorf("tool cannot be nil")
		}

		// Convert SDK tool to pkg/tools.Tool and register
		pkgTool := &sdkToolAdapter{tool: tool}
		if err := s.toolRegistry.Register(pkgTool); err != nil {
			return fmt.Errorf("register tool %s: %w", name, err)
		}

		return nil
	}
}

// WithBuiltinActions enables built-in actions (file, shell, http, transform, utility).
// Actions are local operations that can also be used as LLM tools in agent steps.
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithAnthropicProvider(apiKey),
//		sdk.WithBuiltinActions(), // Enable file, shell, etc.
//	)
func WithBuiltinActions() Option {
	return func(s *SDK) error {
		s.builtinActionsEnabled = true
		return nil
	}
}

// WithBuiltinIntegrations enables built-in integrations (GitHub, Slack, Jira, Discord, Jenkins).
// Integrations are external service APIs that require credentials at runtime.
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithAnthropicProvider(apiKey),
//		sdk.WithBuiltinIntegrations(), // Enable GitHub, Slack, etc.
//	)
func WithBuiltinIntegrations() Option {
	return func(s *SDK) error {
		s.builtinIntegrationsEnabled = true
		return nil
	}
}

// WithMCPServer registers configuration for a user-provided MCP server.
// This stores the configuration for use by workflow execution.
//
// Example:
//
//	config := sdk.MCPConfig{
//		Transport: "stdio",
//		Command:   "mcp-server-gh",
//		Args:      []string{"--token", token},
//	}
//	s, err := sdk.New(sdk.WithMCPServer("github", config))
func WithMCPServer(name string, config MCPConfig) Option {
	return func(s *SDK) error {
		if name == "" {
			return fmt.Errorf("MCP server name cannot be empty")
		}

		s.mcpMu.Lock()
		defer s.mcpMu.Unlock()

		s.mcpServers[name] = config
		return nil
	}
}

