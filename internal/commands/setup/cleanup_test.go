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

	"github.com/tombee/conductor/internal/config"
)

func TestZeroCredentials(t *testing.T) {
	// Create test state with sensitive data
	state := &SetupState{
		Working: &config.Config{
			Providers: config.ProvidersMap{
				"test": config.ProviderConfig{
					Type:   "anthropic",
					APIKey: "sk-ant-1234567890abcdef",
				},
			},
		},
	}

	// Create signal handler and zero credentials
	handler := &SignalHandler{state: state}
	handler.zeroCredentials()

	// Verify API key was zeroed
	provider := state.Working.Providers["test"]
	if provider.APIKey != "" {
		t.Errorf("Expected API key to be zeroed, got: %s", provider.APIKey)
	}
}

func TestZeroCredentialsWithModels(t *testing.T) {
	// Create test state - ProviderConfig uses ModelTierMap, not individual model configs
	// This test is simplified since ModelTierMap doesn't have per-tier API keys
	state := &SetupState{
		Working: &config.Config{
			Providers: config.ProvidersMap{
				"test": config.ProviderConfig{
					Type:   "openai-compatible",
					APIKey: "main-key-12345",
				},
			},
		},
	}

	// Zero credentials
	handler := &SignalHandler{state: state}
	handler.zeroCredentials()

	// Verify provider API key was zeroed
	provider := state.Working.Providers["test"]
	if provider.APIKey != "" {
		t.Errorf("Expected provider API key to be zeroed, got: %s", provider.APIKey)
	}
}

func TestZeroCredentialsWithNilState(t *testing.T) {
	// Should not panic with nil state
	handler := &SignalHandler{state: nil}
	handler.zeroCredentials() // Should not panic
}

func TestZeroCredentialsWithNilConfig(t *testing.T) {
	// Should not panic with nil working config
	state := &SetupState{
		Working: nil,
	}

	handler := &SignalHandler{state: state}
	handler.zeroCredentials() // Should not panic
}

func TestHandleCleanExit(t *testing.T) {
	// Create test state
	state := &SetupState{
		Working: &config.Config{
			Providers: config.ProvidersMap{
				"test": config.ProviderConfig{
					APIKey: "secret-key",
				},
			},
		},
	}

	// Should not panic
	HandleCleanExit(state)

	// Verify credentials were zeroed
	provider := state.Working.Providers["test"]
	if provider.APIKey != "" {
		t.Error("Expected credentials to be zeroed on clean exit")
	}
}
