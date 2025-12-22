package llm

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	pkgerrors "github.com/tombee/conductor/pkg/errors"
)

var (
	// ErrProviderNotFound indicates the requested provider is not registered.
	ErrProviderNotFound = errors.New("provider not found")

	// ErrProviderAlreadyRegistered indicates a provider with this name already exists.
	ErrProviderAlreadyRegistered = errors.New("provider already registered")

	// ErrNoDefaultProvider indicates no default provider has been set.
	ErrNoDefaultProvider = errors.New("no default provider configured")

	// ErrInvalidProvider indicates the provider implementation is invalid.
	ErrInvalidProvider = errors.New("invalid provider")

	// ErrProviderNotActivated indicates the provider factory is registered but not activated.
	ErrProviderNotActivated = errors.New("provider not activated")

	// ErrFactoryNotFound indicates no factory is registered for the provider.
	ErrFactoryNotFound = errors.New("provider factory not found")
)

// ProviderFactory is a function that creates a new Provider instance.
// It accepts Credentials for authentication configuration.
type ProviderFactory func(creds Credentials) (Provider, error)

// Registry manages registered LLM providers.
// It supports a two-phase initialization pattern:
// 1. Factory registration (at import time via init())
// 2. Provider activation (at startup based on config)
// It is safe for concurrent use.
type Registry struct {
	mu              sync.RWMutex
	factories       map[string]ProviderFactory // Registered factories (phase 1)
	providers       map[string]Provider        // Activated providers (phase 2)
	defaultProvider string
	failoverOrder   []string
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ProviderFactory),
		providers: make(map[string]Provider),
	}
}

// RegisterFactory registers a provider factory function.
// This is called at import time (in init() functions) and does not instantiate the provider.
// Call Activate() to instantiate providers based on configuration.
// Registering the same name twice overwrites the previous factory (idempotent).
func (r *Registry) RegisterFactory(name string, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Activate instantiates a provider from its registered factory.
// This is called at startup time for providers that are configured.
// Returns an error if the factory is not registered or provider creation fails.
func (r *Registry) Activate(name string, creds Credentials) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	factory, exists := r.factories[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrFactoryNotFound, name)
	}

	// Check if already activated
	if _, exists := r.providers[name]; exists {
		// Already activated, no-op
		return nil
	}

	provider, err := factory(creds)
	if err != nil {
		return fmt.Errorf("failed to activate provider %s: %w", name, err)
	}

	r.providers[name] = provider
	return nil
}

// IsActive returns true if the provider has been activated.
func (r *Registry) IsActive(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.providers[name]
	return exists
}

// ListFactories returns the names of all registered provider factories, sorted alphabetically.
func (r *Registry) ListFactories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListActive returns the names of all activated providers, sorted alphabetically.
func (r *Registry) ListActive() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasFactory returns true if a factory is registered for the given provider name.
func (r *Registry) HasFactory(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[name]
	return exists
}

// Register adds a provider to the registry.
// Returns an error if a provider with this name already exists or if the provider is invalid.
func (r *Registry) Register(p Provider) error {
	if p == nil {
		return ErrInvalidProvider
	}

	name := p.Name()
	if name == "" {
		return fmt.Errorf("%w: provider name cannot be empty", ErrInvalidProvider)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("%w: %s", ErrProviderAlreadyRegistered, name)
	}

	r.providers[name] = p
	return nil
}

// Get retrieves a provider by name.
// Returns NotFoundError if the provider doesn't exist.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.providers[name]
	if !exists {
		return nil, &pkgerrors.NotFoundError{
			Resource: "provider",
			ID:       name,
		}
	}

	return p, nil
}

// GetDefault returns the default provider.
// Returns ErrNoDefaultProvider if no default is set.
func (r *Registry) GetDefault() (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultProvider == "" {
		return nil, ErrNoDefaultProvider
	}

	p, exists := r.providers[r.defaultProvider]
	if !exists {
		return nil, &pkgerrors.NotFoundError{
			Resource: "provider",
			ID:       r.defaultProvider,
		}
	}

	return p, nil
}

// SetDefault sets the default provider by name.
// Returns an error if the provider is not registered.
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return &pkgerrors.NotFoundError{
			Resource: "provider",
			ID:       name,
		}
	}

	r.defaultProvider = name
	return nil
}

