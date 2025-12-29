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

package runner

import (
	"context"
	"testing"

	"github.com/tombee/conductor/internal/binding"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/daemon/backend/memory"
	"github.com/tombee/conductor/pkg/profile"
)

// TestSubmitWithProfile tests that profile information is correctly stored in the run.
func TestSubmitWithProfile(t *testing.T) {
	// Create test config with workspaces and profiles
	cfg := &config.Config{
		Workspaces: map[string]config.Workspace{
			"test-workspace": {
				Name: "test-workspace",
				Profiles: map[string]profile.Profile{
					"test-profile": {
						Name: "test-profile",
						Bindings: profile.Bindings{
							Integrations: map[string]profile.IntegrationBinding{},
						},
						InheritEnv: profile.InheritEnvConfig{
							Enabled: false,
						},
					},
				},
				DefaultProfile: "test-profile",
			},
		},
	}

	// Create a mock secret registry
	mockRegistry := &mockSecretRegistry{}

	// Create binding resolver
	resolver := binding.NewResolver(mockRegistry, profile.InheritEnvConfig{Enabled: false})

	// Create runner with config and resolver
	be := memory.New()
	r := New(Config{MaxParallel: 1}, be, nil,
		WithConfig(cfg),
		WithBindingResolver(resolver),
	)

	// Set mock adapter
	r.SetAdapter(&MockExecutionAdapter{})

	// Create test workflow
	workflowYAML := []byte(`
name: test-workflow
agents:
  test:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
steps:
  - name: step1
    type: llm
    agent: test
    prompt: "test"
`)

	ctx := context.Background()
	snapshot, err := r.Submit(ctx, SubmitRequest{
		WorkflowYAML: workflowYAML,
		Workspace:    "test-workspace",
		Profile:      "test-profile",
	})

	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Verify workspace and profile are set in snapshot
	if snapshot.Workspace != "test-workspace" {
		t.Errorf("Expected workspace 'test-workspace', got '%s'", snapshot.Workspace)
	}

	if snapshot.Profile != "test-profile" {
		t.Errorf("Expected profile 'test-profile', got '%s'", snapshot.Profile)
	}
}

// TestSubmitWithDefaultProfile tests profile defaults.
func TestSubmitWithDefaultProfile(t *testing.T) {
	cfg := &config.Config{
		Workspaces: map[string]config.Workspace{
			"default": {
				Name: "default",
				Profiles: map[string]profile.Profile{
					"default": {
						Name: "default",
						Bindings: profile.Bindings{
							Integrations: map[string]profile.IntegrationBinding{},
						},
						InheritEnv: profile.InheritEnvConfig{
							Enabled: true,
						},
					},
				},
				DefaultProfile: "default",
			},
		},
	}

	mockRegistry := &mockSecretRegistry{}
	resolver := binding.NewResolver(mockRegistry, profile.InheritEnvConfig{Enabled: true})

	be := memory.New()
	r := New(Config{MaxParallel: 1}, be, nil,
		WithConfig(cfg),
		WithBindingResolver(resolver),
	)
	r.SetAdapter(&MockExecutionAdapter{})

	workflowYAML := []byte(`
name: test-workflow
agents:
  test:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
steps:
  - name: step1
    type: llm
    agent: test
    prompt: "test"
`)

	ctx := context.Background()
	snapshot, err := r.Submit(ctx, SubmitRequest{
		WorkflowYAML: workflowYAML,
		// No workspace/profile specified - should use defaults
	})

	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Should use default workspace and profile
	if snapshot.Workspace != "default" {
		t.Errorf("Expected workspace 'default', got '%s'", snapshot.Workspace)
	}

	if snapshot.Profile != "default" {
		t.Errorf("Expected profile 'default', got '%s'", snapshot.Profile)
	}
}

