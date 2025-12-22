package connector

import (
	"context"
	"fmt"
	"sync"

	"github.com/tombee/conductor/pkg/workflow"
)

// Registry manages a collection of connectors for a workflow.
type Registry struct {
	mu         sync.RWMutex
	connectors map[string]Connector
	config     *Config
}

// NewRegistry creates a new connector registry.
func NewRegistry(config *Config) *Registry {
	if config == nil {
		config = DefaultConfig()
	}

	return &Registry{
		connectors: make(map[string]Connector),
		config:     config,
	}
}

// LoadFromDefinition loads all connectors from a workflow definition.
func (r *Registry) LoadFromDefinition(def *workflow.Definition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing connectors
	r.connectors = make(map[string]Connector)

	// Load each connector definition
	for name, connDef := range def.Connectors {
		// Ensure name is set (it should be from validation)
		connDef.Name = name

		// Create connector
		connector, err := New(&connDef, r.config)
		if err != nil {
			return fmt.Errorf("failed to create connector %q: %w", name, err)
		}

		r.connectors[name] = connector
	}

	return nil
}

// Get retrieves a connector by name.
func (r *Registry) Get(name string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connector, exists := r.connectors[name]
	if !exists {
		return nil, &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("connector %q not found", name),
			Suggestion: "Check that the connector is defined in the workflow connectors section",
		}
	}

	return connector, nil
}

// Execute runs a connector operation.
// The reference should be in format "connector_name.operation_name".
func (r *Registry) Execute(ctx context.Context, reference string, inputs map[string]interface{}) (*Result, error) {
	// Parse reference (connector_name.operation_name)
	connectorName, operationName, err := parseReference(reference)
	if err != nil {
		return nil, err
	}

	// Get connector
	connector, err := r.Get(connectorName)
	if err != nil {
		return nil, err
	}

	// Execute operation
	return connector.Execute(ctx, operationName, inputs)
}

// List returns the names of all registered connectors.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.connectors))
	for name := range r.connectors {
		names = append(names, name)
	}

	return names
}

// parseReference splits a connector reference into connector and operation names.
// Expected format: "connector_name.operation_name"
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
			Message: fmt.Sprintf("invalid connector reference %q: must be in format 'connector_name.operation_name'", reference),
		}
	}

	connectorName := reference[:dotIndex]
	operationName := reference[dotIndex+1:]

	if connectorName == "" || operationName == "" {
		return "", "", &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("invalid connector reference %q: connector and operation names cannot be empty", reference),
		}
	}

	return connectorName, operationName, nil
}
