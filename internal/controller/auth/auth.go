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

// Package auth provides authentication middleware for the daemon API.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tombee/conductor/pkg/security"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	// userContextKey is the context key for user information.
	userContextKey contextKey = "user"
)

// User represents an authenticated user.
type User struct {
	ID     string
	Name   string
	Scopes []string
}

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

// ContextWithUser returns a new context with the given user.
// This is primarily for testing purposes.
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// Config contains authentication configuration.
type Config struct {
	// Enabled controls whether authentication is required.
	Enabled bool

	// APIKeys is the list of valid API keys.
	APIKeys []APIKey

	// AllowUnixSocket allows unauthenticated access via Unix socket.
	AllowUnixSocket bool

	// JWT contains JWT authentication configuration.
	JWT *JWTConfig

	// RateLimit contains rate limiting configuration.
	RateLimit RateLimitConfig

	// OverrideManager manages security overrides.
	OverrideManager *security.OverrideManager

	// Logger for audit logging.
	Logger *slog.Logger
}

// APIKey represents an API key with metadata.
type APIKey struct {
	// Key is the actual API key value.
	Key string `json:"key"`

	// Name is a human-readable name for the key.
	Name string `json:"name"`

	// CreatedAt is when the key was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the key expires (zero means never).
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Scopes limits what the key can access (empty means all).
	Scopes []string `json:"scopes,omitempty"`
}

// Middleware provides authentication middleware.
type Middleware struct {
	mu              sync.RWMutex
	config          Config
	keyLookup       map[string]*APIKey
	rateLimiter     *RateLimiter
	overrideManager *security.OverrideManager
	logger          *slog.Logger
}

// NewMiddleware creates a new auth middleware.
func NewMiddleware(cfg Config) *Middleware {
	m := &Middleware{
		config:          cfg,
		keyLookup:       make(map[string]*APIKey),
		rateLimiter:     NewRateLimiter(cfg.RateLimit),
		overrideManager: cfg.OverrideManager,
		logger:          cfg.Logger,
	}

	// Build key lookup
	for i := range cfg.APIKeys {
		key := &cfg.APIKeys[i]
		m.keyLookup[key.Key] = key
	}

	return m
}

// Wrap wraps an http.Handler with authentication.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check for active security overrides that bypass enforcement
		if m.overrideManager != nil && m.overrideManager.IsActive(security.OverrideDisableEnforcement) {
			// Log override usage for audit trail
			if m.logger != nil {
				m.logger.Warn("authentication bypassed due to active override",
					"override_type", security.OverrideDisableEnforcement,
					"path", r.URL.Path,
					"method", r.Method,
					"remote_addr", r.RemoteAddr)
			}
			next.ServeHTTP(w, r)
			return
		}

		// Check if this is a Unix socket connection (bypass auth)
		if m.config.AllowUnixSocket && isUnixSocketRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for health endpoint
		if r.URL.Path == "/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Detect and reject query parameter authentication attempts (security vulnerability)
		if r.URL.Query().Get("api_key") != "" {
			m.unauthorized(w, "API keys in query parameters are not supported. Use Authorization header or X-API-Key header.")
			return
		}

		// Extract API key or JWT from request
		token := m.extractAPIKey(r)
		if token == "" {
			m.unauthorized(w, "Authentication required")
			return
		}

		var user *User

		// Try JWT validation first (if configured)
		if m.config.JWT != nil && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			claims, err := ValidateJWT(token, *m.config.JWT)
			if err == nil {
				// Valid JWT token
				user = &User{
					ID:     claims.UserID,
					Name:   claims.Subject,
					Scopes: claims.Scopes,
				}
			}
		}

		// Fall back to API key validation if JWT validation failed or not configured
		if user == nil {
			key, valid := m.validateKey(token)
			if !valid {
				m.unauthorized(w, "Invalid credentials")
				return
			}

			// Check expiration
			if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
				m.unauthorized(w, "API key expired")
				return
			}

			user = &User{
				ID:     key.Name,
				Name:   key.Name,
				Scopes: key.Scopes,
			}
		}

		// Apply rate limiting
		if !m.rateLimiter.Allow(user.ID) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}

		// Add user info to request context
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractAPIKey extracts the API key from the request.
// Only accepts secure methods: Authorization header (Bearer) or X-API-Key header.
// Query parameter authentication was removed for security reasons.
func (m *Middleware) extractAPIKey(r *http.Request) string {
	// Try Authorization header (Bearer token)
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Try X-API-Key header
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}

	return ""
}

// validateKey validates an API key.
func (m *Middleware) validateKey(key string) (*APIKey, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apiKey, exists := m.keyLookup[key]
	if !exists {
		return nil, false
	}

	// Use constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey.Key)) != 1 {
		return nil, false
	}

	return apiKey, true
}

// unauthorized sends an unauthorized response.
func (m *Middleware) unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// AddKey adds a new API key.
func (m *Middleware) AddKey(key APIKey) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.APIKeys = append(m.config.APIKeys, key)
	m.keyLookup[key.Key] = &m.config.APIKeys[len(m.config.APIKeys)-1]
}

// RemoveKey removes an API key by value.
func (m *Middleware) RemoveKey(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.keyLookup, key)

	for i, k := range m.config.APIKeys {
		if k.Key == key {
			m.config.APIKeys = append(m.config.APIKeys[:i], m.config.APIKeys[i+1:]...)
			return true
		}
	}
	return false
}

// ListKeys returns all API keys (with keys masked).
func (m *Middleware) ListKeys() []APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]APIKey, len(m.config.APIKeys))
	for i, key := range m.config.APIKeys {
		result[i] = APIKey{
			Key:       maskKey(key.Key),
			Name:      key.Name,
			CreatedAt: key.CreatedAt,
			ExpiresAt: key.ExpiresAt,
			Scopes:    key.Scopes,
		}
	}
	return result
}

// GenerateKey generates a new random API key.
func GenerateKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "cnd_" + hex.EncodeToString(bytes), nil
}

// maskKey masks an API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// isUnixSocketRequest checks if the request came via Unix socket.
// This is determined by checking if the remote address is empty or starts with "@"
// (abstract Unix socket) or "/" (file-based Unix socket).
func isUnixSocketRequest(r *http.Request) bool {
	addr := r.RemoteAddr
	return addr == "" || strings.HasPrefix(addr, "@") || strings.HasPrefix(addr, "/")
}
