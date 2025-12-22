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

package rpc

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"sync"
	"time"
)

var (
	// ErrAuthenticationFailed is returned when token validation fails.
	ErrAuthenticationFailed = errors.New("rpc: authentication failed")

	// ErrRateLimitExceeded is returned when too many failed attempts occur.
	ErrRateLimitExceeded = errors.New("rpc: rate limit exceeded")
)

const (
	// TokenBytes is the number of random bytes in an auth token.
	TokenBytes = 32

	// MaxFailedAttempts is the maximum number of failed auth attempts per client IP.
	MaxFailedAttempts = 5

	// RateLimitWindow is the time window for tracking failed attempts.
	RateLimitWindow = 1 * time.Minute

	// RateLimitLockout is the duration a client is locked out after exceeding the rate limit.
	RateLimitLockout = 60 * time.Second
)

// GenerateToken generates a cryptographically secure random token.
// The token is 32 bytes of random data, base64url-encoded (44 characters).
func GenerateToken() (string, error) {
	bytes := make([]byte, TokenBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Use base64url encoding (URL-safe, no padding)
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// TokenValidator handles token validation with rate limiting.
type TokenValidator struct {
	token string

	mu             sync.RWMutex
	failedAttempts map[string]*rateLimitEntry
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
	closed         bool
}

// rateLimitEntry tracks failed authentication attempts per IP.
type rateLimitEntry struct {
	count      int
	firstFail  time.Time
	lockedUntil time.Time
}

// NewTokenValidator creates a new token validator with the given token.
func NewTokenValidator(token string) *TokenValidator {
	v := &TokenValidator{
		token:          token,
		failedAttempts: make(map[string]*rateLimitEntry),
		stopCleanup:    make(chan struct{}),
	}

	// Start cleanup goroutine to remove stale entries
	v.cleanupTicker = time.NewTicker(1 * time.Minute)
	go v.cleanupLoop()

	return v
}

// Validate checks if the provided token matches the expected token.
// It uses constant-time comparison to prevent timing attacks.
// It enforces rate limiting on failed attempts per client IP.
func (v *TokenValidator) Validate(token, remoteAddr string) error {
	// Extract IP from remoteAddr (may be in format "ip:port")
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// If splitting fails, use the whole string as IP
		ip = remoteAddr
	}

	// Check if IP is currently locked out
	v.mu.Lock()
	entry, exists := v.failedAttempts[ip]
	if exists && time.Now().Before(entry.lockedUntil) {
		v.mu.Unlock()
		return ErrRateLimitExceeded
	}
	v.mu.Unlock()

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(token), []byte(v.token)) != 1 {
		v.recordFailedAttempt(ip)
		return ErrAuthenticationFailed
	}

	// Success - clear any failed attempts for this IP
	v.mu.Lock()
	delete(v.failedAttempts, ip)
	v.mu.Unlock()

	return nil
}

// recordFailedAttempt tracks a failed authentication attempt and enforces rate limiting.
func (v *TokenValidator) recordFailedAttempt(ip string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	entry, exists := v.failedAttempts[ip]

	if !exists {
		// First failed attempt
		v.failedAttempts[ip] = &rateLimitEntry{
			count:     1,
			firstFail: now,
		}
		return
	}

	// Check if we're outside the rate limit window - reset if so
	if now.Sub(entry.firstFail) > RateLimitWindow {
		entry.count = 1
		entry.firstFail = now
		entry.lockedUntil = time.Time{}
		return
	}

	// Increment count within the same window
	entry.count++

	// Lock out if threshold exceeded
	if entry.count >= MaxFailedAttempts {
		entry.lockedUntil = now.Add(RateLimitLockout)
	}
}

// GetFailedAttempts returns the number of failed attempts for an IP (for testing).
func (v *TokenValidator) GetFailedAttempts(ip string) int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	entry, exists := v.failedAttempts[ip]
	if !exists {
		return 0
	}
	return entry.count
}

// IsLockedOut returns whether an IP is currently locked out (for testing).
func (v *TokenValidator) IsLockedOut(ip string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	entry, exists := v.failedAttempts[ip]
	if !exists {
		return false
	}
	return time.Now().Before(entry.lockedUntil)
}

// cleanupLoop periodically removes stale rate limit entries.
func (v *TokenValidator) cleanupLoop() {
	for {
		select {
		case <-v.cleanupTicker.C:
			v.cleanup()
		case <-v.stopCleanup:
			return
		}
	}
}

// cleanup removes expired rate limit entries.
func (v *TokenValidator) cleanup() {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	for ip, entry := range v.failedAttempts {
		// Remove if lockout expired and outside the rate limit window
		if now.After(entry.lockedUntil) && now.Sub(entry.firstFail) > RateLimitWindow {
			delete(v.failedAttempts, ip)
		}
	}
}

// Close stops the cleanup goroutine.
// It is safe to call Close multiple times.
func (v *TokenValidator) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.closed {
		return
	}
	v.closed = true

	if v.cleanupTicker != nil {
		v.cleanupTicker.Stop()
	}
	close(v.stopCleanup)
}
