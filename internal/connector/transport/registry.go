package transport

import (
	"fmt"
	"sync"
)

// TransportConfig is the base interface for transport configuration.
// Each transport type implements this interface with its specific configuration fields.
type TransportConfig interface {
	// TransportType returns the transport type identifier ("http", "aws_sigv4", "oauth2")
	TransportType() string

	// Validate checks if the configuration is valid
	// Returns an error with clear user-facing message if invalid
	Validate() error
}

// TransportFactory creates a transport instance with the given configuration.
// Returns an error if the configuration is invalid or transport creation fails.
type TransportFactory func(config TransportConfig) (Transport, error)

// Registry manages transport registration and creation.
// Built-in transports (http, aws_sigv4, oauth2) are registered at startup.
type Registry struct {
	mu         sync.RWMutex
	transports map[string]TransportFactory
}

// NewRegistry creates a new transport registry.
func NewRegistry() *Registry {
	return &Registry{
		transports: make(map[string]TransportFactory),
	}
}

// Register adds a transport factory to the registry.
// Returns an error if a transport with the same name is already registered.
func (r *Registry) Register(name string, factory TransportFactory) error {
	if name == "" {
		return fmt.Errorf("transport name cannot be empty")
	}
	if factory == nil {
		return fmt.Errorf("transport factory cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transports[name]; exists {
		return fmt.Errorf("transport %q is already registered", name)
	}

	r.transports[name] = factory
	return nil
}

// Create instantiates a transport by name with configuration.
// Returns an error if:
// - The transport name is not registered
// - The configuration is invalid (via config.Validate())
// - Transport creation fails
func (r *Registry) Create(name string, config TransportConfig) (Transport, error) {
	if name == "" {
		return nil, fmt.Errorf("transport name cannot be empty")
	}
	if config == nil {
		return nil, fmt.Errorf("transport configuration cannot be nil")
	}

	// Validate configuration before creation
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration for transport %q: %w", name, err)
	}

	// Verify transport type matches requested name
	if config.TransportType() != name {
		return nil, fmt.Errorf("transport type mismatch: requested %q but config is for %q", name, config.TransportType())
	}

	r.mu.RLock()
	factory, exists := r.transports[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("transport %q is not registered", name)
	}

	return factory(config)
}

// List returns the names of all registered transports.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.transports))
	for name := range r.transports {
		names = append(names, name)
	}
	return names
}

// Has returns true if a transport with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.transports[name]
	return exists
}
