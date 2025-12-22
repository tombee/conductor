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

func TestEnvBackend_Get(t *testing.T) {
	backend := NewEnvBackend()
	ctx := context.Background()

	tests := []struct {
		name      string
		key       string
		envVars   map[string]string
		wantValue string
		wantErr   error
	}{
		{
			name: "normalized key found",
			key:  "providers/anthropic/api_key",
			envVars: map[string]string{
				"CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY": "sk-ant-test",
			},
			wantValue: "sk-ant-test",
			wantErr:   nil,
		},
		{
			name: "provider alias found",
			key:  "providers/anthropic/api_key",
			envVars: map[string]string{
				"ANTHROPIC_API_KEY": "sk-ant-alias",
			},
			wantValue: "sk-ant-alias",
			wantErr:   nil,
		},
		{
			name: "normalized takes precedence over alias",
			key:  "providers/anthropic/api_key",
			envVars: map[string]string{
				"CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY": "sk-ant-normalized",
				"ANTHROPIC_API_KEY":                            "sk-ant-alias",
			},
			wantValue: "sk-ant-normalized",
			wantErr:   nil,
		},
		{
			name:      "key not found",
			key:       "providers/missing/api_key",
			envVars:   map[string]string{},
			wantValue: "",
			wantErr:   ErrSecretNotFound,
		},
		{
			name: "OpenAI provider alias",
			key:  "providers/openai/api_key",
			envVars: map[string]string{
				"OPENAI_API_KEY": "sk-openai-test",
			},
			wantValue: "sk-openai-test",
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			got, err := backend.Get(ctx, tt.key)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantValue {
				t.Errorf("Get() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

func TestEnvBackend_Set(t *testing.T) {
	backend := NewEnvBackend()
	ctx := context.Background()

	err := backend.Set(ctx, "test/key", "value")
	if !errors.Is(err, ErrReadOnlyBackend) {
		t.Errorf("Set() error = %v, want %v", err, ErrReadOnlyBackend)
	}
}

func TestEnvBackend_Delete(t *testing.T) {
	backend := NewEnvBackend()
	ctx := context.Background()

	err := backend.Delete(ctx, "test/key")
	if !errors.Is(err, ErrReadOnlyBackend) {
		t.Errorf("Delete() error = %v, want %v", err, ErrReadOnlyBackend)
	}
}

func TestEnvBackend_List(t *testing.T) {
	backend := NewEnvBackend()
	ctx := context.Background()

	// Set up test environment variables
	t.Setenv("CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY", "sk-test1")
	t.Setenv("CONDUCTOR_SECRET_PROVIDERS_OPENAI_API_KEY", "sk-test2")
	t.Setenv("CONDUCTOR_SECRET_WEBHOOKS_GITHUB_SECRET", "gh-secret")
	t.Setenv("ANTHROPIC_API_KEY", "ignored") // Should not appear in list

	keys, err := backend.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	want := []string{
		"providers/anthropic/api_key",
		"providers/openai/api_key",
		"webhooks/github/secret",
	}

	if len(keys) != len(want) {
		t.Errorf("List() returned %d keys, want %d", len(keys), len(want))
	}

	// Check that all expected keys are present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	for _, w := range want {
		if !keyMap[w] {
			t.Errorf("List() missing key %q", w)
		}
	}
}

func TestEnvBackend_Metadata(t *testing.T) {
	backend := NewEnvBackend()

	if backend.Name() != "env" {
		t.Errorf("Name() = %v, want %v", backend.Name(), "env")
	}

	if !backend.Available() {
		t.Error("Available() = false, want true")
	}

	if backend.Priority() != EnvBackendPriority {
		t.Errorf("Priority() = %v, want %v", backend.Priority(), EnvBackendPriority)
	}

	if !backend.ReadOnly() {
		t.Error("ReadOnly() = false, want true")
	}
}

func TestEnvBackend_NormalizeKey(t *testing.T) {
	backend := NewEnvBackend()

	tests := []struct {
		key  string
		want string
	}{
		{
			key:  "providers/anthropic/api_key",
			want: "CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY",
		},
		{
			key:  "webhooks/github/secret",
			want: "CONDUCTOR_SECRET_WEBHOOKS_GITHUB_SECRET",
		},
		{
			key:  "simple",
			want: "CONDUCTOR_SECRET_SIMPLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := backend.normalizeKey(tt.key)
			if got != tt.want {
				t.Errorf("normalizeKey() = %v, want %v", got, tt.want)
			}

			// Verify round-trip
			denormalized := backend.denormalizeKey(got)
			if denormalized != tt.key {
				t.Errorf("denormalizeKey() = %v, want %v", denormalized, tt.key)
			}
		})
	}
}

func TestEnvBackend_ProviderAlias(t *testing.T) {
	backend := NewEnvBackend()

	tests := []struct {
		key  string
		want string
	}{
		{
			key:  "providers/anthropic/api_key",
			want: "ANTHROPIC_API_KEY",
		},
		{
			key:  "providers/openai/api_key",
			want: "OPENAI_API_KEY",
		},
		{
			key:  "webhooks/github/secret",
			want: "",
		},
		{
			key:  "providers/anthropic/other",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := backend.providerAlias(tt.key)
			if got != tt.want {
				t.Errorf("providerAlias() = %v, want %v", got, tt.want)
			}
		})
	}
}
