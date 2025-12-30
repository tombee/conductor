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

	"github.com/tombee/conductor/pkg/profile"
	"github.com/zalando/go-keyring"
)

// KeychainProvider implements secret resolution from system keychain.
// This provider supports keychain: references for secure credential storage.
//
// Reference format:
//   - keychain:github-token -> resolves "github-token" from system keychain
//
// Supported platforms:
//   - macOS: Keychain Access
//   - Linux: Secret Service API (GNOME Keyring, KWallet)
//   - Windows: Credential Manager
type KeychainProvider struct {
	// service is the keychain service name used for all entries
	service string

	// available indicates if the keychain is accessible
	available bool
}

// NewKeychainProvider creates a new keychain secret provider.
// The service parameter specifies the keychain service name (typically "conductor").
func NewKeychainProvider(service string) *KeychainProvider {
	provider := &KeychainProvider{
		service:   service,
		available: true,
	}

	// Test keychain availability
	_, err := keyring.Get(service, "__conductor_availability_test__")
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		provider.available = false
	}

	return provider
}

// Scheme returns the provider's URI scheme identifier.
func (k *KeychainProvider) Scheme() string {
	return "keychain"
}

// Resolve retrieves a secret value from the system keychain.
//
// The reference should be the keychain key name.
// Example: "github-token" for keychain:github-token
func (k *KeychainProvider) Resolve(ctx context.Context, reference string) (string, error) {
	if !k.available {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryAccessDenied,
			"keychain:"+reference,
			"keychain",
			"system keychain unavailable or locked",
			nil,
		)
	}

	value, err := keyring.Get(k.service, reference)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", profile.NewSecretResolutionError(
				profile.ErrorCategoryNotFound,
				"keychain:"+reference,
				"keychain",
				"keychain entry not found",
				nil,
			)
		}

		// Check if error indicates locked/inaccessible keychain
		if isKeychainUnavailableError(err) {
			return "", profile.NewSecretResolutionError(
				profile.ErrorCategoryAccessDenied,
				"keychain:"+reference,
				"keychain",
				"keychain is locked or inaccessible",
				err,
			)
		}

		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryInvalidSyntax,
			"keychain:"+reference,
			"keychain",
			"keychain access error",
			err,
		)
	}

	return value, nil
}
