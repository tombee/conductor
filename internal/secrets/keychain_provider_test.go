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

package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/tombee/conductor/pkg/profile"
	"github.com/zalando/go-keyring"
)

func TestKeychainProvider_Scheme(t *testing.T) {
	provider := NewKeychainProvider("conductor-test")
	if got := provider.Scheme(); got != "keychain" {
		t.Errorf("Scheme() = %v, want keychain", got)
	}
}

func TestKeychainProvider_Resolve(t *testing.T) {
	// Use a test-specific service to avoid conflicts
	service := "conductor-test-keychain-provider"
	provider := NewKeychainProvider(service)
	ctx := context.Background()

	t.Run("not found", func(t *testing.T) {
		_, err := provider.Resolve(ctx, "nonexistent-key")
		if err == nil {
			t.Fatal("expected error for nonexistent key, got nil")
		}

		var secretErr *profile.SecretResolutionError
		if !errors.As(err, &secretErr) {
			t.Fatalf("expected SecretResolutionError, got %T: %v", err, err)
		}

		if secretErr.Category != profile.ErrorCategoryNotFound {
			t.Errorf("expected error category NotFound, got %v", secretErr.Category)
		}
	})

	t.Run("set and retrieve", func(t *testing.T) {
		// Skip test if keychain is not available
		if !provider.available {
			t.Skip("keychain not available on this system")
		}

		key := "test-secret"
		expectedValue := "test-value"

		// Clean up before and after
		_ = keyring.Delete(service, key)
		defer func() {
			_ = keyring.Delete(service, key)
		}()

		// Store value
		if err := keyring.Set(service, key, expectedValue); err != nil {
			t.Fatalf("failed to set keychain value: %v", err)
		}

		// Resolve value
		value, err := provider.Resolve(ctx, key)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}

		if value != expectedValue {
			t.Errorf("Resolve() = %q, want %q", value, expectedValue)
		}
	})

	t.Run("unavailable keychain", func(t *testing.T) {
		// Create provider with unavailable flag
		unavailableProvider := &KeychainProvider{
			service:   service,
			available: false,
		}

		_, err := unavailableProvider.Resolve(ctx, "any-key")
		if err == nil {
			t.Fatal("expected error for unavailable keychain, got nil")
		}

		var secretErr *profile.SecretResolutionError
		if !errors.As(err, &secretErr) {
			t.Fatalf("expected SecretResolutionError, got %T: %v", err, err)
		}

		if secretErr.Category != profile.ErrorCategoryAccessDenied {
			t.Errorf("expected error category AccessDenied, got %v", secretErr.Category)
		}
	})
}
