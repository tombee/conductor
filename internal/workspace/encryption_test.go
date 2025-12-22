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
	"bytes"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	if len(key) != 32 {
		t.Errorf("GenerateKey() key length = %d, want 32", len(key))
	}

	// Generate another key to ensure randomness
	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	if bytes.Equal(key, key2) {
		t.Error("GenerateKey() generated identical keys (should be random)")
	}
}

func TestNewAESEncryptor(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
		wantErr bool
	}{
		{
			name:    "valid 32-byte key",
			keySize: 32,
			wantErr: false,
		},
		{
			name:    "invalid 16-byte key",
			keySize: 16,
			wantErr: true,
		},
		{
			name:    "invalid 24-byte key",
			keySize: 24,
			wantErr: true,
		},
		{
			name:    "invalid 64-byte key",
			keySize: 64,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keySize)
			_, err := NewAESEncryptor(key)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAESEncryptor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAESEncryptor_EncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	encryptor, err := NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("NewAESEncryptor() error = %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple text",
			plaintext: "hello world",
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "long text",
			plaintext: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
		},
		{
			name:      "special characters",
			plaintext: "!@#$%^&*()_+-=[]{}|;:,.<>?",
		},
		{
			name:      "unicode",
			plaintext: "Hello 世界 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip empty string test for Encrypt (it returns error)
			if tt.plaintext == "" {
				_, err := encryptor.Encrypt([]byte(tt.plaintext))
				if err == nil {
					t.Error("Encrypt() should error on empty plaintext")
				}
				return
			}

			// Encrypt
			encrypted, err := encryptor.Encrypt([]byte(tt.plaintext))
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Verify ciphertext is different from plaintext
			if bytes.Equal(encrypted, []byte(tt.plaintext)) {
				t.Error("Encrypt() ciphertext equals plaintext")
			}

			// Decrypt
			decrypted, err := encryptor.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			// Verify decrypted matches original
			if string(decrypted) != tt.plaintext {
				t.Errorf("Decrypt() = %q, want %q", string(decrypted), tt.plaintext)
			}
		})
	}
}

func TestAESEncryptor_EncryptString(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	encryptor, err := NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("NewAESEncryptor() error = %v", err)
	}

	// Test encryption and decryption
	plaintext := "my secret token"

	encrypted, err := encryptor.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}

	if encrypted == plaintext {
		t.Error("EncryptString() ciphertext equals plaintext")
	}

	decrypted, err := encryptor.DecryptString(encrypted)
	if err != nil {
		t.Fatalf("DecryptString() error = %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("DecryptString() = %q, want %q", decrypted, plaintext)
	}
}

func TestAESEncryptor_DecryptInvalid(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	encryptor, err := NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("NewAESEncryptor() error = %v", err)
	}

	tests := []struct {
		name       string
		ciphertext []byte
		wantErr    bool
	}{
		{
			name:       "too short",
			ciphertext: []byte{1, 2, 3},
			wantErr:    true,
		},
		{
			name:       "random data",
			ciphertext: make([]byte, 100),
			wantErr:    true,
		},
		{
			name:       "empty",
			ciphertext: []byte{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := encryptor.Decrypt(tt.ciphertext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decrypt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAESEncryptor_DifferentNonces(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	encryptor, err := NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("NewAESEncryptor() error = %v", err)
	}

	plaintext := []byte("same plaintext")

	// Encrypt same plaintext multiple times
	encrypted1, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	encrypted2, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Ciphertexts should be different (different nonces)
	if bytes.Equal(encrypted1, encrypted2) {
		t.Error("Encrypt() produced identical ciphertexts for same plaintext (nonces should differ)")
	}

	// But both should decrypt to same plaintext
	decrypted1, err := encryptor.Decrypt(encrypted1)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	decrypted2, err := encryptor.Decrypt(encrypted2)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("Decrypt() failed to recover original plaintext")
	}
}

func TestAESEncryptor_WrongKey(t *testing.T) {
	// Encrypt with one key
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	encryptor1, err := NewAESEncryptor(key1)
	if err != nil {
		t.Fatalf("NewAESEncryptor() error = %v", err)
	}

	plaintext := []byte("secret data")
	encrypted, err := encryptor1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Try to decrypt with different key
	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	encryptor2, err := NewAESEncryptor(key2)
	if err != nil {
		t.Fatalf("NewAESEncryptor() error = %v", err)
	}

	_, err = encryptor2.Decrypt(encrypted)
	if err == nil {
		t.Error("Decrypt() should fail with wrong key")
	}
}
