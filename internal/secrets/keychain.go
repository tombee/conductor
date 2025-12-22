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
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	// KeychainBackendPriority is the priority for keychain backend.
	KeychainBackendPriority = 50

	// keychainService is the service name used for keychain entries.
	keychainService = "conductor"
)

// KeychainBackend provides secure storage using the system keychain.
// Supported platforms:
//   - macOS: Keychain Access
//   - Linux: Secret Service API (GNOME Keyring, KWallet)
//   - Windows: Credential Manager
type KeychainBackend struct {
	available bool
}

// NewKeychainBackend creates a new keychain backend.
// It performs availability detection to check if the keyring service is accessible.
func NewKeychainBackend() *KeychainBackend {
	backend := &KeychainBackend{
		available: true,
	}

	// Test if keychain is available by attempting to get a non-existent key
	// This helps detect locked keychains or unavailable services early
	_, err := keyring.Get(keychainService, "__conductor_availability_test__")
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		// If we get an error other than NotFound, the keychain is likely unavailable
		backend.available = false
	}

	return backend
}

// Name returns the backend identifier.
func (k *KeychainBackend) Name() string {
	return "keychain"
}

// Get retrieves a secret from the system keychain.
func (k *KeychainBackend) Get(ctx context.Context, key string) (string, error) {
	if !k.available {
		return "", fmt.Errorf("%w: keychain service unavailable", ErrBackendUnavailable)
	}

	value, err := keyring.Get(keychainService, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("%w: %s", ErrSecretNotFound, key)
		}
		// Check for common error messages indicating locked or inaccessible keychain
		if isKeychainUnavailableError(err) {
			return "", fmt.Errorf("%w: %s", ErrBackendUnavailable, err.Error())
		}
		return "", fmt.Errorf("keychain error: %w", err)
	}

	return value, nil
}

// Set stores a secret in the system keychain.
func (k *KeychainBackend) Set(ctx context.Context, key string, value string) error {
	if !k.available {
		return fmt.Errorf("%w: keychain service unavailable", ErrBackendUnavailable)
	}

	if err := keyring.Set(keychainService, key, value); err != nil {
		if isKeychainUnavailableError(err) {
			return fmt.Errorf("%w: %s", ErrBackendUnavailable, err.Error())
		}
		return fmt.Errorf("keychain error: %w", err)
	}

	return nil
}

// Delete removes a secret from the system keychain.
func (k *KeychainBackend) Delete(ctx context.Context, key string) error {
	if !k.available {
		return fmt.Errorf("%w: keychain service unavailable", ErrBackendUnavailable)
	}

	if err := keyring.Delete(keychainService, key); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("%w: %s", ErrSecretNotFound, key)
		}
		if isKeychainUnavailableError(err) {
			return fmt.Errorf("%w: %s", ErrBackendUnavailable, err.Error())
		}
		return fmt.Errorf("keychain error: %w", err)
	}

	return nil
}

// List returns all secret keys stored in the keychain.
// Note: go-keyring doesn't provide a list operation, so this returns an empty list.
// Keys are tracked when set through conductor, but we can't enumerate all keychain items.
func (k *KeychainBackend) List(ctx context.Context) ([]string, error) {
	if !k.available {
		return nil, fmt.Errorf("%w: keychain service unavailable", ErrBackendUnavailable)
	}

	// The go-keyring library doesn't support listing keys.
	// This is a limitation of the underlying keychain APIs on some platforms.
	// Return an empty list rather than an error to allow the backend to function.
	return []string{}, nil
}

// Available returns true if the keychain service is accessible.
func (k *KeychainBackend) Available() bool {
	return k.available
}

// Priority returns the backend priority.
func (k *KeychainBackend) Priority() int {
	return KeychainBackendPriority
}

// isKeychainUnavailableError checks if an error indicates the keychain is locked or inaccessible.
// This includes common error messages from different platforms.
func isKeychainUnavailableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Common error indicators across platforms
	unavailableIndicators := []string{
		"locked",
		"cannot access",
		"permission denied",
		"failed to unlock",
		"user interaction required",
		"secret service",
		"dbus",
		"user canceled",
	}

	for _, indicator := range unavailableIndicators {
		if strings.Contains(errStr, indicator) {
			return true
		}
	}

	return false
}
