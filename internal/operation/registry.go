package operation

import (
	"context"
	"fmt"
	"sync"

	"github.com/tombee/conductor/pkg/workflow"
)

// Registry manages a collection of operation providers for a workflow.
type Registry struct {
	mu         sync.RWMutex
	providers  map[string]Connector
	config     *Config
}

// NewRegistry creates a new operation registry.
func NewRegistry(config *Config) *Registry {
	if config == nil {
		config = DefaultConfig()
	}

	// Initialize transport registry if not provided
	if config.TransportRegistry == nil {
		config.TransportRegistry = NewDefaultTransportRegistry()
	}

	return &Registry{
		providers: make(map[string]Connector),
		config:    config,
	}
}

// LoadFromDefinition loads all integrations from a workflow definition.
func (r *Registry) LoadFromDefinition(def *workflow.Definition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing providers
	r.providers = make(map[string]Connector)

	// Load each integration definition
	for name, integrationDef := range def.Integrations {
		// Ensure name is set (it should be from validation)
		integrationDef.Name = name

		// Create integration
		provider, err := New(&integrationDef, r.config)
		if err != nil {
			return fmt.Errorf("failed to create integration %q: %w", name, err)
		}

		r.providers[name] = provider
	}

	return nil
}

// Get retrieves an operation provider by name.
func (r *Registry) Get(name string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, &Error{
			Type:        ErrorTypeValidation,
			Message:     fmt.Sprintf("operation provider %q not found", name),
			SuggestText: "Check that the integration or action is defined in the workflow",
		}
	}

	return provider, nil
}

// Execute runs an operation.
// The reference should be in format "provider_name.operation_name".
func (r *Registry) Execute(ctx context.Context, reference string, inputs map[string]interface{}) (*Result, error) {
	// Parse reference (provider_name.operation_name)
	providerName, operationName, err := parseReference(reference)
	if err != nil {
		return nil, err
	}

	// Get provider
	provider, err := r.Get(providerName)
	if err != nil {
		return nil, err
	}

	// Execute operation
	return provider.Execute(ctx, operationName, inputs)
}

// List returns the names of all registered operation providers.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

// Register adds an operation provider to the registry.
func (r *Registry) Register(name string, provider Connector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

// LoadBuiltins loads all builtin actions (file, shell, transform, utility, http).
func (r *Registry) LoadBuiltins(config *BuiltinConfig) error {
	for name := range builtinNames {
		action, err := NewBuiltin(name, config)
		if err != nil {
			return fmt.Errorf("failed to load builtin action %q: %w", name, err)
		}
		r.Register(name, action)
	}
	return nil
}

// NewBuiltinRegistry creates a registry with all builtin actions pre-loaded.
func NewBuiltinRegistry(config *BuiltinConfig) (*Registry, error) {
	registry := NewRegistry(nil)
	if err := registry.LoadBuiltins(config); err != nil {
		return nil, err
	}
	return registry, nil
}

// AsWorkflowRegistry returns a wrapper that implements workflow.OperationRegistry.
func (r *Registry) AsWorkflowRegistry() workflow.OperationRegistry {
	return &workflowRegistryAdapter{r}
}

// workflowRegistryAdapter wraps Registry to implement workflow.OperationRegistry.
type workflowRegistryAdapter struct {
	registry *Registry
}

// Execute implements workflow.OperationRegistry.
func (a *workflowRegistryAdapter) Execute(ctx context.Context, reference string, inputs map[string]interface{}) (workflow.OperationResult, error) {
	result, err := a.registry.Execute(ctx, reference, inputs)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// parseReference splits an operation reference into provider and operation names.
// Expected format: "provider_name.operation_name"
func parseReference(reference string) (string, string, error) {
	// Find the first dot
	dotIndex := -1
	for i, ch := range reference {
		if ch == '.' {
			dotIndex = i
			break
		}
	}

	if dotIndex == -1 {
		return "", "", &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("invalid operation reference %q: must be in format 'provider.operation'", reference),
		}
	}

	providerName := reference[:dotIndex]
	operationName := reference[dotIndex+1:]

	if providerName == "" || operationName == "" {
		return "", "", &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("invalid operation reference %q: provider and operation names cannot be empty", reference),
		}
	}

	return providerName, operationName, nil
}
