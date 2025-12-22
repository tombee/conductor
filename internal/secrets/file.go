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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	// FileBackendPriority is the priority for encrypted file backend.
	FileBackendPriority = 25

	// Argon2 parameters (from spec: time=3, memory=64MB, parallelism=4)
	argon2Time        = 3
	argon2Memory      = 64 * 1024 // 64MB in KB
	argon2Parallelism = 4
	argon2KeyLength   = 32 // 256 bits for AES-256

	// AES-GCM nonce size
	gcmNonceSize = 12 // 96 bits (standard for GCM)
)

// FileBackend provides encrypted storage using AES-256-GCM.
// Secrets are stored in a JSON file encrypted with a master key derived from:
//  1. CONDUCTOR_MASTER_KEY environment variable
//  2. ~/.config/conductor/master.key file
//  3. Interactive prompt (CLI only, not controller)
type FileBackend struct {
	path      string
	masterKey []byte
	mu        sync.RWMutex
	available bool
}

// encryptedData represents the structure of the encrypted secrets file.
type encryptedData struct {
	Salt    []byte            `json:"salt"`  // Salt for Argon2 key derivation
	Nonce   []byte            `json:"nonce"` // GCM nonce
	Data    []byte            `json:"data"`  // Encrypted secrets data
	Secrets map[string]string `json:"-"`     // In-memory plaintext (not serialized)
}

// NewFileBackend creates a new encrypted file backend.
// The path parameter specifies where to store the encrypted file.
// If empty, it defaults to ~/.config/conductor/secrets.enc
func NewFileBackend(path string, masterKey string) (*FileBackend, error) {
	if path == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get config directory: %w", err)
		}
		path = filepath.Join(configDir, "conductor", "secrets.enc")
	}

	// Resolve master key
	key, err := resolveMasterKey(masterKey)
	if err != nil {
		return &FileBackend{
			path:      path,
			available: false,
		}, nil // Return unavailable backend rather than error
	}

	backend := &FileBackend{
		path:      path,
		masterKey: key,
		available: true,
	}

	// Ensure parent directory exists with secure permissions
	if err := backend.ensureParentDir(); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	return backend, nil
}

// Name returns the backend identifier.
func (f *FileBackend) Name() string {
	return "file"
}

// Get retrieves a secret from the encrypted file.
func (f *FileBackend) Get(ctx context.Context, key string) (string, error) {
	if !f.available {
		return "", fmt.Errorf("%w: master key not available", ErrBackendUnavailable)
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Load and decrypt secrets
	secrets, err := f.load()
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrSecretNotFound, key)
		}
		return "", fmt.Errorf("failed to load secrets: %w", err)
	}

	value, ok := secrets[key]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, key)
	}

	return value, nil
}

// Set stores a secret in the encrypted file.
func (f *FileBackend) Set(ctx context.Context, key string, value string) error {
	if !f.available {
		return fmt.Errorf("%w: master key not available", ErrBackendUnavailable)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Load existing secrets or create new map
	secrets, err := f.load()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load secrets: %w", err)
	}
	if secrets == nil {
		secrets = make(map[string]string)
	}

	// Update secret
	secrets[key] = value

	// Save encrypted secrets
	if err := f.save(secrets); err != nil {
		return fmt.Errorf("failed to save secrets: %w", err)
	}

	return nil
}

// Delete removes a secret from the encrypted file.
func (f *FileBackend) Delete(ctx context.Context, key string) error {
	if !f.available {
		return fmt.Errorf("%w: master key not available", ErrBackendUnavailable)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Load existing secrets
	secrets, err := f.load()
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrSecretNotFound, key)
		}
		return fmt.Errorf("failed to load secrets: %w", err)
	}

	// Check if key exists
	if _, ok := secrets[key]; !ok {
		return fmt.Errorf("%w: %s", ErrSecretNotFound, key)
	}

	// Delete and save
	delete(secrets, key)
	if err := f.save(secrets); err != nil {
		return fmt.Errorf("failed to save secrets: %w", err)
	}

	return nil
}

// List returns all secret keys from the encrypted file.
func (f *FileBackend) List(ctx context.Context) ([]string, error) {
	if !f.available {
		return nil, fmt.Errorf("%w: master key not available", ErrBackendUnavailable)
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	secrets, err := f.load()
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to load secrets: %w", err)
	}

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}

	return keys, nil
}

