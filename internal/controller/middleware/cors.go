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

package middleware

import (
	"fmt"
	"net/http"
	"strings"
)

// CORSConfig holds CORS middleware configuration.
type CORSConfig struct {
	// Enabled determines if CORS middleware is active (default: false)
	Enabled bool

	// AllowedOrigins specifies which origins can make cross-origin requests
	// Use ["*"] to allow all origins (not recommended for production)
	AllowedOrigins []string

	// AllowedMethods specifies which HTTP methods are allowed
	// Default: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
	AllowedMethods []string

	// AllowedHeaders specifies which headers can be used in requests
	// Default: ["Content-Type", "Authorization"]
	AllowedHeaders []string

	// ExposedHeaders specifies which headers can be exposed to the browser
	ExposedHeaders []string

	// MaxAge specifies how long (in seconds) preflight results can be cached
	// Default: 86400 (24 hours)
	MaxAge int

	// AllowCredentials indicates whether credentials (cookies, auth) can be sent
	// Default: true
	AllowCredentials bool

	// ExcludePaths are paths that should not have CORS headers applied
	// Used to exclude admin endpoints from CORS
	ExcludePaths []string
}

// DefaultCORSConfig returns a CORS configuration with sensible defaults.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		Enabled:          false,
		AllowedOrigins:   []string{},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"X-Run-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		MaxAge:           86400,
		AllowCredentials: true,
		ExcludePaths:     []string{"/v1/admin/"},
	}
}

// CORS creates a CORS middleware with the given configuration.
// If config.Enabled is false, returns a no-op middleware.
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	// If CORS is disabled, return a no-op middleware
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Apply defaults
	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = DefaultCORSConfig().AllowedMethods
	}
	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = DefaultCORSConfig().AllowedHeaders
	}
	if config.MaxAge == 0 {
		config.MaxAge = DefaultCORSConfig().MaxAge
	}
	if len(config.ExcludePaths) == 0 {
		config.ExcludePaths = DefaultCORSConfig().ExcludePaths
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should be excluded from CORS
			for _, excludePath := range config.ExcludePaths {
				if strings.HasPrefix(r.URL.Path, excludePath) {
					// Skip CORS for excluded paths
					next.ServeHTTP(w, r)
					return
				}
			}

			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if origin != "" && isOriginAllowed(origin, config.AllowedOrigins) {
				// Set CORS headers
				w.Header().Set("Access-Control-Allow-Origin", origin)

				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				// Handle preflight OPTIONS request
				if r.Method == http.MethodOptions {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))

					if len(config.ExposedHeaders) > 0 {
						w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
					}

					if config.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
					}

					w.WriteHeader(http.StatusNoContent)
					return
				}

				// Set exposed headers for actual requests
				if len(config.ExposedHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isOriginAllowed checks if the given origin is in the allowed list.
// Supports wildcard "*" to allow all origins.
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// Support wildcard suffixes (e.g., "*.example.com")
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:] // Remove the *
			if strings.HasSuffix(origin, suffix) {
				return true
			}
		}
	}
	return false
}
