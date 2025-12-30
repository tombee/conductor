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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrInvalidCiphertext is returned when ciphertext cannot be decrypted
	ErrInvalidCiphertext = errors.New("invalid ciphertext")

	// ErrInvalidKey is returned when the encryption key is invalid
	ErrInvalidKey = errors.New("invalid encryption key")
)

// AESEncryptor implements credential encryption using AES-256-GCM.
//
// AES-256-GCM provides:
//   - Confidentiality through AES-256 encryption
//   - Authenticity through Galois/Counter Mode
//   - Protection against tampering and forgery attacks
//
// The master key is derived from the system keychain or CONDUCTOR_MASTER_KEY environment variable.
// Each encryption operation generates a unique nonce (number used once) for security.
type AESEncryptor struct {
	// masterKey is the 32-byte AES-256 key
	masterKey []byte

	// aead is the Galois/Counter Mode cipher
	aead cipher.AEAD
}

// NewAESEncryptor creates a new AES-256-GCM encryptor.
//
// The masterKey must be exactly 32 bytes (256 bits) for AES-256.
// Use GenerateKey() to create a cryptographically secure random key,
// or derive from keychain via KeychainManager.
func NewAESEncryptor(masterKey []byte) (*AESEncryptor, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("%w: key must be 32 bytes for AES-256, got %d bytes", ErrInvalidKey, len(masterKey))
	}

	// Create AES-256 cipher
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM cipher mode
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	return &AESEncryptor{
		masterKey: masterKey,
		aead:      aead,
	}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
//
// Format of returned ciphertext:
//
//	[nonce (12 bytes)][encrypted data + auth tag (variable length)]
//
// The nonce is prepended to the ciphertext for easy retrieval during decryption.
// The authentication tag is automatically appended by GCM.
func (e *AESEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, errors.New("plaintext cannot be empty")
	}

	// Generate a random nonce
	// GCM standard nonce size is 12 bytes
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	// The result includes the encrypted plaintext and authentication tag
	ciphertext := e.aead.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM.
//
// The ciphertext must be in the format produced by Encrypt():
//
//	[nonce (12 bytes)][encrypted data + auth tag]
//
// Returns ErrInvalidCiphertext if:
//   - Ciphertext is too short (less than nonce size)
//   - Authentication tag verification fails (data has been tampered with)
//   - Decryption fails for any other reason
func (e *AESEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := e.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("%w: ciphertext too short (expected at least %d bytes, got %d)",
			ErrInvalidCiphertext, nonceSize, len(ciphertext))
	}

	// Extract nonce and encrypted data
	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt and verify authentication tag
	plaintext, err := e.aead.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCiphertext, err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64-encoded ciphertext.
// This is a convenience wrapper around Encrypt() for string values.
func (e *AESEncryptor) EncryptString(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	ciphertext, err := e.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts base64-encoded ciphertext and returns a string.
// This is a convenience wrapper around Decrypt() for string values.
func (e *AESEncryptor) DecryptString(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 ciphertext: %w", err)
	}

	plaintext, err := e.Decrypt(decoded)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GenerateKey generates a cryptographically secure random 32-byte key for AES-256.
// This should be used when creating a new master key for the first time.
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32) // 256 bits for AES-256
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return key, nil
}