// Available returns true if the master key is available.
func (f *FileBackend) Available() bool {
	return f.available
}

// Priority returns the backend priority.
func (f *FileBackend) Priority() int {
	return FileBackendPriority
}

// load reads and decrypts the secrets file.
func (f *FileBackend) load() (map[string]string, error) {
	// Read encrypted file
	encData, err := os.ReadFile(f.path)
	if err != nil {
		return nil, err
	}

	// Parse encrypted data structure
	var data encryptedData
	if err := json.Unmarshal(encData, &data); err != nil {
		return nil, fmt.Errorf("invalid encrypted data format: %w", err)
	}

	// Derive encryption key from master key and salt
	key := argon2.IDKey(f.masterKey, data.Salt, argon2Time, argon2Memory, argon2Parallelism, argon2KeyLength)
	defer zeroBytes(key)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt data
	plaintext, err := gcm.Open(nil, data.Nonce, data.Data, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong master key or corrupted data): %w", err)
	}
	defer zeroBytes(plaintext)

	// Parse decrypted JSON
	var secrets map[string]string
	if err := json.Unmarshal(plaintext, &secrets); err != nil {
		return nil, fmt.Errorf("invalid decrypted data format: %w", err)
	}

	return secrets, nil
}

// save encrypts and writes the secrets file.
func (f *FileBackend) save(secrets map[string]string) error {
	// Serialize secrets to JSON
	plaintext, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}
	defer zeroBytes(plaintext)

	// Generate random salt for key derivation
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive encryption key from master key and salt
	key := argon2.IDKey(f.masterKey, salt, argon2Time, argon2Memory, argon2Parallelism, argon2KeyLength)
	defer zeroBytes(key)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcmNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Create encrypted data structure
	data := encryptedData{
		Salt:  salt,
		Nonce: nonce,
		Data:  ciphertext,
	}

	// Serialize to JSON
	encData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal encrypted data: %w", err)
	}

	// Write to temp file first (atomic write)
	tmpPath := f.path + ".tmp"
	if err := os.WriteFile(tmpPath, encData, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, f.path); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Verify file permissions
	if err := verifyFilePermissions(f.path); err != nil {
		return fmt.Errorf("file permission verification failed: %w", err)
	}

	return nil
}

// ensureParentDir creates the parent directory with secure permissions.
func (f *FileBackend) ensureParentDir() error {
	dir := filepath.Dir(f.path)

	// Check if directory exists
	info, err := os.Stat(dir)
	if err == nil {
		// Directory exists, verify it's a directory
		if !info.IsDir() {
			return fmt.Errorf("parent path exists but is not a directory: %s", dir)
		}
		return nil
	}

	// Create directory with secure permissions (0700)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

// resolveMasterKey resolves the master key from various sources.
func resolveMasterKey(providedKey string) ([]byte, error) {
	// 1. Use provided key if given
	if providedKey != "" {
		return []byte(providedKey), nil
	}

	// 2. Check environment variable
	if envKey := os.Getenv("CONDUCTOR_MASTER_KEY"); envKey != "" {
		return []byte(envKey), nil
	}

	// 3. Check file
	configDir, err := os.UserConfigDir()
	if err == nil {
		keyPath := filepath.Join(configDir, "conductor", "master.key")
		if key, err := os.ReadFile(keyPath); err == nil {
			// Verify file permissions
			if err := verifyFilePermissions(keyPath); err == nil {
				return key, nil
			}
		}
	}

	// 4. For controller mode, fail here (no interactive prompt)
	// CLI mode would need to prompt, but that's handled at a higher level
	return nil, errors.New("master key not available (set CONDUCTOR_MASTER_KEY or create ~/.config/conductor/master.key)")
}

// verifyFilePermissions checks that a file has secure permissions (0600 or stricter).
func verifyFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Check if file is a symlink (security risk)
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("file is a symlink (not allowed for security)")
	}

	// Check permissions (should be 0600 or stricter)
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		return fmt.Errorf("file permissions too open (got %o, want 0600)", perm)
	}

	return nil
}

// zeroBytes securely zeros a byte slice.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
