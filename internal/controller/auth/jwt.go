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
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig contains JWT authentication configuration.
type JWTConfig struct {
	// Secret is the signing key for symmetric algorithms (HS256).
	// Either Secret or PublicKey must be set.
	Secret []byte

	// PublicKey is the public key for asymmetric algorithms (RS256, ES256).
	PublicKey ed25519.PublicKey

	// PrivateKey is used for signing tokens (optional, only needed for token generation).
	PrivateKey ed25519.PrivateKey

	// Issuer is the expected issuer claim.
	Issuer string

	// Audience is the expected audience claim.
	Audience string

	// ClockSkew allows for clock skew when validating exp/nbf claims.
	ClockSkew time.Duration
}

// Claims represents the JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	// UserID identifies the authenticated user.
	UserID string `json:"user_id,omitempty"`
	// Scopes defines what the token can access.
	Scopes []string `json:"scopes,omitempty"`
}

// ValidateJWT validates a JWT token and returns the claims.
func ValidateJWT(tokenString string, cfg JWTConfig) (*Claims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token is empty")
	}

	// Configure parser with clock skew tolerance
	parser := jwt.NewParser(
		jwt.WithLeeway(cfg.ClockSkew),
	)

	// Parse the token
	token, err := parser.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		switch token.Method.Alg() {
		case "HS256":
			if len(cfg.Secret) == 0 {
				return nil, fmt.Errorf("HS256 requires secret key")
			}
			return cfg.Secret, nil
		case "EdDSA":
			if cfg.PublicKey == nil {
				return nil, fmt.Errorf("EdDSA requires public key")
			}
			return cfg.PublicKey, nil
		default:
			return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer
	if cfg.Issuer != "" && claims.Issuer != cfg.Issuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", cfg.Issuer, claims.Issuer)
	}

	// Validate audience
	if cfg.Audience != "" {
		valid := false
		for _, aud := range claims.Audience {
			if aud == cfg.Audience {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid audience: expected %s", cfg.Audience)
		}
	}

	// Time-based validation (exp, nbf, iat) is already handled by the parser with clock skew

	return claims, nil
}

// GenerateJWT generates a new JWT token with the given claims.
func GenerateJWT(claims Claims, cfg JWTConfig) (string, error) {
	// Set default expiration if not provided
	if claims.ExpiresAt == nil {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(24 * time.Hour))
	}

	// Set issuer if configured
	if cfg.Issuer != "" && claims.Issuer == "" {
		claims.Issuer = cfg.Issuer
	}

	// Create token
	var token *jwt.Token
	if cfg.PrivateKey != nil {
		token = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	} else if len(cfg.Secret) > 0 {
		token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	} else {
		return "", fmt.Errorf("no signing key configured")
	}

	// Sign token
	var signedToken string
	var err error
	if cfg.PrivateKey != nil {
		signedToken, err = token.SignedString(cfg.PrivateKey)
	} else {
		signedToken, err = token.SignedString(cfg.Secret)
	}

	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}
