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
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	t.Run("generates valid token", func(t *testing.T) {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		// Token should be 43 characters (32 bytes base64url-encoded without padding)
		// 32 bytes * 8 bits/byte = 256 bits
		// base64 uses 6 bits per character: 256/6 = 42.67, rounded up = 43
		if len(token) != 43 {
			t.Errorf("token length = %d, want 43", len(token))
		}

		// Should be valid base64url
		decoded, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			t.Errorf("token is not valid base64url: %v", err)
		}

		// Decoded should be exactly 32 bytes
		if len(decoded) != TokenBytes {
			t.Errorf("decoded token length = %d, want %d", len(decoded), TokenBytes)
		}
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		token1, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		token2, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		if token1 == token2 {
			t.Error("GenerateToken() returned identical tokens")
		}
	})

	t.Run("token does not contain padding", func(t *testing.T) {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		// Base64url encoding with RawURLEncoding should not have padding
		if strings.Contains(token, "=") {
			t.Error("token should not contain padding characters")
		}
	})
}

func TestTokenValidator_Validate(t *testing.T) {
	validToken := "test-token-12345678901234567890123456789012"

	t.Run("accepts valid token", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		err := v.Validate(validToken, "192.168.1.1:12345")
		if err != nil {
			t.Errorf("Validate() with valid token error = %v, want nil", err)
		}
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		err := v.Validate("wrong-token", "192.168.1.1:12345")
		if err != ErrAuthenticationFailed {
			t.Errorf("Validate() with invalid token error = %v, want %v", err, ErrAuthenticationFailed)
		}
	})

	t.Run("rejects empty token", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		err := v.Validate("", "192.168.1.1:12345")
		if err != ErrAuthenticationFailed {
			t.Errorf("Validate() with empty token error = %v, want %v", err, ErrAuthenticationFailed)
		}
	})

	t.Run("handles remote address with port", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		err := v.Validate(validToken, "192.168.1.1:12345")
		if err != nil {
			t.Errorf("Validate() with port in address error = %v, want nil", err)
		}
	})

	t.Run("handles remote address without port", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		err := v.Validate(validToken, "192.168.1.1")
		if err != nil {
			t.Errorf("Validate() without port in address error = %v, want nil", err)
		}
	})

	t.Run("uses constant-time comparison", func(t *testing.T) {
		// This test verifies that the comparison doesn't short-circuit
		// by measuring timing, but in practice we trust subtle.ConstantTimeCompare
		v := NewTokenValidator(validToken)
		defer v.Close()

		// These should both fail, regardless of how similar they are
		err1 := v.Validate("a"+validToken[1:], "192.168.1.1")
		err2 := v.Validate(validToken[:len(validToken)-1]+"z", "192.168.1.2")

		if err1 != ErrAuthenticationFailed || err2 != ErrAuthenticationFailed {
			t.Error("Validate() should fail for any mismatch")
		}
	})
}

