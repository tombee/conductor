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
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

// BearerAuthenticator provides Bearer token authentication for API endpoints.
type BearerAuthenticator struct{}

// NewBearerAuthenticator creates a new Bearer token authenticator.
func NewBearerAuthenticator() *BearerAuthenticator {
	return &BearerAuthenticator{}
}

// ExtractBearerToken extracts the Bearer token from the Authorization header.
// Returns the token value (without "Bearer " prefix) and an error if invalid.
func (a *BearerAuthenticator) ExtractBearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", fmt.Errorf("missing Authorization header")
	}

	// Check Bearer prefix (case-insensitive per RFC 6750)
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(auth, bearerPrefix) && !strings.HasPrefix(auth, "bearer ") {
		return "", fmt.Errorf("invalid Authorization header format, expected 'Bearer <token>'")
	}

	// Extract token (skip "Bearer " prefix - 7 characters)
	token := strings.TrimSpace(auth[7:])
	if token == "" {
		return "", fmt.Errorf("empty Bearer token")
	}

	return token, nil
}

// VerifyToken compares the provided token with the expected secret using constant-time comparison.
// This prevents timing attacks that could reveal information about the secret.
func (a *BearerAuthenticator) VerifyToken(token, secret string) bool {
	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
}

// Authenticate verifies the Bearer token from the request against the expected secret.
// Returns an error if authentication fails.
func (a *BearerAuthenticator) Authenticate(r *http.Request, secret string) error {
	token, err := a.ExtractBearerToken(r)
	if err != nil {
		return err
	}

	if !a.VerifyToken(token, secret) {
		return fmt.Errorf("invalid Bearer token")
	}

	return nil
}
