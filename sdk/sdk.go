package sdk

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/tools"
)

// SDK is the main entry point for workflow execution.
// It manages providers, tools, workflows, and execution state.
// Each SDK instance maintains completely isolated state with no shared globals.
type SDK struct {
	// Provider registry for LLM providers
	providers *llm.Registry

	// Tool registry for custom tools
	toolRegistry *tools.Registry

	// Logger for structured logging
	logger *slog.Logger

	// Cost limit for workflow execution (0 = no limit)
	costLimit float64

	// Store for workflow state persistence
	store Store

	// Event handlers by event type
	eventHandlers map[EventType][]EventHandler
	eventMu       sync.RWMutex

	// MCP server configurations
	mcpServers map[string]MCPConfig
	mcpMu      sync.RWMutex

	// Cleanup resources
	closeMu sync.Mutex
	closed  bool
}

// New creates a new SDK instance with the given options.
// Returns an error if any option fails to apply.
//
// Example:
//
//	s, err := sdk.New(
//		sdk.WithAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY")),
//		sdk.WithLogger(slog.Default()),
//		sdk.WithCostLimit(10.0),
//	)
//	if err != nil {
//		return err
//	}
//	defer s.Close()
func New(opts ...Option) (*SDK, error) {
	s := &SDK{
		providers:     llm.NewRegistry(),
		toolRegistry:  tools.NewRegistry(),
		logger:        slog.Default(),
		costLimit:     0, // No limit by default
		store:         newInMemoryStore(),
		eventHandlers: make(map[EventType][]EventHandler),
		mcpServers:    make(map[string]MCPConfig),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return s, nil
}

// Close releases resources held by the SDK.
// This includes disconnecting MCP servers, cleaning up providers, and zeroing credentials.
// Returns an error if cleanup fails.
//
// Close is safe to call multiple times.
func (s *SDK) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return nil
	}

	var errs []error

	// Disconnect all MCP servers
	s.mcpMu.RLock()
	serverNames := make([]string, 0, len(s.mcpServers))
	for name := range s.mcpServers {
		serverNames = append(serverNames, name)
	}
	s.mcpMu.RUnlock()

	for _, name := range serverNames {
		if err := s.DisconnectMCP(name); err != nil {
			errs = append(errs, fmt.Errorf("disconnect MCP %s: %w", name, err))
		}
	}

	// TODO: Zero credential memory (NFR12)
	// This will be implemented as part of credential handling

	s.closed = true

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

// OnEvent registers an event handler for the specified event type.
// Multiple handlers can be registered for the same event type.
// Handlers are called synchronously in registration order during workflow execution.
//
// If a handler panics, the panic is recovered and logged, but subsequent handlers
// and workflow execution continue.
//
// Example:
//
//	s.OnEvent(sdk.EventStepStarted, func(ctx context.Context, e *sdk.Event) {
//		log.Printf("Step started: %s", e.StepID)
//	})
func (s *SDK) OnEvent(eventType EventType, handler EventHandler) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()

	s.eventHandlers[eventType] = append(s.eventHandlers[eventType], handler)
}

// emitEvent emits an event to all registered handlers.
// Panics in handlers are recovered and logged.
func (s *SDK) emitEvent(ctx context.Context, event *Event) {
	s.eventMu.RLock()
	handlers := s.eventHandlers[event.Type]
	s.eventMu.RUnlock()

	for _, handler := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("event handler panic",
						"event_type", event.Type,
						"panic", r,
					)
				}
			}()
			handler(ctx, event)
		}()
	}
}

// NewWorkflow starts building a new programmatic workflow.
// Returns a WorkflowBuilder for fluent workflow definition.
//
// Example:
//
//	wf, err := s.NewWorkflow("code-review").
//		Input("code", sdk.TypeString).
//		Step("analyze").LLM().
//			Model("claude-sonnet-4-20250514").
//			Prompt("Review this code: {{.inputs.code}}").
//			Done().
//		Build()
func (s *SDK) NewWorkflow(name string) *WorkflowBuilder {
	return &WorkflowBuilder{
		sdk:    s,
		name:   name,
		inputs: make(map[string]*InputDef),
		steps:  make([]*stepDef, 0),
	}
}

// llmRegistry returns the internal LLM provider registry for testing
func (s *SDK) llmRegistry() *llm.Registry {
	return s.providers
}
