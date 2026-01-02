package llm

import (
	"context"
)

// ModelDiscoverer is an optional interface that providers can implement
// to support dynamic model discovery from the provider's API.
type ModelDiscoverer interface {
	// DiscoverModels retrieves available models from the provider.
	// Returns a list of ModelInfo with populated metadata.
	// If the provider doesn't support discovery (e.g., known model list),
	// this method can return a static list of known models.
	DiscoverModels(ctx context.Context) ([]ModelInfo, error)
}

// DiscoveryResult contains the result of model discovery from a provider.
type DiscoveryResult struct {
	// ProviderName identifies which provider these models belong to.
	ProviderName string

	// Models contains the discovered models with metadata.
	Models []ModelInfo

	// Error contains any error that occurred during discovery.
	// A provider may return partial results along with an error.
	Error error

	// Source indicates whether models came from API or hardcoded list.
	Source DiscoverySource
}

// DiscoverySource indicates where model information came from.
type DiscoverySource string

const (
	// DiscoverySourceAPI indicates models were discovered via API call.
	DiscoverySourceAPI DiscoverySource = "api"

	// DiscoverySourceStatic indicates models came from a hardcoded list.
	DiscoverySourceStatic DiscoverySource = "static"
)

// DiscoveryError represents an error during model discovery.
type DiscoveryError struct {
	// ProviderName identifies which provider failed.
	ProviderName string

	// Err is the underlying error.
	Err error

	// Recoverable indicates whether discovery can be retried.
	Recoverable bool
}

func (e *DiscoveryError) Error() string {
	return e.ProviderName + ": " + e.Err.Error()
}

func (e *DiscoveryError) Unwrap() error {
	return e.Err
}
