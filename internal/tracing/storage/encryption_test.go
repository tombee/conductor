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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateEncryptionKey(t *testing.T) {
	key, err := GenerateEncryptionKey()
	require.NoError(t, err)
	assert.NotNil(t, key)

	// Check key is 32 bytes
	assert.Len(t, key.key, 32)

	// Keys should be unique
	key2, err := GenerateEncryptionKey()
	require.NoError(t, err)
	assert.NotEqual(t, key.String(), key2.String())
}

func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateEncryptionKey()
	require.NoError(t, err)

	plaintext := []byte("sensitive trace data")

	// Encrypt
	ciphertext, err := key.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	assert.NotEqual(t, string(plaintext), ciphertext)

	// Decrypt
	decrypted, err := key.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptDecrypt_EmptyData(t *testing.T) {
	key, err := GenerateEncryptionKey()
	require.NoError(t, err)

	plaintext := []byte("")

	// Encrypt empty data
	ciphertext, err := key.Encrypt(plaintext)
	require.NoError(t, err)

	// Decrypt should return empty (nil or empty slice are equivalent)
	decrypted, err := key.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Empty(t, decrypted)
}

func TestEncryptDecrypt_LargeData(t *testing.T) {
	key, err := GenerateEncryptionKey()
	require.NoError(t, err)

	// 1MB of data
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, err := key.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := key.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	key, err := GenerateEncryptionKey()
	require.NoError(t, err)

	// Try to decrypt invalid data
	_, err = key.Decrypt("invalid-base64")
	assert.Error(t, err)
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1, err := GenerateEncryptionKey()
	require.NoError(t, err)

	key2, err := GenerateEncryptionKey()
	require.NoError(t, err)

	plaintext := []byte("secret data")
	ciphertext, err := key1.Encrypt(plaintext)
	require.NoError(t, err)

	// Try to decrypt with wrong key
	_, err = key2.Decrypt(ciphertext)
	assert.Error(t, err)
}

func TestLoadEncryptionKey_FromEnv(t *testing.T) {
	key, err := GenerateEncryptionKey()
	require.NoError(t, err)

	// Set environment variable
	os.Setenv("CONDUCTOR_TRACE_KEY", key.String())
	defer os.Unsetenv("CONDUCTOR_TRACE_KEY")

	// Load key
	loadedKey, err := LoadEncryptionKey()
	require.NoError(t, err)
	assert.NotNil(t, loadedKey)

	// Keys should be equal
	assert.Equal(t, key.String(), loadedKey.String())
}

func TestLoadEncryptionKey_FromPassphrase(t *testing.T) {
	// Set a passphrase (not base64)
	os.Setenv("CONDUCTOR_TRACE_KEY", "my-secret-passphrase")
	defer os.Unsetenv("CONDUCTOR_TRACE_KEY")

	// Load key (should derive from passphrase)
	key, err := LoadEncryptionKey()
	require.NoError(t, err)
	assert.NotNil(t, key)

	// Should be able to encrypt/decrypt
	plaintext := []byte("test data")
	ciphertext, err := key.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := key.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestLoadEncryptionKey_NoEnv(t *testing.T) {
	os.Unsetenv("CONDUCTOR_TRACE_KEY")

	// Should return nil when no key is set
	key, err := LoadEncryptionKey()
	require.NoError(t, err)
	assert.Nil(t, key)
}

func TestEncrypt_NilKey(t *testing.T) {
	var key *EncryptionKey = nil

	_, err := key.Encrypt([]byte("data"))
	assert.Error(t, err)
}

func TestDecrypt_NilKey(t *testing.T) {
	var key *EncryptionKey = nil

	_, err := key.Decrypt("data")
	assert.Error(t, err)
}
