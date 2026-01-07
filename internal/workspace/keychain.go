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

package workspace

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/zalando/go-keyring"
)

const (
	// keychainService is the service name for conductor keychain entries
	keychainService = "conductor"

	// masterKeyName is the keychain key for the workspace encryption master key
	masterKeyName = "workspace-master-key"

	// masterKeyEnvVar is the environment variable fallback for the master key
	masterKeyEnvVar = "CONDUCTOR_MASTER_KEY"
)

var (
	// ErrKeychainUnavailable is returned when the system keychain is not accessible
	ErrKeychainUnavailable = errors.New("system keychain unavailable")

	// ErrMasterKeyNotFound is returned when no master key is configured
	ErrMasterKeyNotFound = errors.New("master key not found in keychain or environment")

	// generatedKeyCache caches a generated key within a single process run
	// This prevents generating multiple different keys when keychain is unavailable
	generatedKeyCache   []byte
	generatedKeyCacheMu sync.Mutex
)

// KeychainManager handles retrieval and storage of the workspace encryption master key.
//
// Key resolution order:
//  1. System keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager)
//  2. CONDUCTOR_MASTER_KEY environment variable (fallback for headless/CI environments)
//
// The master key is a 32-byte (256-bit) AES-256 key used to encrypt integration credentials.
type KeychainManager struct {
	// keychainAvailable indicates if the system keychain is accessible
	keychainAvailable bool
}

// NewKeychainManager creates a new keychain manager.
// It tests keychain availability but doesn't fail if unavailable
// (fallback to environment variable is automatic).
func NewKeychainManager() *KeychainManager {
	manager := &KeychainManager{
		keychainAvailable: true,
	}

	// Test keychain availability
	_, err := keyring.Get(keychainService, "__conductor_availability_test__")
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		manager.keychainAvailable = false
	}

	return manager
}

// GetMasterKey retrieves the master encryption key.
//
// Resolution order:
//  1. System keychain (if available)
//  2. CONDUCTOR_MASTER_KEY environment variable
//
// Returns ErrMasterKeyNotFound if no key is configured in either location.
func (m *KeychainManager) GetMasterKey() ([]byte, error) {
	// Try keychain first (preferred)
	if m.keychainAvailable {
		keyStr, err := keyring.Get(keychainService, masterKeyName)
		if err == nil {
			// Decode base64-encoded key
			key, err := base64.StdEncoding.DecodeString(keyStr)
			if err != nil {
				return nil, fmt.Errorf("failed to decode master key from keychain: %w", err)
			}
			if len(key) != 32 {
				return nil, fmt.Errorf("invalid master key length in keychain: expected 32 bytes, got %d", len(key))
			}
			return key, nil
		}

		// If error is not "not found", keychain is likely locked/unavailable
		if !errors.Is(err, keyring.ErrNotFound) {
			// Continue to env var fallback rather than failing
			m.keychainAvailable = false
		}
	}

	// Fallback to environment variable
	envKey := os.Getenv(masterKeyEnvVar)
	if envKey != "" {
		// Decode base64-encoded key
		key, err := base64.StdEncoding.DecodeString(envKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode CONDUCTOR_MASTER_KEY: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("invalid CONDUCTOR_MASTER_KEY length: expected 32 bytes, got %d", len(key))
		}
		return key, nil
	}

	return nil, ErrMasterKeyNotFound
}

// GetOrCreateMasterKey retrieves the master key or creates a new one if not found.
//
// If no key exists:
//  1. Generate a new cryptographically secure 32-byte key
//  2. Store in system keychain (if available)
//  3. Return the key
//
// If keychain is unavailable, prints the key to stderr with instructions to set CONDUCTOR_MASTER_KEY.
// The generated key is cached in memory to prevent generating different keys within the same process.
func (m *KeychainManager) GetOrCreateMasterKey() ([]byte, error) {
	// Try to get existing key
	key, err := m.GetMasterKey()
	if err == nil {
		return key, nil
	}

	// Only create new key if error is "not found"
	if !errors.Is(err, ErrMasterKeyNotFound) {
		return nil, err
	}

	// Check if we already generated a key in this process
	generatedKeyCacheMu.Lock()
	defer generatedKeyCacheMu.Unlock()

	if generatedKeyCache != nil {
		return generatedKeyCache, nil
	}

	// Generate new key
	key, err = GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate master key: %w", err)
	}

	// Encode key as base64 for storage
	keyStr := base64.StdEncoding.EncodeToString(key)

	// Try to store in keychain
	if m.keychainAvailable {
		if err := keyring.Set(keychainService, masterKeyName, keyStr); err != nil {
			// Keychain write failed - continue with env var instructions
			m.keychainAvailable = false
		} else {
			// Successfully stored in keychain
			return key, nil
		}
	}

	// Cache the key for subsequent calls in this process
	generatedKeyCache = key

	// Keychain unavailable - print key to stderr with instructions
	fmt.Fprintf(os.Stderr, "\n"+
		"System keychain unavailable. To persist the encryption key, set the environment variable:\n"+
		"\n"+
		"export %s=%s\n"+
		"\n"+
		"WARNING: Store this value securely. If lost, encrypted integrations cannot be recovered.\n"+
		"\n",
		masterKeyEnvVar, keyStr)

	return key, nil
}

// StoreMasterKey stores a master key in the keychain.
// This is useful for restoring from backup or migrating between systems.
//
// Returns ErrKeychainUnavailable if the keychain is not accessible.
func (m *KeychainManager) StoreMasterKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(key))
	}

	if !m.keychainAvailable {
		return ErrKeychainUnavailable
	}

	// Encode key as base64
	keyStr := base64.StdEncoding.EncodeToString(key)

	// Store in keychain
	if err := keyring.Set(keychainService, masterKeyName, keyStr); err != nil {
		return fmt.Errorf("failed to store master key in keychain: %w", err)
	}

	return nil
}

// DeleteMasterKey removes the master key from the keychain.
// This is primarily used for testing and cleanup.
//
// WARNING: After deletion, encrypted integrations cannot be decrypted
// unless the key is backed up elsewhere.
func (m *KeychainManager) DeleteMasterKey() error {
	if !m.keychainAvailable {
		return ErrKeychainUnavailable
	}

	if err := keyring.Delete(keychainService, masterKeyName); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			// Already deleted - not an error
			return nil
		}
		return fmt.Errorf("failed to delete master key from keychain: %w", err)
	}

	return nil
}

// IsKeychainAvailable returns true if the system keychain is accessible.
func (m *KeychainManager) IsKeychainAvailable() bool {
	return m.keychainAvailable
}