// List returns the names of all registered providers, sorted alphabetically.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SetFailoverOrder configures the order in which providers should be tried on failure.
// All provider names must be registered.
func (r *Registry) SetFailoverOrder(names []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate all providers exist
	for _, name := range names {
		if _, exists := r.providers[name]; !exists {
			return &pkgerrors.NotFoundError{
				Resource: "provider",
				ID:       name,
			}
		}
	}

	r.failoverOrder = make([]string, len(names))
	copy(r.failoverOrder, names)
	return nil
}

// GetFailoverOrder returns the configured failover provider order.
func (r *Registry) GetFailoverOrder() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order := make([]string, len(r.failoverOrder))
	copy(order, r.failoverOrder)
	return order
}

// CreateWithRetry creates a retry-wrapped provider.
// Returns an error if the provider is not found.
func (r *Registry) CreateWithRetry(name string, config RetryConfig) (Provider, error) {
	provider, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	return NewRetryableProvider(provider, config), nil
}

// CreateFailover creates a failover provider using the registry's failover order.
// If providerNames is empty, uses the registry's configured failover order.
// All providers must be registered in the registry.
func (r *Registry) CreateFailover(config FailoverConfig, providerNames ...string) (*FailoverProvider, error) {
	names := providerNames
	if len(names) == 0 {
		names = r.GetFailoverOrder()
	}

	if len(names) == 0 {
		return nil, &pkgerrors.ValidationError{
			Field:      "failover_providers",
			Message:    "no failover providers specified and no failover order configured",
			Suggestion: "Configure failover order with SetFailoverOrder() or pass provider names",
		}
	}

	// Validate all providers exist
	for _, name := range names {
		if _, err := r.Get(name); err != nil {
			return nil, err
		}
	}

	// Set provider order in config
	config.ProviderOrder = names

	return NewFailoverProvider(r, config)
}

// Unregister removes a provider from the registry.
// Returns an error if the provider is not found or is currently set as default.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return &pkgerrors.NotFoundError{
			Resource: "provider",
			ID:       name,
		}
	}

	if r.defaultProvider == name {
		return &pkgerrors.ValidationError{
			Field:      "provider",
			Message:    fmt.Sprintf("cannot unregister default provider %s", name),
			Suggestion: "Set a different default provider before unregistering this one",
		}
	}

	delete(r.providers, name)

	// Remove from failover order if present
	newOrder := make([]string, 0, len(r.failoverOrder))
	for _, p := range r.failoverOrder {
		if p != name {
			newOrder = append(newOrder, p)
		}
	}
	r.failoverOrder = newOrder

	return nil
}

// globalRegistry is the default global registry instance.
var globalRegistry = NewRegistry()

// Register adds a provider to the global registry.
func Register(p Provider) error {
	return globalRegistry.Register(p)
}

// Get retrieves a provider from the global registry by name.
func Get(name string) (Provider, error) {
	return globalRegistry.Get(name)
}

// GetDefault returns the default provider from the global registry.
func GetDefault() (Provider, error) {
	return globalRegistry.GetDefault()
}

// SetDefault sets the default provider in the global registry.
func SetDefault(name string) error {
	return globalRegistry.SetDefault(name)
}

// List returns all registered provider names from the global registry.
func List() []string {
	return globalRegistry.List()
}

// SetFailoverOrder configures failover order in the global registry.
func SetFailoverOrder(names []string) error {
	return globalRegistry.SetFailoverOrder(names)
}

// GetFailoverOrder returns the failover order from the global registry.
func GetFailoverOrder() []string {
	return globalRegistry.GetFailoverOrder()
}

// Unregister removes a provider from the global registry.
func Unregister(name string) error {
	return globalRegistry.Unregister(name)
}

// RegisterFactory registers a provider factory in the global registry.
// This is typically called from init() functions in provider packages.
func RegisterFactory(name string, factory ProviderFactory) {
	globalRegistry.RegisterFactory(name, factory)
}

// Activate instantiates a provider from its factory in the global registry.
func Activate(name string, creds Credentials) error {
	return globalRegistry.Activate(name, creds)
}

// IsActive returns true if the provider is activated in the global registry.
func IsActive(name string) bool {
	return globalRegistry.IsActive(name)
}

// ListFactories returns all registered factory names from the global registry.
func ListFactories() []string {
	return globalRegistry.ListFactories()
}

// ListActive returns all activated provider names from the global registry.
func ListActive() []string {
	return globalRegistry.ListActive()
}

// HasFactory returns true if a factory is registered for the provider in the global registry.
func HasFactory(name string) bool {
	return globalRegistry.HasFactory(name)
}
