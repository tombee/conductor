package truncate

import (
	"strings"
	"sync"
)

var (
	// registry stores language parsers by normalized language name
	registry = make(map[string]Language)
	// registryMu protects concurrent access to the registry
	registryMu sync.RWMutex
)

// RegisterLanguage registers a language parser for the given language identifier.
// The language name is case-insensitive and will be normalized to lowercase.
// If a parser already exists for this language, it will be replaced.
// Thread-safe for concurrent registration.
func RegisterLanguage(language string, parser Language) {
	registryMu.Lock()
	defer registryMu.Unlock()

	normalized := normalizeLanguage(language)
	registry[normalized] = parser
}

// GetLanguage retrieves the language parser for the given language identifier.
// Returns nil if no parser is registered for this language.
// The language name is case-insensitive.
// Thread-safe for concurrent access.
func GetLanguage(language string) Language {
	registryMu.RLock()
	defer registryMu.RUnlock()

	normalized := normalizeLanguage(language)
	return registry[normalized]
}

// normalizeLanguage converts a language identifier to its canonical form.
// Converts to lowercase and trims whitespace for consistent lookups.
func normalizeLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}
