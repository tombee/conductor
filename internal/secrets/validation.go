// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secrets

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tombee/conductor/pkg/profile"
)

const (
	// MaxSecretReferenceDepth is the maximum depth of secret reference chains.
	// This prevents circular references and excessively deep resolution chains.
	MaxSecretReferenceDepth = 10
)

var (
	// secretReferencePattern matches valid secret references.
	// Supports: env:VAR, file:/path, scheme:reference
	secretReferencePattern = regexp.MustCompile(`^[a-z][a-z0-9]*:.+$`)

	// malformedSchemePattern detects malformed scheme:reference format.
	// Valid: "env:VAR", "file:/path"
	// Invalid: "env:", ":reference", "ENV:var" (uppercase scheme)
	malformedSchemePattern = regexp.MustCompile(`^([A-Z]|.*:$|^:)`)
)

// ValidateSecretReference validates the syntax of a secret reference.
//
// Valid formats:
//   - env:API_KEY - environment variable
//   - file:/etc/secrets/token - file-based secret
//   - vault:secret/data/prod#token - vault secret (future)
//
// Returns an error if the reference syntax is invalid.
func ValidateSecretReference(reference string) error {
	if reference == "" {
		return profile.NewSecretResolutionError(
			profile.ErrorCategoryInvalidSyntax,
			reference,
			"",
			"empty secret reference",
			nil,
		)
	}

	// Check for malformed scheme (e.g., "ENV:var" instead of "env:var")
	if strings.Contains(reference, ":") {
		parts := strings.SplitN(reference, ":", 2)
		if len(parts) == 2 {
			scheme := parts[0]
			key := parts[1]

			// Scheme must be lowercase
			if scheme != strings.ToLower(scheme) {
				return profile.NewSecretResolutionError(
					profile.ErrorCategoryInvalidSyntax,
					reference,
					scheme,
					"scheme must be lowercase",
					nil,
				)
			}

			// Key must not be empty
			if strings.TrimSpace(key) == "" {
				return profile.NewSecretResolutionError(
					profile.ErrorCategoryInvalidSyntax,
					reference,
					scheme,
					"empty key for scheme",
					nil,
				)
			}

			// Scheme must match [a-z][a-z0-9]*
			if !regexp.MustCompile(`^[a-z][a-z0-9]*$`).MatchString(scheme) {
				return profile.NewSecretResolutionError(
					profile.ErrorCategoryInvalidSyntax,
					reference,
					scheme,
					"invalid scheme format (must be lowercase alphanumeric)",
					nil,
				)
			}
		}
	}

	// Check overall format
	if !secretReferencePattern.MatchString(reference) && !isPlainValue(reference) {
		return profile.NewSecretResolutionError(
			profile.ErrorCategoryInvalidSyntax,
			reference,
			"",
			"invalid secret reference format",
			nil,
		)
	}

	return nil
}

// isPlainValue returns true if the reference is a plain value (not a secret reference).
// Plain values don't start with $ or contain :.
func isPlainValue(reference string) bool {
	return !strings.HasPrefix(reference, "$") && !strings.Contains(reference, ":")
}

// DetectCircularReferences detects circular dependencies in secret references.
//
// This function analyzes a map of secret bindings to detect circular reference chains.
// For example:
//   - A -> B -> C -> A (circular)
//   - A -> B -> C -> D (valid, depth 4)
//   - A -> A (self-reference, circular)
//
// The bindings map should contain all secret references in a profile.
// Keys are binding names, values are the secret references they resolve to.
//
// Returns a CircularReferenceError if a cycle is detected, or an error if max depth is exceeded.
func DetectCircularReferences(bindings map[string]string) error {
	// Build dependency graph
	graph := make(map[string][]string)
	for key, value := range bindings {
		refs := extractReferences(value)
		if len(refs) > 0 {
			graph[key] = refs
		}
	}

	// Check each binding for cycles using DFS
	for key := range bindings {
		visited := make(map[string]bool)
		path := []string{}
		if err := detectCycle(key, graph, visited, path); err != nil {
			return err
		}
	}

	return nil
}

// detectCycle performs depth-first search to detect cycles.
func detectCycle(key string, graph map[string][]string, visited map[string]bool, path []string) error {
	// Check for cycle
	if visited[key] {
		// Found a cycle - construct the chain
		chain := append(path, key)
		return &profile.CircularReferenceError{
			Chain: chain,
		}
	}

	// Check max depth
	if len(path) >= MaxSecretReferenceDepth {
		return profile.NewSecretResolutionError(
			profile.ErrorCategoryCircularRef,
			key,
			"",
			fmt.Sprintf("secret reference chain exceeds maximum depth of %d", MaxSecretReferenceDepth),
			nil,
		)
	}

	// Mark as visited
	visited[key] = true
	path = append(path, key)

	// Visit dependencies
	for _, ref := range graph[key] {
		if err := detectCycle(ref, graph, visited, path); err != nil {
			return err
		}
	}

	// Unmark for other paths
	visited[key] = false

	return nil
}

// extractReferences extracts all secret references from a value string.
//
// Examples:
//   - "${GITHUB_TOKEN}" -> ["GITHUB_TOKEN"]
//   - "env:API_KEY" -> ["API_KEY"]
//   - "https://api.example.com" -> [] (no references)
//   - "token:${TOKEN}" -> ["TOKEN"] (mixed format)
func extractReferences(value string) []string {
	var refs []string

	// Extract ${VAR} style references
	dollarPattern := regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)
	matches := dollarPattern.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		if len(match) > 1 {
			refs = append(refs, match[1])
		}
	}

	// Extract scheme:reference style references
	// Only if the entire value is a reference (not embedded in other text)
	if strings.Contains(value, ":") && !strings.Contains(value, "//") {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 {
			scheme := parts[0]
			key := parts[1]
			// Only consider as reference if it looks like a scheme
			if regexp.MustCompile(`^[a-z][a-z0-9]*$`).MatchString(scheme) {
				// For env: references, extract the key
				if scheme == "env" {
					refs = append(refs, key)
				}
				// For other schemes, we don't track cross-provider dependencies
				// (e.g., file:/path doesn't reference another binding)
			}
		}
	}

	return refs
}

// ValidateSecretReferences validates all secret references in a map of bindings.
// This is used at profile load time to catch syntax errors early.
func ValidateSecretReferences(bindings map[string]string) error {
	for key, value := range bindings {
		// Skip if value doesn't look like a secret reference
		if !strings.HasPrefix(value, "$") && !strings.Contains(value, ":") {
			continue
		}

		// Validate syntax
		if err := ValidateSecretReference(value); err != nil {
			var resErr *profile.SecretResolutionError
			if errors, ok := err.(*profile.SecretResolutionError); ok {
				resErr = errors
			} else {
				// Wrap unknown error type
				resErr = profile.NewSecretResolutionError(
					profile.ErrorCategoryInvalidSyntax,
					value,
					"",
					fmt.Sprintf("validation failed for binding %q", key),
					err,
				)
			}
			return resErr
		}
	}

	// Check for circular references
	return DetectCircularReferences(bindings)
}
