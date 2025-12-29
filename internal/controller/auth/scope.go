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

package auth

import "strings"

// MatchesScope checks if a user's scopes allow access to an endpoint.
// Returns true if the user has access, false otherwise.
//
// Matching rules:
//   - Empty user scopes (admin keys): Full access to all endpoints
//   - Exact match: scope "review-pr" matches endpoint "review-pr"
//   - Wildcard suffix: scope "review-*" matches endpoints "review-pr", "review-main", etc.
func MatchesScope(userScopes []string, endpointName string) bool {
	// Empty scopes means full access (admin key)
	if len(userScopes) == 0 {
		return true
	}

	// Check each user scope for a match
	for _, scope := range userScopes {
		if matchesScopePattern(scope, endpointName) {
			return true
		}
	}

	return false
}

// matchesScopePattern checks if a single scope pattern matches an endpoint name.
func matchesScopePattern(pattern, name string) bool {
	// Exact match
	if pattern == name {
		return true
	}

	// Wildcard suffix match (e.g., "review-*" matches "review-pr")
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(name, prefix)
	}

	return false
}
