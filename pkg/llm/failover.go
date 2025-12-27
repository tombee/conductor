package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	pkgerrors "github.com/tombee/conductor/pkg/errors"
)

var (
	// ErrAllProvidersFailed indicates all providers in the failover chain failed.
	ErrAllProvidersFailed = errors.New("all providers failed")

	// ErrCircuitOpen indicates the circuit breaker is open for a provider.
	ErrCircuitOpen = errors.New("circuit breaker open")
)

// FailoverConfig configures provider failover behavior.
type FailoverConfig struct {
	// ProviderOrder is the ordered list of provider names to try.
	ProviderOrder []string

	// CircuitBreakerThreshold is the number of consecutive failures before opening the circuit.
	// 0 disables circuit breaker.
	CircuitBreakerThreshold int

	// CircuitBreakerTimeout is how long to keep the circuit open before trying again.
	CircuitBreakerTimeout time.Duration

	// OnFailover is called when failing over to the next provider.
	// Useful for logging and monitoring.
	OnFailover func(from, to string, err error)
}

// DefaultFailoverConfig returns sensible default failover settings.
func DefaultFailoverConfig() FailoverConfig {
	return FailoverConfig{
		ProviderOrder:           []string{},
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30 * time.Second,
		OnFailover:              nil,
	}
}

// FailoverProvider implements automatic failover between multiple providers.
type FailoverProvider struct {
	registry       *Registry
	config         FailoverConfig
	circuitBreaker *circuitBreaker
}

// NewFailoverProvider creates a provider with automatic failover.
func NewFailoverProvider(registry *Registry, config FailoverConfig) (*FailoverProvider, error) {
	if len(config.ProviderOrder) == 0 {
		return nil, &pkgerrors.ConfigError{
			Key:    "failover.provider_order",
			Reason: "failover requires at least one provider",
		}
	}

	// Validate all providers exist
	for _, name := range config.ProviderOrder {
		if _, err := registry.Get(name); err != nil {
			return nil, fmt.Errorf("validating failover provider %s: %w", name, err)
		}
	}

	fp := &FailoverProvider{
		registry: registry,
		config:   config,
	}

	if config.CircuitBreakerThreshold > 0 {
		fp.circuitBreaker = newCircuitBreaker(
			config.CircuitBreakerThreshold,
			config.CircuitBreakerTimeout,
		)
	}

	return fp, nil
}

// Name returns the name of the first (primary) provider.
func (f *FailoverProvider) Name() string {
	if len(f.config.ProviderOrder) > 0 {
		return f.config.ProviderOrder[0] + "-failover"
	}
	return "failover"
}

// Capabilities returns the capabilities of the primary provider.
func (f *FailoverProvider) Capabilities() Capabilities {
	provider, err := f.registry.Get(f.config.ProviderOrder[0])
	if err != nil {
		return Capabilities{}
	}
	return provider.Capabilities()
}

// Complete tries providers in order until one succeeds.
func (f *FailoverProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	var lastErr error
	var attemptedProviders []string

	for _, providerName := range f.config.ProviderOrder {
		// Check circuit breaker
		if f.circuitBreaker != nil && !f.circuitBreaker.allowRequest(providerName) {
			lastErr = fmt.Errorf("%w for provider %s", ErrCircuitOpen, providerName)
			attemptedProviders = append(attemptedProviders, providerName)
			continue
		}

		provider, err := f.registry.Get(providerName)
		if err != nil {
			lastErr = err
			attemptedProviders = append(attemptedProviders, providerName)
			continue
		}

		resp, err := provider.Complete(ctx, req)

		if err == nil {
			// Success - record success in circuit breaker
			if f.circuitBreaker != nil {
				f.circuitBreaker.recordSuccess(providerName)
			}
			return resp, nil
		}

		// Record failure
		if f.circuitBreaker != nil {
			f.circuitBreaker.recordFailure(providerName)
		}

		lastErr = err
		attemptedProviders = append(attemptedProviders, providerName)

		// Check if we should failover based on error type
		if !shouldFailover(err) {
			// Non-failover error (e.g., bad request) - return original error preserving type
			return nil, fmt.Errorf("provider %s: %w", providerName, err)
		}

		// Trigger failover callback
		if f.config.OnFailover != nil && len(attemptedProviders) < len(f.config.ProviderOrder) {
			nextProvider := f.config.ProviderOrder[len(attemptedProviders)]
			f.config.OnFailover(providerName, nextProvider, err)
		}
	}

	// All providers failed - wrap last error in ProviderError if it isn't one already
	var provErr *pkgerrors.ProviderError
	if !errors.As(lastErr, &provErr) {
		return nil, &pkgerrors.ProviderError{
			Provider:   "failover",
			Message:    fmt.Sprintf("all providers failed (tried: %v)", attemptedProviders),
			Suggestion: "Check provider availability and configuration",
			Cause:      lastErr,
		}
	}
	return nil, fmt.Errorf("all providers failed (tried: %v): %w", attemptedProviders, lastErr)
}

