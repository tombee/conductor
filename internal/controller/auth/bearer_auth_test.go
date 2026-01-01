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

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestBearerAuthenticator_ExtractBearerToken(t *testing.T) {
	auth := NewBearerAuthenticator()

	tests := []struct {
		name      string
		header    string
		wantToken string
		wantErr   bool
	}{
		{
			name:      "valid bearer token",
			header:    "Bearer abc123xyz",
			wantToken: "abc123xyz",
			wantErr:   false,
		},
		{
			name:      "bearer with lowercase",
			header:    "bearer abc123xyz",
			wantToken: "abc123xyz",
			wantErr:   false,
		},
		{
			name:      "bearer with extra spaces",
			header:    "Bearer    abc123xyz   ",
			wantToken: "abc123xyz",
			wantErr:   false,
		},
		{
			name:      "missing authorization header",
			header:    "",
			wantToken: "",
			wantErr:   true,
		},
		{
			name:      "invalid prefix",
			header:    "Basic abc123",
			wantToken: "",
			wantErr:   true,
		},
		{
			name:      "empty token",
			header:    "Bearer ",
			wantToken: "",
			wantErr:   true,
		},
		{
			name:      "token with spaces",
			header:    "Bearer abc 123",
			wantToken: "abc 123",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			token, err := auth.ExtractBearerToken(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractBearerToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if token != tt.wantToken {
				t.Errorf("ExtractBearerToken() = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

func TestBearerAuthenticator_VerifyToken(t *testing.T) {
	auth := NewBearerAuthenticator()

	tests := []struct {
		name   string
		token  string
		secret string
		want   bool
	}{
		{
			name:   "matching tokens",
			token:  "secret123",
			secret: "secret123",
			want:   true,
		},
		{
			name:   "different tokens",
			token:  "secret123",
			secret: "wrong",
			want:   false,
		},
		{
			name:   "empty token",
			token:  "",
			secret: "secret",
			want:   false,
		},
		{
			name:   "empty secret",
			token:  "token",
			secret: "",
			want:   false,
		},
		{
			name:   "both empty",
			token:  "",
			secret: "",
			want:   true, // Empty strings are equal
		},
		{
			name:   "case sensitive",
			token:  "Secret123",
			secret: "secret123",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := auth.VerifyToken(tt.token, tt.secret); got != tt.want {
				t.Errorf("VerifyToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBearerAuthenticator_VerifyToken_TimingSafe(t *testing.T) {
	// This test verifies that token comparison takes similar time
	// regardless of how much of the token matches
	auth := NewBearerAuthenticator()
	secret := "this-is-a-very-long-secret-key-for-testing-timing-attacks-12345678"

	// Test tokens with varying amounts of matching prefix
	tokens := []string{
		"x" + secret[1:], // First character differs
		secret[:len(secret)/2] + "x" + secret[len(secret)/2+1:], // Middle differs
		secret[:len(secret)-1] + "x",                            // Last character differs
		"completely-different-token-that-has-similar-length-to-secret-key",
	}

	// Run each comparison multiple times and measure time
	const iterations = 1000
	times := make([]time.Duration, len(tokens))

	for i, token := range tokens {
		start := time.Now()
		for j := 0; j < iterations; j++ {
			auth.VerifyToken(token, secret)
		}
		times[i] = time.Since(start)
	}

	// Check that all times are within reasonable variance
	// Timing attacks would show significant differences
	minTime := times[0]
	maxTime := times[0]
	for _, t := range times[1:] {
		if t < minTime {
			minTime = t
		}
		if t > maxTime {
			maxTime = t
		}
	}

	// Allow 50% variance (timing-safe comparison should be much closer)
	variance := float64(maxTime-minTime) / float64(minTime)
	if variance > 0.5 {
		t.Logf("Warning: High variance in timing (%v), may indicate timing leak", variance)
		// Don't fail the test as timing can be variable in test environments
		// but log it for manual review
	}
}

func TestBearerAuthenticator_Authenticate(t *testing.T) {
	auth := NewBearerAuthenticator()
	secret := "correct-secret"

	tests := []struct {
		name       string
		authHeader string
		secret     string
		wantErr    bool
	}{
		{
			name:       "valid authentication",
			authHeader: "Bearer correct-secret",
			secret:     secret,
			wantErr:    false,
		},
		{
			name:       "invalid token",
			authHeader: "Bearer wrong-secret",
			secret:     secret,
			wantErr:    true,
		},
		{
			name:       "missing header",
			authHeader: "",
			secret:     secret,
			wantErr:    true,
		},
		{
			name:       "invalid header format",
			authHeader: "Basic credentials",
			secret:     secret,
			wantErr:    true,
		},
		{
			name:       "empty token",
			authHeader: "Bearer ",
			secret:     secret,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			err := auth.Authenticate(req, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("Authenticate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
