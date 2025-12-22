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

package completion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tombee/conductor/internal/config"
)

func TestCompleteProviderNames(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := config.Default()
	cfg.Providers = config.ProvidersMap{
		"work-claude": {
			Type: "claude-code",
		},
		"personal-claude": {
			Type: "claude-code",
		},
		"test-anthropic": {
			Type: "anthropic",
		},
	}

	if err := config.WriteConfig(cfg, configPath); err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}

	// Set environment to use test config
	t.Setenv("CONDUCTOR_CONFIG", configPath)

	completions, directive := CompleteProviderNames(nil, nil, "")

	if len(completions) != 3 {
		t.Fatalf("expected 3 provider names, got %d", len(completions))
	}

	// Verify all provider names are present
	nameSet := make(map[string]bool)
	for _, name := range completions {
		nameSet[name] = true
	}

	expectedNames := []string{"work-claude", "personal-claude", "test-anthropic"}
	for _, expected := range expectedNames {
		if !nameSet[expected] {
			t.Errorf("expected provider name %q not found in completions", expected)
		}
	}

	if directive != 4 { // cobra.ShellCompDirectiveNoFileComp
		t.Errorf("expected NoFileComp directive, got %d", directive)
	}
}

func TestCompleteProviderNames_NoProviders(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := config.Default()
	cfg.Providers = config.ProvidersMap{}

	if err := config.WriteConfig(cfg, configPath); err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}

	t.Setenv("CONDUCTOR_CONFIG", configPath)

	completions, _ := CompleteProviderNames(nil, nil, "")

	if len(completions) != 0 {
		t.Errorf("expected 0 completions when no providers configured, got %d", len(completions))
	}
}

func TestCompleteProviderNames_NoConfig(t *testing.T) {
	// Point to non-existent config
	tmpDir := t.TempDir()
	t.Setenv("CONDUCTOR_CONFIG", filepath.Join(tmpDir, "nonexistent.yaml"))

	completions, directive := CompleteProviderNames(nil, nil, "")

	// Should handle gracefully
	if len(completions) != 0 {
		t.Errorf("expected 0 completions when config doesn't exist, got %d", len(completions))
	}

	if directive != 4 { // cobra.ShellCompDirectiveNoFileComp
		t.Errorf("expected NoFileComp directive, got %d", directive)
	}
}

func TestCompleteProviderNames_BadPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := config.Default()
	cfg.Providers = config.ProvidersMap{
		"test": {Type: "claude-code"},
	}

	if err := config.WriteConfig(cfg, configPath); err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}

	// Make file world-writable (bad permissions)
	if err := os.Chmod(configPath, 0644); err != nil {
		t.Fatalf("failed to chmod config: %v", err)
	}

	t.Setenv("CONDUCTOR_CONFIG", configPath)

	completions, _ := CompleteProviderNames(nil, nil, "")

	// Should reject due to bad permissions
	if len(completions) != 0 {
		t.Errorf("expected 0 completions with bad permissions, got %d", len(completions))
	}
}

func TestCompleteProviderTypes(t *testing.T) {
	completions, directive := CompleteProviderTypes(nil, nil, "")

	expectedTypes := []string{"claude-code", "anthropic", "openai", "ollama"}

	if len(completions) != len(expectedTypes) {
		t.Fatalf("expected %d provider types, got %d", len(expectedTypes), len(completions))
	}

	// Verify all expected types are present
	for i, expected := range expectedTypes {
		if completions[i] != expected {
			t.Errorf("completions[%d] = %q, want %q", i, completions[i], expected)
		}
	}

	if directive != 4 { // cobra.ShellCompDirectiveNoFileComp
		t.Errorf("expected NoFileComp directive, got %d", directive)
	}
}

func TestCompleteProviderTypes_NoConfig(t *testing.T) {
	// Provider types should be available even without config
	tmpDir := t.TempDir()
	t.Setenv("CONDUCTOR_CONFIG", filepath.Join(tmpDir, "nonexistent.yaml"))

	completions, _ := CompleteProviderTypes(nil, nil, "")

	if len(completions) != 4 {
		t.Errorf("expected 4 provider types regardless of config, got %d", len(completions))
	}
}
