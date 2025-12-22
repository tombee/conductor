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
)

// TestKeychainBackend_Metadata tests the basic metadata methods.
func TestKeychainBackend_Metadata(t *testing.T) {
	backend := NewKeychainBackend()

	if backend.Name() != "keychain" {
		t.Errorf("Name() = %v, want %v", backend.Name(), "keychain")
	}

	if backend.Priority() != KeychainBackendPriority {
		t.Errorf("Priority() = %v, want %v", backend.Priority(), KeychainBackendPriority)
	}

	// Available() may be true or false depending on the system
	// Just verify it returns a boolean without panicking
	_ = backend.Available()
}

// TestKeychainBackend_Integration tests actual keychain operations.
// This is tagged as an integration test since it requires a working keychain.
func TestKeychainBackend_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	backend := NewKeychainBackend()
	if !backend.Available() {
		t.Skip("Keychain not available on this system")
	}

	ctx := context.Background()
	testKey := "test/conductor/integration_test"
	testValue := "test-secret-value"

	// Clean up before and after test
	_ = backend.Delete(ctx, testKey)
	defer func() {
		_ = backend.Delete(ctx, testKey)
	}()

	// Test Set
	err := backend.Set(ctx, testKey, testValue)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Test Get
	got, err := backend.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != testValue {
		t.Errorf("Get() = %v, want %v", got, testValue)
	}

	// Test Update (overwrite existing)
	newValue := "updated-secret-value"
	err = backend.Set(ctx, testKey, newValue)
	if err != nil {
		t.Fatalf("Set() (update) error = %v", err)
	}

	got, err = backend.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Get() (after update) error = %v", err)
	}
	if got != newValue {
		t.Errorf("Get() (after update) = %v, want %v", got, newValue)
	}

	// Test Delete
	err = backend.Delete(ctx, testKey)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's deleted
	_, err = backend.Get(ctx, testKey)
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("Get() after delete error = %v, want %v", err, ErrSecretNotFound)
	}

	// Test Delete non-existent key
	err = backend.Delete(ctx, testKey)
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("Delete() non-existent error = %v, want %v", err, ErrSecretNotFound)
	}
}

// TestKeychainBackend_List tests the List operation.
func TestKeychainBackend_List(t *testing.T) {
	backend := NewKeychainBackend()
	if !backend.Available() {
		t.Skip("Keychain not available on this system")
	}

	ctx := context.Background()

	// List should return empty slice (go-keyring doesn't support listing)
	keys, err := backend.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should return an empty list rather than nil
	if keys == nil {
		t.Error("List() returned nil, want empty slice")
	}
}

// TestIsKeychainUnavailableError tests the error detection logic.
func TestIsKeychainUnavailableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "locked keychain",
			err:  errors.New("keychain is locked"),
			want: true,
		},
		{
			name: "permission denied",
			err:  errors.New("permission denied"),
			want: true,
		},
		{
			name: "dbus error",
			err:  errors.New("failed to connect to dbus"),
			want: true,
		},
		{
			name: "user canceled",
			err:  errors.New("user canceled the operation"),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("some other error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isKeychainUnavailableError(tt.err)
			if got != tt.want {
				t.Errorf("isKeychainUnavailableError() = %v, want %v", got, tt.want)
			}
		})
	}
}
