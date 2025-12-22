package utility

import (
	"crypto/rand"
	"math/big"
	mrand "math/rand"
	"sync"
)

// RandomSource provides random number generation.
// It abstracts the randomness source to allow deterministic testing.
type RandomSource interface {
	// Int returns a random integer in [min, max] (inclusive).
	Int(min, max int64) (int64, error)

	// Intn returns a random integer in [0, n) (exclusive upper bound).
	Intn(n int) int
}

// CryptoRandomSource uses crypto/rand for cryptographically secure randomness.
type CryptoRandomSource struct{}

// NewCryptoRandomSource creates a new cryptographic random source.
func NewCryptoRandomSource() *CryptoRandomSource {
	return &CryptoRandomSource{}
}

// Int returns a random integer in [min, max] (inclusive).
func (c *CryptoRandomSource) Int(min, max int64) (int64, error) {
	if min > max {
		return 0, &OperationError{
			Operation:  "random",
			Message:    "min must be <= max",
			ErrorType:  ErrorTypeRange,
			Suggestion: "Swap min and max values, or ensure min <= max",
		}
	}
	if min == max {
		return min, nil
	}

	rangeSize := big.NewInt(max - min + 1)
	n, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		return 0, &OperationError{
			Operation:  "random",
			Message:    "failed to generate random number",
			ErrorType:  ErrorTypeInternal,
			Cause:      err,
			Suggestion: "Check system entropy source",
		}
	}

	return min + n.Int64(), nil
}

// Intn returns a random integer in [0, n) (exclusive upper bound).
// Returns 0 if n <= 0 or if crypto/rand fails (which should be extremely rare).
// Errors are not surfaced here due to interface constraints; the caller should
// check for consistently zero results if debugging entropy issues.
func (c *CryptoRandomSource) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	result, err := c.Int(0, int64(n-1))
	if err != nil {
		return 0
	}
	return int(result)
}

// DeterministicRandomSource uses a seeded PRNG for reproducible results in tests.
type DeterministicRandomSource struct {
	rng *mrand.Rand
	mu  sync.Mutex
}

// NewDeterministicRandomSource creates a new deterministic random source with the given seed.
func NewDeterministicRandomSource(seed int64) *DeterministicRandomSource {
	return &DeterministicRandomSource{
		rng: mrand.New(mrand.NewSource(seed)),
	}
}

// Int returns a random integer in [min, max] (inclusive).
func (d *DeterministicRandomSource) Int(min, max int64) (int64, error) {
	if min > max {
		return 0, &OperationError{
			Operation:  "random",
			Message:    "min must be <= max",
			ErrorType:  ErrorTypeRange,
			Suggestion: "Swap min and max values, or ensure min <= max",
		}
	}
	if min == max {
		return min, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	rangeSize := max - min + 1
	return min + d.rng.Int63n(rangeSize), nil
}

// Intn returns a random integer in [0, n) (exclusive upper bound).
func (d *DeterministicRandomSource) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.rng.Intn(n)
}
