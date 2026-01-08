package harness

import (
	"time"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/sdk"
)

// Option configures the test harness.
type Option func(*Harness) error

// WithMockProvider configures the harness to use a mock LLM provider.
// This is the default for E2E tests and ensures no external API calls.
//
// Example:
//
//	h := harness.New(t,
//		harness.WithMockProvider(
//			harness.MockResponse{Content: "test response"},
//		),
//	)
func WithMockProvider(responses ...MockResponse) Option {
	return func(h *Harness) error {
		h.mockProvider = NewMockProvider(responses...)
		return nil
	}
}

// WithProvider configures the harness to use a specific LLM provider.
// This is used for smoke tests with real providers.
//
// Example:
//
//	provider, _ := providers.NewAnthropicProvider(apiKey)
//	h := harness.New(t, harness.WithProvider(provider))
func WithProvider(provider llm.Provider) Option {
	return func(h *Harness) error {
		h.provider = provider
		return nil
	}
}

// WithTimeout sets a custom timeout for workflow execution.
// Default is 30 seconds for mock tests, 60 seconds for smoke tests.
//
// Example:
//
//	h := harness.New(t, harness.WithTimeout(5*time.Second))
func WithTimeout(d time.Duration) Option {
	return func(h *Harness) error {
		h.timeout = d
		return nil
	}
}

// WithEventCapture enables event capture for test assertions.
// Events are stored in the harness and can be retrieved via Events().
//
// Example:
//
//	h := harness.New(t, harness.WithEventCapture())
//	result := h.Run(wf, inputs)
//	events := h.Events()
func WithEventCapture() Option {
	return func(h *Harness) error {
		h.captureEvents = true
		return nil
	}
}

// WithSDKOption passes options directly to SDK creation.
// This allows configuring the SDK beyond what the harness options provide.
//
// Example:
//
//	h := harness.New(t,
//		harness.WithSDKOption(sdk.WithTokenLimit(10000)),
//	)
func WithSDKOption(opt sdk.Option) Option {
	return func(h *Harness) error {
		h.sdkOptions = append(h.sdkOptions, opt)
		return nil
	}
}
