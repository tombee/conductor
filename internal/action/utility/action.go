// Package utility provides a builtin action for utility functions.
//
// The utility action provides general-purpose functions for:
// - Random number generation and selection (random_int, random_choose, etc.)
// - ID generation (uuid, nanoid, custom)
// - Math operations (clamp, round, min, max)
//
// All random operations use crypto/rand by default for cryptographic security.
package utility

import (
	"context"
)

// UtilityAction implements the action interface for utility operations.
type UtilityAction struct {
	config       *Config
	randomSource RandomSource
}

// Config holds configuration for the utility action.
type Config struct {
	// RandomSeed, if set, enables deterministic random for testing.
	// When nil, crypto/rand is used for secure randomness.
	RandomSeed *int64

	// MaxArraySize is the maximum size for input arrays (default: 10,000).
	MaxArraySize int

	// MaxIDLength is the maximum length for generated IDs (default: 256).
	MaxIDLength int

	// DefaultNanoidLength is the default length for nanoid generation (default: 21).
	DefaultNanoidLength int
}

// DefaultConfig returns sensible defaults for utility action configuration.
func DefaultConfig() *Config {
	return &Config{
		RandomSeed:          nil, // Use crypto/rand by default
		MaxArraySize:        10000,
		MaxIDLength:         256,
		DefaultNanoidLength: 21,
	}
}

// New creates a new utility action instance.
func New(config *Config) (*UtilityAction, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Apply defaults to unset fields
	if config.MaxArraySize == 0 {
		config.MaxArraySize = 10000
	}
	if config.MaxIDLength == 0 {
		config.MaxIDLength = 256
	}
	if config.DefaultNanoidLength == 0 {
		config.DefaultNanoidLength = 21
	}

	// Create random source based on configuration
	var randomSource RandomSource
	if config.RandomSeed != nil {
		randomSource = NewDeterministicRandomSource(*config.RandomSeed)
	} else {
		randomSource = NewCryptoRandomSource()
	}

	return &UtilityAction{
		config:       config,
		randomSource: randomSource,
	}, nil
}

// Name returns the action identifier.
func (c *UtilityAction) Name() string {
	return "utility"
}

// Result represents the output of a utility operation.
type Result struct {
	Response interface{}
	Metadata map[string]interface{}
}

// Execute runs a named utility operation with the given inputs.
func (c *UtilityAction) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*Result, error) {
	switch operation {
	// Random operations
	case "random_int":
		return c.randomInt(ctx, inputs)
	case "random_choose":
		return c.randomChoose(ctx, inputs)
	case "random_weighted":
		return c.randomWeighted(ctx, inputs)
	case "random_sample":
		return c.randomSample(ctx, inputs)
	case "random_shuffle":
		return c.randomShuffle(ctx, inputs)

	// ID operations
	case "id_uuid":
		return c.idUUID(ctx, inputs)
	case "id_nanoid":
		return c.idNanoid(ctx, inputs)
	case "id_custom":
		return c.idCustom(ctx, inputs)

	// Math operations
	case "math_clamp":
		return c.mathClamp(ctx, inputs)
	case "math_round":
		return c.mathRound(ctx, inputs)
	case "math_min":
		return c.mathMin(ctx, inputs)
	case "math_max":
		return c.mathMax(ctx, inputs)

	// Time operations
	case "timestamp":
		return c.timestamp(ctx, inputs)
	case "sleep":
		return c.sleep(ctx, inputs)

	default:
		return nil, &OperationError{
			Operation:  operation,
			Message:    "unknown operation",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Valid operations: random_int, random_choose, random_weighted, random_sample, random_shuffle, id_uuid, id_nanoid, id_custom, math_clamp, math_round, math_min, math_max, timestamp, sleep",
		}
	}
}

// Operations returns the list of supported operations.
func (c *UtilityAction) Operations() []string {
	return []string{
		"random_int", "random_choose", "random_weighted", "random_sample", "random_shuffle",
		"id_uuid", "id_nanoid", "id_custom",
		"math_clamp", "math_round", "math_min", "math_max",
		"timestamp", "sleep",
	}
}
