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
	"context"
	"testing"
)

func TestGetProviderTypes(t *testing.T) {
	types := GetProviderTypes()

	if len(types) == 0 {
		t.Fatal("GetProviderTypes() returned no types")
	}

	// Verify all expected provider types are present
	expectedTypes := map[string]bool{
		"claude-code":       false,
		"ollama":            false,
		"anthropic":         false,
		"openai-compatible": false,
	}

	for _, pt := range types {
		if _, ok := expectedTypes[pt.Name()]; ok {
			expectedTypes[pt.Name()] = true
		}
	}

	// Check all expected types were found
	for name, found := range expectedTypes {
		if !found {
			t.Errorf("Expected provider type %s not found", name)
		}
	}
}

func TestGetProviderType(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		wantOK   bool
	}{
		{
			name:     "claude-code exists",
			typeName: "claude-code",
			wantOK:   true,
		},
		{
			name:     "ollama exists",
			typeName: "ollama",
			wantOK:   true,
		},
		{
			name:     "anthropic exists",
			typeName: "anthropic",
			wantOK:   true,
		},
		{
			name:     "openai-compatible exists",
			typeName: "openai-compatible",
			wantOK:   true,
		},
		{
			name:     "unknown type",
			typeName: "unknown",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt, ok := GetProviderType(tt.typeName)
			if ok != tt.wantOK {
				t.Errorf("GetProviderType(%q) ok = %v, want %v", tt.typeName, ok, tt.wantOK)
			}
			if ok && pt.Name() != tt.typeName {
				t.Errorf("GetProviderType(%q).Name() = %q, want %q", tt.typeName, pt.Name(), tt.typeName)
			}
		})
	}
}

func TestProviderTypeFields(t *testing.T) {
	types := GetProviderTypes()

	for _, pt := range types {
		t.Run(pt.Name(), func(t *testing.T) {
			if pt.Name() == "" {
				t.Error("Name() is empty")
			}
			if pt.DisplayName() == "" {
				t.Error("DisplayName() is empty")
			}
			if pt.Description() == "" {
				t.Error("Description() is empty")
			}

			// Test config creation
			cfg := pt.CreateConfig()
			if cfg.Type != pt.Name() {
				t.Errorf("CreateConfig().Type = %q, want %q", cfg.Type, pt.Name())
			}

			// Test validation
			if err := pt.ValidateConfig(cfg); err != nil {
				t.Errorf("ValidateConfig() error = %v", err)
			}
		})
	}
}

func TestCLIProviderDetection(t *testing.T) {
	ctx := context.Background()

	cliProviders := []string{"claude-code", "ollama"}

	for _, typeName := range cliProviders {
		t.Run(typeName, func(t *testing.T) {
			pt, ok := GetProviderType(typeName)
			if !ok {
				t.Fatalf("Provider type %s not found", typeName)
			}

			if !pt.IsCLI() {
				t.Errorf("%s should be a CLI provider", typeName)
			}

			// Try to detect (won't error even if not found)
			found, version, err := pt.DetectCLI(ctx)
			if err != nil {
				t.Errorf("DetectCLI() error = %v", err)
			}

			if found {
				t.Logf("%s detected: %s", typeName, version)
			} else {
				t.Logf("%s not found on system", typeName)
			}
		})
	}
}

func TestAPIProviderCharacteristics(t *testing.T) {
	tests := []struct {
		typeName        string
		wantRequiresKey bool
		wantRequiresURL bool
		wantDefaultURL  string
	}{
		{
			typeName:        "anthropic",
			wantRequiresKey: true,
			wantRequiresURL: false,
			wantDefaultURL:  "https://api.anthropic.com",
		},
		{
			typeName:        "openai-compatible",
			wantRequiresKey: true,
			wantRequiresURL: true,
			wantDefaultURL:  "https://api.openai.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			pt, ok := GetProviderType(tt.typeName)
			if !ok {
				t.Fatalf("Provider type %s not found", tt.typeName)
			}

			if pt.RequiresAPIKey() != tt.wantRequiresKey {
				t.Errorf("RequiresAPIKey() = %v, want %v", pt.RequiresAPIKey(), tt.wantRequiresKey)
			}

			if pt.RequiresBaseURL() != tt.wantRequiresURL {
				t.Errorf("RequiresBaseURL() = %v, want %v", pt.RequiresBaseURL(), tt.wantRequiresURL)
			}

			if pt.DefaultBaseURL() != tt.wantDefaultURL {
				t.Errorf("DefaultBaseURL() = %q, want %q", pt.DefaultBaseURL(), tt.wantDefaultURL)
			}
		})
	}
}
