package workspace

import (
	"context"
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
)

// mockStorage is a mock implementation of the Storage interface for testing.
type mockStorage struct {
	integrations map[string][]*Integration
}

func (m *mockStorage) GetIntegration(ctx context.Context, workspaceName, name string) (*Integration, error) {
	integrations, ok := m.integrations[workspaceName]
	if !ok {
		return nil, ErrIntegrationNotFound
	}

	for _, integration := range integrations {
		if integration.Name == name {
			return integration, nil
		}
	}

	return nil, ErrIntegrationNotFound
}

func (m *mockStorage) ListIntegrations(ctx context.Context, workspaceName string) ([]*Integration, error) {
	integrations, ok := m.integrations[workspaceName]
	if !ok {
		return []*Integration{}, nil
	}
	return integrations, nil
}

// Stub methods for other Storage interface methods (not used in resolver tests)
func (m *mockStorage) CreateWorkspace(ctx context.Context, workspace *Workspace) error {
	return nil
}
func (m *mockStorage) GetWorkspace(ctx context.Context, name string) (*Workspace, error) {
	return nil, nil
}
func (m *mockStorage) ListWorkspaces(ctx context.Context) ([]*Workspace, error) {
	return nil, nil
}
func (m *mockStorage) UpdateWorkspace(ctx context.Context, workspace *Workspace) error {
	return nil
}
func (m *mockStorage) DeleteWorkspace(ctx context.Context, name string) error {
	return nil
}
func (m *mockStorage) CreateIntegration(ctx context.Context, integration *Integration) error {
	return nil
}
func (m *mockStorage) ListIntegrationsByType(ctx context.Context, workspaceName, integrationType string) ([]*Integration, error) {
	return nil, nil
}
func (m *mockStorage) UpdateIntegration(ctx context.Context, integration *Integration) error {
	return nil
}
func (m *mockStorage) DeleteIntegration(ctx context.Context, workspaceName, name string) error {
	return nil
}
func (m *mockStorage) Close() error {
	return nil
}

