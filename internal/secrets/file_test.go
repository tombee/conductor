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
	"os"
	"path/filepath"
	"testing"
)

func TestFileBackend_Metadata(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secrets.enc")

	backend, err := NewFileBackend(path, "test-master-key-123")
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	if backend.Name() != "file" {
		t.Errorf("Name() = %v, want %v", backend.Name(), "file")
	}

	if backend.Priority() != FileBackendPriority {
		t.Errorf("Priority() = %v, want %v", backend.Priority(), FileBackendPriority)
	}

	if !backend.Available() {
		t.Error("Available() = false, want true")
	}
}

func TestFileBackend_SetGetDelete(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secrets.enc")
	masterKey := "test-master-key-for-encryption-123"

	backend, err := NewFileBackend(path, masterKey)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	ctx := context.Background()

	// Test Set
	err = backend.Set(ctx, "test/key1", "value1")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Encrypted file was not created")
	}

	// Verify file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("File permissions = %o, want 0600", info.Mode().Perm())
	}

	// Test Get
	value, err := backend.Get(ctx, "test/key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}

	// Test Get non-existent
	_, err = backend.Get(ctx, "test/missing")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("Get() non-existent error = %v, want %v", err, ErrSecretNotFound)
	}

	// Test Update
	err = backend.Set(ctx, "test/key1", "updated-value")
	if err != nil {
		t.Fatalf("Set() (update) error = %v", err)
	}

	value, err = backend.Get(ctx, "test/key1")
	if err != nil {
		t.Fatalf("Get() (after update) error = %v", err)
	}
	if value != "updated-value" {
		t.Errorf("Get() (after update) = %v, want %v", value, "updated-value")
	}

	// Test Delete
	err = backend.Delete(ctx, "test/key1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err = backend.Get(ctx, "test/key1")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("Get() after delete error = %v, want %v", err, ErrSecretNotFound)
	}

	// Test Delete non-existent
	err = backend.Delete(ctx, "test/key1")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("Delete() non-existent error = %v, want %v", err, ErrSecretNotFound)
	}
}

func TestFileBackend_List(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secrets.enc")
	masterKey := "test-master-key-for-listing-456"

	backend, err := NewFileBackend(path, masterKey)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	ctx := context.Background()

	// List empty backend
	keys, err := backend.List(ctx)
	if err != nil {
		t.Fatalf("List() empty error = %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("List() empty returned %d keys, want 0", len(keys))
	}

	// Add some secrets
	backend.Set(ctx, "key1", "value1")
	backend.Set(ctx, "key2", "value2")
	backend.Set(ctx, "key3", "value3")

	// List all
	keys, err = backend.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("List() returned %d keys, want 3", len(keys))
	}

	// Verify all keys are present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	expectedKeys := []string{"key1", "key2", "key3"}
	for _, k := range expectedKeys {
		if !keyMap[k] {
			t.Errorf("List() missing key %q", k)
		}
	}
}

func TestFileBackend_EncryptionRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secrets.enc")
	masterKey := "test-encryption-round-trip-key"

	// Create first backend
	backend1, err := NewFileBackend(path, masterKey)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	ctx := context.Background()

	// Store multiple secrets
	secrets := map[string]string{
		"api/key1": "secret-value-1",
		"api/key2": "secret-value-2",
		"db/pass":  "database-password",
	}

	for k, v := range secrets {
		if err := backend1.Set(ctx, k, v); err != nil {
			t.Fatalf("Set(%q) error = %v", k, err)
		}
	}

	// Create new backend instance with same key
	backend2, err := NewFileBackend(path, masterKey)
	if err != nil {
		t.Fatalf("NewFileBackend() (second) error = %v", err)
	}

	// Verify all secrets can be read
	for k, want := range secrets {
		got, err := backend2.Get(ctx, k)
		if err != nil {
			t.Errorf("Get(%q) error = %v", k, err)
			continue
		}
		if got != want {
			t.Errorf("Get(%q) = %v, want %v", k, got, want)
		}
	}
}

func TestFileBackend_WrongMasterKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secrets.enc")

	// Create and store with one key
	backend1, err := NewFileBackend(path, "correct-key")
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	ctx := context.Background()
	err = backend1.Set(ctx, "test/key", "value")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Try to read with wrong key
	backend2, err := NewFileBackend(path, "wrong-key")
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	_, err = backend2.Get(ctx, "test/key")
	if err == nil {
		t.Error("Get() with wrong key succeeded, want error")
	}
}

func TestFileBackend_NoMasterKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secrets.enc")

	// Create backend without master key
	backend, err := NewFileBackend(path, "")
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	// Should be unavailable
	if backend.Available() {
		t.Error("Available() = true, want false (no master key)")
	}

	ctx := context.Background()

	// Operations should fail
	_, err = backend.Get(ctx, "test/key")
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Errorf("Get() error = %v, want %v", err, ErrBackendUnavailable)
	}

	err = backend.Set(ctx, "test/key", "value")
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Errorf("Set() error = %v, want %v", err, ErrBackendUnavailable)
	}

	err = backend.Delete(ctx, "test/key")
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Errorf("Delete() error = %v, want %v", err, ErrBackendUnavailable)
	}
}

func TestFileBackend_ResolveMasterKeyFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secrets.enc")

	// Set master key via environment
	t.Setenv("CONDUCTOR_MASTER_KEY", "env-master-key-789")

	backend, err := NewFileBackend(path, "")
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	if !backend.Available() {
		t.Error("Available() = false, want true (env key set)")
	}

	// Test that it works
	ctx := context.Background()
	err = backend.Set(ctx, "test/key", "value")
	if err != nil {
		t.Errorf("Set() with env key error = %v", err)
	}

	value, err := backend.Get(ctx, "test/key")
	if err != nil {
		t.Errorf("Get() with env key error = %v", err)
	}
	if value != "value" {
		t.Errorf("Get() = %v, want %v", value, "value")
	}
}

func TestFileBackend_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secrets.enc")
	masterKey := "concurrent-test-key"

	backend, err := NewFileBackend(path, masterKey)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	ctx := context.Background()

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			key := filepath.Join("concurrent", "key", string(rune('0'+n)))
			value := filepath.Join("value", string(rune('0'+n)))
			backend.Set(ctx, key, value)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all values
	for i := 0; i < 10; i++ {
		key := filepath.Join("concurrent", "key", string(rune('0'+i)))
		expectedValue := filepath.Join("value", string(rune('0'+i)))
		value, err := backend.Get(ctx, key)
		if err != nil {
			t.Errorf("Get(%q) error = %v", key, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Get(%q) = %v, want %v", key, value, expectedValue)
		}
	}
}

func TestZeroBytes(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	zeroBytes(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("zeroBytes() data[%d] = %d, want 0", i, b)
		}
	}
}

func TestVerifyFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		perm    os.FileMode
		wantErr bool
	}{
		{
			name:    "secure permissions 0600",
			perm:    0600,
			wantErr: false,
		},
		{
			name:    "secure permissions 0400",
			perm:    0400,
			wantErr: false,
		},
		{
			name:    "insecure permissions 0644",
			perm:    0644,
			wantErr: true,
		},
		{
			name:    "insecure permissions 0666",
			perm:    0666,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name)
			err := os.WriteFile(path, []byte("test"), tt.perm)
			if err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			err = verifyFilePermissions(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("verifyFilePermissions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