// TestSubmitWithoutProfileSupport tests backward compatibility when no resolver is configured.
func TestSubmitWithoutProfileSupport(t *testing.T) {
	be := memory.New()
	r := New(Config{MaxParallel: 1}, be, nil)
	r.SetAdapter(&MockExecutionAdapter{})

	workflowYAML := []byte(`
name: test-workflow
agents:
  test:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
steps:
  - name: step1
    type: llm
    agent: test
    prompt: "test"
`)

	ctx := context.Background()
	snapshot, err := r.Submit(ctx, SubmitRequest{
		WorkflowYAML: workflowYAML,
	})

	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Without profile support, workspace and profile should be empty
	if snapshot.Workspace != "" {
		t.Errorf("Expected empty workspace, got '%s'", snapshot.Workspace)
	}

	if snapshot.Profile != "" {
		t.Errorf("Expected empty profile, got '%s'", snapshot.Profile)
	}
}

// TestSubmitWithInvalidWorkspace tests error handling for missing workspace.
func TestSubmitWithInvalidWorkspace(t *testing.T) {
	cfg := &config.Config{
		Workspaces: map[string]config.Workspace{
			"valid": {
				Name: "valid",
				Profiles: map[string]profile.Profile{
					"default": {
						Name: "default",
					},
				},
			},
		},
	}

	mockRegistry := &mockSecretRegistry{}
	resolver := binding.NewResolver(mockRegistry, profile.InheritEnvConfig{Enabled: false})

	be := memory.New()
	r := New(Config{MaxParallel: 1}, be, nil,
		WithConfig(cfg),
		WithBindingResolver(resolver),
	)
	r.SetAdapter(&MockExecutionAdapter{})

	workflowYAML := []byte(`
name: test-workflow
agents:
  test:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
steps:
  - name: step1
    type: llm
    agent: test
    prompt: "test"
`)

	ctx := context.Background()
	_, err := r.Submit(ctx, SubmitRequest{
		WorkflowYAML: workflowYAML,
		Workspace:    "invalid",
		Profile:      "default",
	})

	if err == nil {
		t.Fatal("Expected error for invalid workspace, got nil")
	}

	if !contains(err.Error(), "workspace not found") {
		t.Errorf("Expected 'workspace not found' error, got: %v", err)
	}
}

// TestSubmitWithInvalidProfile tests error handling for missing profile.
func TestSubmitWithInvalidProfile(t *testing.T) {
	cfg := &config.Config{
		Workspaces: map[string]config.Workspace{
			"test": {
				Name: "test",
				Profiles: map[string]profile.Profile{
					"valid": {
						Name: "valid",
					},
				},
			},
		},
	}

	mockRegistry := &mockSecretRegistry{}
	resolver := binding.NewResolver(mockRegistry, profile.InheritEnvConfig{Enabled: false})

	be := memory.New()
	r := New(Config{MaxParallel: 1}, be, nil,
		WithConfig(cfg),
		WithBindingResolver(resolver),
	)
	r.SetAdapter(&MockExecutionAdapter{})

	workflowYAML := []byte(`
name: test-workflow
agents:
  test:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
steps:
  - name: step1
    type: llm
    agent: test
    prompt: "test"
`)

	ctx := context.Background()
	_, err := r.Submit(ctx, SubmitRequest{
		WorkflowYAML: workflowYAML,
		Workspace:    "test",
		Profile:      "invalid",
	})

	if err == nil {
		t.Fatal("Expected error for invalid profile, got nil")
	}

	if !contains(err.Error(), "profile not found") {
		t.Errorf("Expected 'profile not found' error, got: %v", err)
	}
}

// mockSecretRegistry is a mock implementation of SecretProviderRegistry for testing.
type mockSecretRegistry struct{}

func (m *mockSecretRegistry) Register(provider profile.SecretProvider) error {
	return nil
}

func (m *mockSecretRegistry) Resolve(ctx context.Context, reference string) (string, error) {
	// Return dummy value for any secret reference
	return "resolved-secret", nil
}

func (m *mockSecretRegistry) GetProvider(scheme string) profile.SecretProvider {
	// Return nil - not used in these tests
	return nil
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsCheck(s, substr))
}

func containsCheck(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
