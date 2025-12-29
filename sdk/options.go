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

// WithOpenAIProvider creates an OpenAI provider.
// Note: OpenAI provider is not fully implemented in Phase 1.
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithOpenAIProvider(os.Getenv("OPENAI_API_KEY")),
//	)
func WithOpenAIProvider(apiKey string) Option {
	return func(s *SDK) error {
		if apiKey == "" {
			return fmt.Errorf("OpenAI API key cannot be empty")
		}

		provider, err := providers.NewOpenAIProvider(apiKey)
		if err != nil {
			return fmt.Errorf("create OpenAI provider: %w", err)
		}

		s.providers.Register(provider)
		return nil
	}
}

// WithOllamaProvider creates an Ollama provider.
// Ollama runs locally and does not require an API key.
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithOllamaProvider("http://localhost:11434"),
//	)
func WithOllamaProvider(baseURL string) Option {
	return func(s *SDK) error {
		if baseURL == "" {
			return fmt.Errorf("Ollama base URL cannot be empty")
		}

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

// WithCostLimit sets a maximum cost limit for workflow execution.
// The limit is enforced per workflow run. Set to 0 for no limit.
// Can be overridden per-run with WithRunCostLimit().
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithAnthropicProvider(apiKey),
//		sdk.WithCostLimit(10.0), // $10 max per workflow
//	)
func WithCostLimit(maxCost float64) Option {
	return func(s *SDK) error {
		if maxCost < 0 {
			return fmt.Errorf("cost limit cannot be negative: %f", maxCost)
		}
		s.costLimit = maxCost
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
		// TODO: Implement in Phase 2
		// This will import internal/action/* packages and register them
		return fmt.Errorf("WithBuiltinActions not implemented yet (Phase 2)")
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
		// TODO: Implement in Phase 2
		// This will import internal/integration/* packages and register them
		return fmt.Errorf("WithBuiltinIntegrations not implemented yet (Phase 2)")
	}
}

// WithMCPServer connects a user-provided MCP server.
// The server is not connected until ConnectMCP() is called or the server is
// enabled via WithMCPServers() run option.
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

// WithConductorMCP enables the built-in Conductor MCP server.
// This exposes actions/integrations via the MCP protocol.
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithAnthropicProvider(apiKey),
//		sdk.WithConductorMCP(),
//	)
func WithConductorMCP() Option {
	return func(s *SDK) error {
		// TODO: Implement in Phase 2
		// This will configure the built-in MCP server
		return fmt.Errorf("WithConductorMCP not implemented yet (Phase 2)")
	}
}
