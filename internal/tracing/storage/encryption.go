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

package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// EncryptionKey represents an encryption key for data at rest.
type EncryptionKey struct {
	key []byte
}

// LoadEncryptionKey loads the encryption key from environment variable or keychain.
// The key should be 32 bytes for AES-256.
func LoadEncryptionKey() (*EncryptionKey, error) {
	// Try environment variable first
	keyStr := os.Getenv("CONDUCTOR_TRACE_KEY")
	if keyStr == "" {
		// In a production system, we would fall back to system keychain here
		// For now, return nil to indicate encryption is disabled
		return nil, nil
	}

	// Decode the base64-encoded key
	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		// If decoding fails, treat the string as a passphrase and derive a key
		keyBytes = deriveKey(keyStr)
	}

	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes for AES-256, got %d bytes", len(keyBytes))
	}

	return &EncryptionKey{key: keyBytes}, nil
}

// GenerateEncryptionKey generates a new random 32-byte encryption key.
func GenerateEncryptionKey() (*EncryptionKey, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	return &EncryptionKey{key: key}, nil
}

// String returns the base64-encoded key for storage/display.
func (k *EncryptionKey) String() string {
	return base64.StdEncoding.EncodeToString(k.key)
}

// deriveKey derives a 32-byte key from a passphrase using SHA-256.
func deriveKey(passphrase string) []byte {
	hash := sha256.Sum256([]byte(passphrase))
	return hash[:]
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns base64-encoded ciphertext with nonce prepended.
func (k *EncryptionKey) Encrypt(plaintext []byte) (string, error) {
	if k == nil {
		return "", fmt.Errorf("encryption key is nil")
	}

	block, err := aes.NewCipher(k.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Base64 encode for storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM.
// Expects nonce to be prepended to the ciphertext.
func (k *EncryptionKey) Decrypt(encoded string) ([]byte, error) {
	if k == nil {
		return nil, fmt.Errorf("encryption key is nil")
	}

	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(k.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
