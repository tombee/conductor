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

import "testing"

func TestMatchesScope(t *testing.T) {
	tests := []struct {
		name         string
		userScopes   []string
		endpointName string
		want         bool
	}{
		// Empty scopes (admin keys)
		{
			name:         "empty scopes grants full access",
			userScopes:   []string{},
			endpointName: "any-endpoint",
			want:         true,
		},
		{
			name:         "nil scopes grants full access",
			userScopes:   nil,
			endpointName: "any-endpoint",
			want:         true,
		},

		// Exact matches
		{
			name:         "exact match allows access",
			userScopes:   []string{"review-pr"},
			endpointName: "review-pr",
			want:         true,
		},
		{
			name:         "exact match with multiple scopes",
			userScopes:   []string{"deploy", "review-pr", "triage"},
			endpointName: "review-pr",
			want:         true,
		},
		{
			name:         "no exact match denies access",
			userScopes:   []string{"review-pr"},
			endpointName: "deploy",
			want:         false,
		},

		// Wildcard suffix matches
		{
			name:         "wildcard suffix matches endpoint",
			userScopes:   []string{"review-*"},
			endpointName: "review-pr",
			want:         true,
		},
		{
			name:         "wildcard suffix matches multiple endpoints",
			userScopes:   []string{"review-*"},
			endpointName: "review-main",
			want:         true,
		},
		{
			name:         "wildcard suffix matches nested names",
			userScopes:   []string{"review-*"},
			endpointName: "review-pr-security",
			want:         true,
		},
		{
			name:         "wildcard suffix does not match different prefix",
			userScopes:   []string{"review-*"},
			endpointName: "deploy-prod",
			want:         false,
		},
		{
			name:         "wildcard suffix does not match partial prefix",
			userScopes:   []string{"review-*"},
			endpointName: "revie-pr",
			want:         false,
		},

		// Multiple scopes with mixed patterns
		{
			name:         "multiple scopes with wildcard and exact",
			userScopes:   []string{"deploy", "review-*"},
			endpointName: "review-pr",
			want:         true,
		},
		{
			name:         "multiple wildcards",
			userScopes:   []string{"review-*", "deploy-*"},
			endpointName: "deploy-staging",
			want:         true,
		},
		{
			name:         "no match with multiple scopes",
			userScopes:   []string{"review-*", "deploy-*"},
			endpointName: "triage",
			want:         false,
		},

		// Edge cases
		{
			name:         "wildcard alone matches everything",
			userScopes:   []string{"*"},
			endpointName: "any-endpoint",
			want:         true,
		},
		{
			name:         "empty string endpoint with exact match",
			userScopes:   []string{""},
			endpointName: "",
			want:         true,
		},
		{
			name:         "empty string endpoint with wildcard",
			userScopes:   []string{"*"},
			endpointName: "",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesScope(tt.userScopes, tt.endpointName)
			if got != tt.want {
				t.Errorf("MatchesScope(%v, %q) = %v, want %v",
					tt.userScopes, tt.endpointName, got, tt.want)
			}
		})
	}
}

func TestMatchesScopePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		target  string
		want    bool
	}{
		// Exact matches
		{
			name:    "exact match",
			pattern: "review-pr",
			target:  "review-pr",
			want:    true,
		},
		{
			name:    "no match",
			pattern: "review-pr",
			target:  "deploy",
			want:    false,
		},

		// Wildcard suffix
		{
			name:    "wildcard suffix matches",
			pattern: "review-*",
			target:  "review-pr",
			want:    true,
		},
		{
			name:    "wildcard suffix with empty suffix",
			pattern: "review-*",
			target:  "review-",
			want:    true,
		},
		{
			name:    "wildcard suffix no match",
			pattern: "review-*",
			target:  "deploy-pr",
			want:    false,
		},
		{
			name:    "wildcard alone matches everything",
			pattern: "*",
			target:  "anything",
			want:    true,
		},
		{
			name:    "wildcard alone matches empty string",
			pattern: "*",
			target:  "",
			want:    true,
		},

		// Case sensitivity
		{
			name:    "case sensitive exact match fails",
			pattern: "Review-PR",
			target:  "review-pr",
			want:    false,
		},
		{
			name:    "case sensitive wildcard fails",
			pattern: "Review-*",
			target:  "review-pr",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesScopePattern(tt.pattern, tt.target)
			if got != tt.want {
				t.Errorf("matchesScopePattern(%q, %q) = %v, want %v",
					tt.pattern, tt.target, got, tt.want)
			}
		})
	}
}