// Stream tries providers in order until one succeeds.
func (f *FailoverProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	var lastErr error
	var attemptedProviders []string

	for _, providerName := range f.config.ProviderOrder {
		// Check circuit breaker
		if f.circuitBreaker != nil && !f.circuitBreaker.allowRequest(providerName) {
			lastErr = fmt.Errorf("%w for provider %s", ErrCircuitOpen, providerName)
			attemptedProviders = append(attemptedProviders, providerName)
			continue
		}

		provider, err := f.registry.Get(providerName)
		if err != nil {
			lastErr = err
			attemptedProviders = append(attemptedProviders, providerName)
			continue
		}

		chunks, err := provider.Stream(ctx, req)

		if err == nil {
			// Success - record success in circuit breaker
			if f.circuitBreaker != nil {
				f.circuitBreaker.recordSuccess(providerName)
			}
			return chunks, nil
		}

		// Record failure
		if f.circuitBreaker != nil {
			f.circuitBreaker.recordFailure(providerName)
		}

		lastErr = err
		attemptedProviders = append(attemptedProviders, providerName)

		// Check if we should failover
		if !shouldFailover(err) {
			// Non-failover error - return original error preserving type
			return nil, fmt.Errorf("provider %s: %w", providerName, err)
		}

		// Trigger failover callback
		if f.config.OnFailover != nil && len(attemptedProviders) < len(f.config.ProviderOrder) {
			nextProvider := f.config.ProviderOrder[len(attemptedProviders)]
			f.config.OnFailover(providerName, nextProvider, err)
		}
	}

	// All providers failed - wrap last error in ProviderError if it isn't one already
	var provErr *pkgerrors.ProviderError
	if !errors.As(lastErr, &provErr) {
		return nil, &pkgerrors.ProviderError{
			Provider:   "failover",
			Message:    fmt.Sprintf("all providers failed (tried: %v)", attemptedProviders),
			Suggestion: "Check provider availability and configuration",
			Cause:      lastErr,
		}
	}
	return nil, fmt.Errorf("all providers failed (tried: %v): %w", attemptedProviders, lastErr)
}

// GetCircuitBreakerStatus returns the current circuit breaker state for all providers.
func (f *FailoverProvider) GetCircuitBreakerStatus() map[string]CircuitBreakerStatus {
	if f.circuitBreaker == nil {
		return nil
	}
	return f.circuitBreaker.getStatus()
}

// shouldFailover determines if an error should trigger failover to the next provider.
// Failover occurs for:
// - HTTP 5xx errors (server errors)
// - HTTP 429 (rate limiting)
// - Timeout errors
// - Network errors
func shouldFailover(err error) bool {
	if err == nil {
		return false
	}

	// Check for ProviderError with HTTP status codes
	var provErr *pkgerrors.ProviderError
	if errors.As(err, &provErr) {
		// Failover on server errors, timeouts, and rate limiting
		return provErr.StatusCode >= 500 ||
			provErr.StatusCode == http.StatusTooManyRequests ||
			provErr.StatusCode == http.StatusRequestTimeout
	}

	// Check for timeout errors
	var timeoutErr *pkgerrors.TimeoutError
	if errors.As(err, &timeoutErr) {
		return true
	}

	// Check for context timeout (should failover)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for circuit breaker open
	if errors.Is(err, ErrCircuitOpen) {
		return true
	}

	// Check for temporary errors
	type temporary interface {
		Temporary() bool
	}
	if temp, ok := err.(temporary); ok {
		return temp.Temporary()
	}

	// Auth errors (HTTP 401/403 in ProviderError) should not trigger failover
	if provErr != nil {
		if provErr.StatusCode == http.StatusUnauthorized || provErr.StatusCode == http.StatusForbidden {
			return false
		}
	}

	// Default to not failing over for unknown errors
	return false
}

// circuitBreaker tracks provider health and prevents requests to unhealthy providers.
type circuitBreaker struct {
	mu                 sync.RWMutex
	states             map[string]*circuitState
	failureThreshold   int
	recoveryTimeout    time.Duration
}

type circuitState struct {
	consecutiveFailures int
	lastFailureTime     time.Time
	open                bool
}

// CircuitBreakerStatus represents the current state of a circuit breaker.
type CircuitBreakerStatus struct {
	Open                bool
	ConsecutiveFailures int
	LastFailureTime     time.Time
}

func newCircuitBreaker(threshold int, timeout time.Duration) *circuitBreaker {
	return &circuitBreaker{
		states:           make(map[string]*circuitState),
		failureThreshold: threshold,
		recoveryTimeout:  timeout,
	}
}

func (cb *circuitBreaker) allowRequest(providerName string) bool {
	cb.mu.RLock()
	state, exists := cb.states[providerName]
	cb.mu.RUnlock()

	if !exists {
		return true
	}

	// Check if circuit is open
	if state.open {
		// Check if recovery timeout has elapsed
		if time.Since(state.lastFailureTime) > cb.recoveryTimeout {
			// Try to close circuit (half-open state)
			cb.mu.Lock()
			state.open = false
			state.consecutiveFailures = 0
			cb.mu.Unlock()
			return true
		}
		return false
	}

	return true
}

func (cb *circuitBreaker) recordSuccess(providerName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state, exists := cb.states[providerName]
	if !exists {
		cb.states[providerName] = &circuitState{}
		return
	}

	// Reset failure count on success
	state.consecutiveFailures = 0
	state.open = false
}

func (cb *circuitBreaker) recordFailure(providerName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state, exists := cb.states[providerName]
	if !exists {
		state = &circuitState{}
		cb.states[providerName] = state
	}

	state.consecutiveFailures++
	state.lastFailureTime = time.Now()

	// Open circuit if threshold exceeded
	if state.consecutiveFailures >= cb.failureThreshold {
		state.open = true
	}
}

func (cb *circuitBreaker) getStatus() map[string]CircuitBreakerStatus {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	status := make(map[string]CircuitBreakerStatus)
	for name, state := range cb.states {
		status[name] = CircuitBreakerStatus{
			Open:                state.open,
			ConsecutiveFailures: state.consecutiveFailures,
			LastFailureTime:     state.lastFailureTime,
		}
	}
	return status
}