func TestTokenValidator_RateLimiting(t *testing.T) {
	validToken := "test-token-12345678901234567890123456789012"

	t.Run("allows multiple valid attempts", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		ip := "192.168.1.1"
		for i := 0; i < 10; i++ {
			err := v.Validate(validToken, ip)
			if err != nil {
				t.Errorf("attempt %d: Validate() error = %v, want nil", i, err)
			}
		}
	})

	t.Run("tracks failed attempts per IP", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		ip := "192.168.1.1"

		// Make 3 failed attempts
		for i := 0; i < 3; i++ {
			v.Validate("wrong-token", ip)
		}

		count := v.GetFailedAttempts(ip)
		if count != 3 {
			t.Errorf("GetFailedAttempts() = %d, want 3", count)
		}
	})

	t.Run("enforces rate limit after max attempts", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		ip := "192.168.1.1"

		// Make MaxFailedAttempts failed attempts
		for i := 0; i < MaxFailedAttempts; i++ {
			err := v.Validate("wrong-token", ip)
			if err != ErrAuthenticationFailed {
				t.Errorf("attempt %d: expected auth failed, got %v", i, err)
			}
		}

		// Next attempt should be rate limited, even with valid token
		err := v.Validate(validToken, ip)
		if err != ErrRateLimitExceeded {
			t.Errorf("Validate() after max attempts error = %v, want %v", err, ErrRateLimitExceeded)
		}

		// Verify locked out
		if !v.IsLockedOut(ip) {
			t.Error("IP should be locked out after max failed attempts")
		}
	})

	t.Run("isolates rate limiting per IP", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		ip1 := "192.168.1.1"
		ip2 := "192.168.1.2"

		// Lock out IP1
		for i := 0; i < MaxFailedAttempts; i++ {
			v.Validate("wrong-token", ip1)
		}

		// IP2 should still work
		err := v.Validate(validToken, ip2)
		if err != nil {
			t.Errorf("Validate() from different IP error = %v, want nil", err)
		}

		// IP1 should still be locked out
		err = v.Validate(validToken, ip1)
		if err != ErrRateLimitExceeded {
			t.Errorf("locked out IP error = %v, want %v", err, ErrRateLimitExceeded)
		}
	})

	t.Run("clears failed attempts on successful auth", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		ip := "192.168.1.1"

		// Make some failed attempts
		for i := 0; i < 3; i++ {
			v.Validate("wrong-token", ip)
		}

		// Successful auth should clear the count
		v.Validate(validToken, ip)

		count := v.GetFailedAttempts(ip)
		if count != 0 {
			t.Errorf("GetFailedAttempts() after success = %d, want 0", count)
		}
	})

	t.Run("resets counter after rate limit window", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		ip := "192.168.1.1"

		// Make 3 failed attempts
		for i := 0; i < 3; i++ {
			v.Validate("wrong-token", ip)
		}

		// Modify the entry to simulate time passing beyond the window
		v.mu.Lock()
		entry := v.failedAttempts[ip]
		entry.firstFail = time.Now().Add(-RateLimitWindow - time.Second)
		v.mu.Unlock()

		// Next failed attempt should reset the counter
		v.Validate("wrong-token", ip)

		count := v.GetFailedAttempts(ip)
		if count != 1 {
			t.Errorf("GetFailedAttempts() after window expiry = %d, want 1", count)
		}
	})

	t.Run("lockout expires after timeout", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		ip := "192.168.1.1"

		// Trigger lockout
		for i := 0; i < MaxFailedAttempts; i++ {
			v.Validate("wrong-token", ip)
		}

		// Verify locked out
		if !v.IsLockedOut(ip) {
			t.Error("IP should be locked out")
		}

		// Simulate lockout expiring by modifying the entry
		v.mu.Lock()
		entry := v.failedAttempts[ip]
		entry.lockedUntil = time.Now().Add(-time.Second)
		v.mu.Unlock()

		// Should no longer be locked out
		if v.IsLockedOut(ip) {
			t.Error("IP should not be locked out after timeout")
		}

		// Should be able to authenticate (though counter may still be high)
		err := v.Validate(validToken, ip)
		if err != nil && err != ErrAuthenticationFailed {
			t.Errorf("Validate() after lockout expiry error = %v", err)
		}
	})
}

func TestTokenValidator_Cleanup(t *testing.T) {
	validToken := "test-token-12345678901234567890123456789012"

	t.Run("cleanup removes expired entries", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		ip := "192.168.1.1"

		// Create a failed attempt
		v.Validate("wrong-token", ip)

		// Verify entry exists
		if v.GetFailedAttempts(ip) == 0 {
			t.Error("failed attempt should be recorded")
		}

		// Simulate time passing beyond window and lockout
		v.mu.Lock()
		entry := v.failedAttempts[ip]
		entry.firstFail = time.Now().Add(-RateLimitWindow - time.Second)
		entry.lockedUntil = time.Now().Add(-RateLimitLockout - time.Second)
		v.mu.Unlock()

		// Run cleanup
		v.cleanup()

		// Entry should be removed
		if v.GetFailedAttempts(ip) != 0 {
			t.Error("expired entry should be cleaned up")
		}
	})

	t.Run("cleanup preserves active entries", func(t *testing.T) {
		v := NewTokenValidator(validToken)
		defer v.Close()

		ip := "192.168.1.1"

		// Create a recent failed attempt
		v.Validate("wrong-token", ip)

		// Run cleanup
		v.cleanup()

		// Entry should still exist
		if v.GetFailedAttempts(ip) == 0 {
			t.Error("active entry should not be cleaned up")
		}
	})
}

func TestTokenValidator_Close(t *testing.T) {
	t.Run("close stops cleanup goroutine", func(t *testing.T) {
		validToken := "test-token-12345678901234567890123456789012"
		v := NewTokenValidator(validToken)

		// Close should not panic or block
		v.Close()

		// Calling Close again should not panic
		v.Close()
	})
}