func TestBindingResolver_ResolveBindings(t *testing.T) {
	tests := []struct {
		name             string
		workflowRequires []string
		integrations     []*Integration
		explicitBindings map[string]string
		wantError        bool
		errorType        string // "NoIntegration", "MultipleIntegrations", "Binding"
		checkBindings    func(t *testing.T, bindings map[string]*ResolvedBinding)
	}{
		{
			name:             "auto-bind single integration",
			workflowRequires: []string{"github"},
			integrations: []*Integration{
				{Name: "github", Type: "github"},
			},
			explicitBindings: nil,
			wantError:        false,
			checkBindings: func(t *testing.T, bindings map[string]*ResolvedBinding) {
				if len(bindings) != 1 {
					t.Errorf("expected 1 binding, got %d", len(bindings))
				}
				if binding, ok := bindings["github"]; ok {
					if binding.Integration.Name != "github" {
						t.Errorf("expected integration name 'github', got %q", binding.Integration.Name)
					}
					if binding.BindingMethod != BindingMethodAuto {
						t.Errorf("expected auto binding, got %q", binding.BindingMethod)
					}
				} else {
					t.Error("expected binding for 'github' not found")
				}
			},
		},
		{
			name:             "error on no integration",
			workflowRequires: []string{"github"},
			integrations:     []*Integration{},
			explicitBindings: nil,
			wantError:        true,
			errorType:        "NoIntegration",
		},
		{
			name:             "error on multiple integrations without explicit binding",
			workflowRequires: []string{"github"},
			integrations: []*Integration{
				{Name: "github", Type: "github"},
				{Name: "work", Type: "github"},
			},
			explicitBindings: nil,
			wantError:        true,
			errorType:        "MultipleIntegrations",
		},
		{
			name:             "explicit binding resolves correctly",
			workflowRequires: []string{"github"},
			integrations: []*Integration{
				{Name: "github", Type: "github"},
				{Name: "work", Type: "github"},
			},
			explicitBindings: map[string]string{"github": "work"},
			wantError:        false,
			checkBindings: func(t *testing.T, bindings map[string]*ResolvedBinding) {
				if binding, ok := bindings["github"]; ok {
					if binding.Integration.Name != "work" {
						t.Errorf("expected integration name 'work', got %q", binding.Integration.Name)
					}
					if binding.BindingMethod != BindingMethodExplicit {
						t.Errorf("expected explicit binding, got %q", binding.BindingMethod)
					}
				} else {
					t.Error("expected binding for 'github' not found")
				}
			},
		},
		{
			name:             "aliased requirement requires explicit binding",
			workflowRequires: []string{"github as source"},
			integrations: []*Integration{
				{Name: "github", Type: "github"},
			},
			explicitBindings: nil,
			wantError:        true,
			errorType:        "Binding",
		},
		{
			name:             "aliased requirement with explicit binding",
			workflowRequires: []string{"github as source", "github as target"},
			integrations: []*Integration{
				{Name: "personal", Type: "github"},
				{Name: "work", Type: "github"},
			},
			explicitBindings: map[string]string{"source": "personal", "target": "work"},
			wantError:        false,
			checkBindings: func(t *testing.T, bindings map[string]*ResolvedBinding) {
				if len(bindings) != 2 {
					t.Errorf("expected 2 bindings, got %d", len(bindings))
				}
				if binding, ok := bindings["source"]; ok {
					if binding.Integration.Name != "personal" {
						t.Errorf("expected integration name 'personal', got %q", binding.Integration.Name)
					}
					if binding.Requirement.Alias != "source" {
						t.Errorf("expected alias 'source', got %q", binding.Requirement.Alias)
					}
				} else {
					t.Error("expected binding for 'source' not found")
				}
				if binding, ok := bindings["target"]; ok {
					if binding.Integration.Name != "work" {
						t.Errorf("expected integration name 'work', got %q", binding.Integration.Name)
					}
				} else {
					t.Error("expected binding for 'target' not found")
				}
			},
		},
		{
			name:             "mixed simple and aliased",
			workflowRequires: []string{"slack", "github as source"},
			integrations: []*Integration{
				{Name: "slack", Type: "slack"},
				{Name: "personal", Type: "github"},
			},
			explicitBindings: map[string]string{"source": "personal"},
			wantError:        false,
			checkBindings: func(t *testing.T, bindings map[string]*ResolvedBinding) {
				if len(bindings) != 2 {
					t.Errorf("expected 2 bindings, got %d", len(bindings))
				}
				// Check slack auto-bind
				if binding, ok := bindings["slack"]; ok {
					if binding.BindingMethod != BindingMethodAuto {
						t.Errorf("expected auto binding for slack, got %q", binding.BindingMethod)
					}
				}
				// Check github explicit bind
				if binding, ok := bindings["source"]; ok {
					if binding.BindingMethod != BindingMethodExplicit {
						t.Errorf("expected explicit binding for source, got %q", binding.BindingMethod)
					}
				}
			},
		},
		{
			name:             "no requirements returns empty bindings",
			workflowRequires: []string{},
			integrations:     []*Integration{},
			explicitBindings: nil,
			wantError:        false,
			checkBindings: func(t *testing.T, bindings map[string]*ResolvedBinding) {
				if len(bindings) != 0 {
					t.Errorf("expected 0 bindings, got %d", len(bindings))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock storage
			storage := &mockStorage{
				integrations: map[string][]*Integration{
					"default": tt.integrations,
				},
			}

			// Create resolver
			resolver := NewBindingResolver(storage)

			// Create workflow with requirements
			wf := &workflow.Definition{
				Name: "test-workflow",
				Requires: &workflow.RequirementsDefinition{
					Integrations: tt.workflowRequires,
				},
			}

			// Resolve bindings
			bindings, err := resolver.ResolveBindings(context.Background(), "default", wf, tt.explicitBindings)

			// Check error expectations
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				// Check error type if specified
				switch tt.errorType {
				case "NoIntegration":
					if _, ok := err.(*NoIntegrationError); !ok {
						t.Errorf("expected NoIntegrationError, got %T: %v", err, err)
					}
				case "MultipleIntegrations":
					if _, ok := err.(*MultipleIntegrationsError); !ok {
						t.Errorf("expected MultipleIntegrationsError, got %T: %v", err, err)
					}
				case "Binding":
					if _, ok := err.(*BindingError); !ok {
						t.Errorf("expected BindingError, got %T: %v", err, err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check bindings if provided
			if tt.checkBindings != nil {
				tt.checkBindings(t, bindings)
			}
		})
	}
}
