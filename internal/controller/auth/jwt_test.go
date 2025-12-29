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
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateJWT_HS256(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	cfg := JWTConfig{
		Secret: secret,
		Issuer: "test-issuer",
	}

	// Generate a valid token
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "user123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		UserID: "user123",
		Scopes: []string{"read", "write"},
	}

	tokenString, err := GenerateJWT(claims, cfg)
	require.NoError(t, err)

	// Validate the token
	parsedClaims, err := ValidateJWT(tokenString, cfg)
	require.NoError(t, err)
	assert.Equal(t, "user123", parsedClaims.UserID)
	assert.Equal(t, []string{"read", "write"}, parsedClaims.Scopes)
	assert.Equal(t, "test-issuer", parsedClaims.Issuer)
}

func TestValidateJWT_EdDSA(t *testing.T) {
	// Generate Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	cfg := JWTConfig{
		PublicKey:  pub,
		PrivateKey: priv,
		Issuer:     "test-issuer",
	}

	// Generate a valid token
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "user456",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		UserID: "user456",
		Scopes: []string{"admin"},
	}

	tokenString, err := GenerateJWT(claims, cfg)
	require.NoError(t, err)

	// Validate the token
	parsedClaims, err := ValidateJWT(tokenString, cfg)
	require.NoError(t, err)
	assert.Equal(t, "user456", parsedClaims.UserID)
	assert.Equal(t, []string{"admin"}, parsedClaims.Scopes)
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	cfg := JWTConfig{
		Secret: secret,
	}

	// Generate an expired token
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
		UserID: "user123",
	}

	tokenString, err := GenerateJWT(claims, cfg)
	require.NoError(t, err)

	// Validation should fail
	_, err = ValidateJWT(tokenString, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestValidateJWT_InvalidIssuer(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	cfg := JWTConfig{
		Secret: secret,
		Issuer: "expected-issuer",
	}

	// Generate token with wrong issuer
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wrong-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		UserID: "user123",
	}

	tokenString, err := GenerateJWT(claims, cfg)
	require.NoError(t, err)

	// Validation should fail
	_, err = ValidateJWT(tokenString, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid issuer")
}

func TestValidateJWT_ClockSkew(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	cfg := JWTConfig{
		Secret:    secret,
		ClockSkew: 5 * time.Minute,
	}

	// Generate token that's slightly expired but within clock skew
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
		},
		UserID: "user123",
	}

	tokenString, err := GenerateJWT(claims, cfg)
	require.NoError(t, err)

	// Should succeed due to clock skew
	parsedClaims, err := ValidateJWT(tokenString, cfg)
	require.NoError(t, err)
	assert.Equal(t, "user123", parsedClaims.UserID)
}

func TestGenerateJWT_DefaultExpiration(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	cfg := JWTConfig{
		Secret: secret,
	}

	claims := Claims{
		UserID: "user123",
	}

	tokenString, err := GenerateJWT(claims, cfg)
	require.NoError(t, err)

	// Validate and check expiration was set
	parsedClaims, err := ValidateJWT(tokenString, cfg)
	require.NoError(t, err)
	assert.NotNil(t, parsedClaims.ExpiresAt)
	assert.True(t, parsedClaims.ExpiresAt.After(time.Now()))
}
