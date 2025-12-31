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

package setup

import (
	"testing"
)

func TestGetAvailableBackends(t *testing.T) {
	backends := GetAvailableBackends()

	if len(backends) == 0 {
		t.Fatal("GetAvailableBackends() returned no backends")
	}

	// Verify all expected backends are present
	expectedBackends := map[string]bool{
		"keychain": false,
		"env":      false,
	}

	for _, backend := range backends {
		if _, ok := expectedBackends[backend.Name]; ok {
			expectedBackends[backend.Name] = true
		}

		// Verify required fields are populated
		if backend.Name == "" {
			t.Errorf("Backend has empty Name")
		}
		if backend.DisplayName == "" {
			t.Errorf("Backend %s has empty DisplayName", backend.Name)
		}
		if backend.Description == "" {
			t.Errorf("Backend %s has empty Description", backend.Name)
		}
	}

	// Check all expected backends were found
	for name, found := range expectedBackends {
		if !found {
			t.Errorf("Expected backend %s not found", name)
		}
	}

	// Verify env is always available
	for _, backend := range backends {
		if backend.Name == "env" && !backend.Available {
			t.Errorf("Backend %s should always be available", backend.Name)
		}
	}
}

func TestGetRecommendedBackend(t *testing.T) {
	recommended := GetRecommendedBackend()

	// Should return either keychain or env
	if recommended != "keychain" && recommended != "env" {
		t.Errorf("GetRecommendedBackend() = %q, want 'keychain' or 'env'", recommended)
	}

	// If keychain is available, it should be recommended
	if isKeychainAvailable() && recommended != "keychain" {
		t.Errorf("GetRecommendedBackend() = %q, want 'keychain' (keychain is available)", recommended)
	}

	// If keychain is not available, env should be recommended
	if !isKeychainAvailable() && recommended != "env" {
		t.Errorf("GetRecommendedBackend() = %q, want 'env' (keychain is not available)", recommended)
	}
}

func TestBackendInfoFields(t *testing.T) {
	backends := GetAvailableBackends()

	for _, backend := range backends {
		t.Run(backend.Name, func(t *testing.T) {
			if backend.Name == "" {
				t.Error("Name is empty")
			}
			if backend.DisplayName == "" {
				t.Error("DisplayName is empty")
			}
			if backend.Description == "" {
				t.Error("Description is empty")
			}
		})
	}
}
