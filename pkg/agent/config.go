package agent

// Config configures agent execution limits and behavior.
type Config struct {
	// MaxIterations limits the number of ReAct loop iterations
	// Default: 25
	MaxIterations int

	// TokenLimit sets cumulative token threshold across all iterations
	// Default: 50000
	TokenLimit int

	// StopOnError determines agent behavior on tool failures
	// When true: stop immediately on first tool error
	// When false: report error to agent, allow recovery attempts (default)
	StopOnError bool

	// Model specifies the model ID to use (already resolved from tier)
	Model string
}

// DefaultConfig returns the default agent configuration.
func DefaultConfig() Config {
	return Config{
		MaxIterations: 25,
		TokenLimit:    50000,
		StopOnError:   false,
		Model:         "balanced",
	}
}

// WithDefaults fills in missing config values with defaults.
func (c Config) WithDefaults() Config {
	result := c
	if result.MaxIterations == 0 {
		result.MaxIterations = 25
	}
	if result.TokenLimit == 0 {
		result.TokenLimit = 50000
	}
	if result.Model == "" {
		result.Model = "balanced"
	}
	return result
}
