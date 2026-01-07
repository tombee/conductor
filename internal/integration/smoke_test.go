package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
)

// =============================================================================
// MOCK-BASED SMOKE TESTS
// =============================================================================
// These tests use mock HTTP servers to verify integration behavior without
// requiring actual API credentials. They should catch basic issues like:
//   - Integration registration problems
//   - Operation declaration issues
//   - Input validation errors
//   - Unknown operation handling
// =============================================================================

// TestAllIntegrationsRegistered verifies that all integrations in BuiltinRegistry
// can be instantiated and have valid operations declared.
// This is a smoke test to catch registration issues early.
func TestAllIntegrationsRegistered(t *testing.T) {
	if len(BuiltinRegistry) == 0 {
		t.Fatal("BuiltinRegistry is empty - no integrations registered")
	}

	for name := range BuiltinRegistry {
		t.Run(name, func(t *testing.T) {
			// Skip integrations that require special transports
			if transportType, special := integrationsRequiringSpecialTransport[name]; special {
				t.Skipf("%s integration requires %s transport, skipping HTTP mock test", name, transportType)
			}

			// Create a mock HTTP server for the integration
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "ok",
				})
			}))
			defer server.Close()

			// Create transport
			httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
				BaseURL: server.URL,
				Timeout: 5 * time.Second,
			})
			if err != nil {
				t.Fatalf("failed to create transport: %v", err)
			}

			// Create provider config with all common additional auth fields
			config := &api.ProviderConfig{
				BaseURL:   server.URL,
				Token:     "test-token",
				Transport: httpTransport,
				AdditionalAuth: map[string]string{
					"email":    "test@example.com",
					"username": "testuser",
					"password": "testpass",
				},
			}

			// Instantiate the integration
			factory := BuiltinRegistry[name]
			provider, err := factory(config)
			if err != nil {
				t.Fatalf("failed to create %s integration: %v", name, err)
			}

			if provider == nil {
				t.Fatalf("%s factory returned nil provider without error", name)
			}

			// Verify provider has a name
			if provider.Name() == "" {
				t.Errorf("%s integration has empty name", name)
			}

			// Verify operations are declared
			if typed, ok := provider.(api.TypedProvider); ok {
				ops := typed.Operations()
				if len(ops) == 0 {
					t.Errorf("%s integration has no operations declared", name)
				}

				// Verify each operation has required metadata
				for _, op := range ops {
					if op.Name == "" {
						t.Errorf("%s integration has operation with empty name", name)
					}
					if op.Description == "" {
						t.Errorf("%s integration operation %q has empty description", name, op.Name)
					}
				}
			}
		})
	}
}

// TestAllIntegrationsHandleUnknownOperation verifies that all integrations
// properly reject unknown operations with a clear error message.
func TestAllIntegrationsHandleUnknownOperation(t *testing.T) {
	for name := range BuiltinRegistry {
		t.Run(name, func(t *testing.T) {
			provider := createTestProvider(t, name)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := provider.Execute(ctx, "nonexistent_operation_xyz", map[string]interface{}{})

			if err == nil {
				t.Fatalf("%s integration should reject unknown operations", name)
			}

			errMsg := strings.ToLower(err.Error())
			if !strings.Contains(errMsg, "unknown") && !strings.Contains(errMsg, "unsupported") {
				t.Errorf("%s integration error for unknown operation should mention 'unknown' or 'unsupported', got: %v", name, err)
			}
		})
	}
}

// TestAllIntegrationsValidateMissingRequiredParams verifies that integrations
// return helpful validation errors when required parameters are missing.
func TestAllIntegrationsValidateMissingRequiredParams(t *testing.T) {
	// Map of integration -> operation -> expected missing param substring
	testCases := map[string]map[string]string{
		"github": {
			"create_issue": "owner",
			"create_pr":    "owner",
			"get_file":     "owner",
		},
		"slack": {
			"post_message": "channel",
			"get_channel":  "channel",
		},
		"notion": {
			"create_page":          "parent_id",
			"get_page":             "page_id",
			"append_blocks":        "page_id",
			"query_database":       "database_id",
			"create_database_item": "database_id",
		},
		"jira": {
			"create_issue": "project",
			"get_issue":    "issue_key",
		},
		"discord": {
			"send_message": "channel_id",
			"get_channel":  "channel_id",
		},
	}

	for name, operations := range testCases {
		t.Run(name, func(t *testing.T) {
			if _, exists := BuiltinRegistry[name]; !exists {
				t.Skipf("integration %s not in registry", name)
			}

			provider := createTestProvider(t, name)

			for opName, expectedParam := range operations {
				t.Run(opName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					// Call with empty inputs to trigger validation error
					_, err := provider.Execute(ctx, opName, map[string]interface{}{})

					if err == nil {
						t.Fatalf("%s.%s should require parameter %q", name, opName, expectedParam)
					}

					errMsg := strings.ToLower(err.Error())
					if !strings.Contains(errMsg, strings.ToLower(expectedParam)) {
						t.Errorf("%s.%s error should mention missing %q, got: %v", name, opName, expectedParam, err)
					}
				})
			}
		})
	}
}

// TestIntegrationRegistryWiring verifies that integrations can be registered
// with the operation registry and executed through it.
func TestIntegrationRegistryWiring(t *testing.T) {
	// Create a mock server that returns success for all requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return appropriate mock response based on path
		if strings.Contains(r.URL.Path, "/pages") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object":       "page",
				"id":           "abc123def456789012345678901234ab",
				"url":          "https://notion.so/test-page",
				"created_time": "2026-01-07T12:00:00.000Z",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}))
	defer server.Close()

	// Create operation registry
	registry, err := operation.NewBuiltinRegistry(&operation.BuiltinConfig{
		WorkflowDir: "/tmp",
	})
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	// Register an integration
	err = registry.RegisterIntegration(operation.IntegrationConfig{
		Name:      "notion",
		Type:      "notion",
		BaseURL:   server.URL,
		AuthType:  "token",
		AuthToken: "test-token",
	})
	if err != nil {
		t.Fatalf("failed to register notion integration: %v", err)
	}

	// Verify integration is accessible
	providers := registry.List()
	found := false
	for _, p := range providers {
		if p == "notion" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("notion integration not found in registry after registration")
	}

	// Execute an operation through the registry
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := registry.Execute(ctx, "notion.create_page", map[string]interface{}{
		"parent_id": "abc123def456789012345678901234ab",
		"title":     "Test Page",
	})

	if err != nil {
		t.Fatalf("registry.Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("registry.Execute returned nil result")
	}

	// Verify response structure
	response, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", result.Response)
	}

	if response["id"] == nil {
		t.Error("response missing 'id' field")
	}
}

// TestAllIntegrationsHaveConsistentNaming verifies that operation names
// follow the snake_case convention.
func TestAllIntegrationsHaveConsistentNaming(t *testing.T) {
	for name := range BuiltinRegistry {
		t.Run(name, func(t *testing.T) {
			provider := createTestProvider(t, name)

			if typed, ok := provider.(api.TypedProvider); ok {
				ops := typed.Operations()

				for _, op := range ops {
					// Check for snake_case (no uppercase, no camelCase)
					if strings.ToLower(op.Name) != op.Name {
						t.Errorf("%s operation %q should be snake_case (lowercase)", name, op.Name)
					}

					// Check for consistent separators (underscores, not hyphens)
					if strings.Contains(op.Name, "-") {
						t.Errorf("%s operation %q should use underscores, not hyphens", name, op.Name)
					}

					// Check operation name is not empty
					if strings.TrimSpace(op.Name) == "" {
						t.Errorf("%s has operation with empty/whitespace name", name)
					}
				}
			}
		})
	}
}

// TestIntegrationOperationsAreDocumented verifies that all operations
// have descriptions and categorization.
func TestIntegrationOperationsAreDocumented(t *testing.T) {
	for name := range BuiltinRegistry {
		t.Run(name, func(t *testing.T) {
			provider := createTestProvider(t, name)

			if typed, ok := provider.(api.TypedProvider); ok {
				ops := typed.Operations()

				for _, op := range ops {
					if op.Description == "" {
						t.Errorf("%s.%s has no description", name, op.Name)
					}

					if op.Category == "" {
						t.Errorf("%s.%s has no category", name, op.Name)
					}

					if len(op.Tags) == 0 {
						t.Errorf("%s.%s has no tags (should have at least 'read' or 'write')", name, op.Name)
					}
				}
			}
		})
	}
}

// integrationsRequiringSpecialTransport lists integrations that need non-HTTP transports.
// These are skipped in smoke tests that use mock HTTP servers.
var integrationsRequiringSpecialTransport = map[string]string{
	"cloudwatch": "aws_sigv4",
}

// =============================================================================
// WORKFLOW INTEGRATION TESTS
// =============================================================================
// These tests verify that integrations work correctly when used in workflows.
// This catches issues where integration code exists but isn't properly wired
// into the workflow executor.
// =============================================================================

// TestWorkflowWithIntegrationSteps verifies that a workflow can parse and execute
// integration steps through the operation registry.
func TestWorkflowWithIntegrationSteps(t *testing.T) {
	// Create a mock server for all integration calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return appropriate responses based on path
		switch {
		case strings.Contains(r.URL.Path, "/pages"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object":       "page",
				"id":           "abc123def456789012345678901234ab",
				"url":          "https://notion.so/test",
				"created_time": "2026-01-07T12:00:00.000Z",
			})
		case strings.Contains(r.URL.Path, "/blocks"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object":  "list",
				"results": []interface{}{},
			})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	// Create operation registry with builtin actions
	registry, err := operation.NewBuiltinRegistry(&operation.BuiltinConfig{
		WorkflowDir: "/tmp",
	})
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	// Register the Notion integration
	err = registry.RegisterIntegration(operation.IntegrationConfig{
		Name:      "notion",
		Type:      "notion",
		BaseURL:   server.URL,
		AuthType:  "token",
		AuthToken: "test-token",
	})
	if err != nil {
		t.Fatalf("failed to register notion integration: %v", err)
	}

	// Execute an integration operation through the registry
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test 1: Create page operation
	t.Run("notion.create_page", func(t *testing.T) {
		result, err := registry.Execute(ctx, "notion.create_page", map[string]interface{}{
			"parent_id": "abc123def456789012345678901234ab",
			"title":     "Test Page",
		})

		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		if response["id"] == nil {
			t.Error("response missing 'id' field")
		}
	})

	// Test 2: Get page operation
	t.Run("notion.get_page", func(t *testing.T) {
		result, err := registry.Execute(ctx, "notion.get_page", map[string]interface{}{
			"page_id": "abc123def456789012345678901234ab",
		})

		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}
	})

	// Test 3: Append blocks operation
	t.Run("notion.append_blocks", func(t *testing.T) {
		result, err := registry.Execute(ctx, "notion.append_blocks", map[string]interface{}{
			"page_id": "abc123def456789012345678901234ab",
			"blocks": []interface{}{
				map[string]interface{}{
					"type": "paragraph",
					"text": "Test content",
				},
			},
		})

		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}
	})
}

// TestMultipleIntegrationsInRegistry verifies that multiple integrations
// can be registered and used in the same registry.
func TestMultipleIntegrationsInRegistry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":   true,
			"id":   "abc123def456789012345678901234ab",
			"name": "test",
		})
	}))
	defer server.Close()

	registry, err := operation.NewBuiltinRegistry(&operation.BuiltinConfig{
		WorkflowDir: "/tmp",
	})
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	// Register multiple integrations
	integrations := []struct {
		name string
		typ  string
	}{
		{"notion", "notion"},
		{"github", "github"},
		{"slack", "slack"},
	}

	for _, intg := range integrations {
		err = registry.RegisterIntegration(operation.IntegrationConfig{
			Name:      intg.name,
			Type:      intg.typ,
			BaseURL:   server.URL,
			AuthType:  "token",
			AuthToken: "test-token",
		})
		if err != nil {
			t.Fatalf("failed to register %s integration: %v", intg.name, err)
		}
	}

	// Verify all integrations are accessible
	providers := registry.List()

	for _, intg := range integrations {
		found := false
		for _, p := range providers {
			if p == intg.name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("integration %s not found in registry", intg.name)
		}
	}
}

// createTestProvider creates a test provider with a mock HTTP server.
func createTestProvider(t *testing.T, name string) operation.Provider {
	t.Helper()

	// Skip integrations that require special transports
	if transportType, special := integrationsRequiringSpecialTransport[name]; special {
		t.Skipf("%s integration requires %s transport, skipping HTTP mock test", name, transportType)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}))
	t.Cleanup(server.Close)

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	config := &api.ProviderConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
		AdditionalAuth: map[string]string{
			"email":    "test@example.com",
			"username": "testuser",
			"password": "testpass",
		},
	}

	factory := BuiltinRegistry[name]
	provider, err := factory(config)
	if err != nil {
		t.Fatalf("failed to create %s integration: %v", name, err)
	}

	return provider
}

// createTestProviderWithSpecialConfig creates a test provider handling special requirements.
// Returns nil and skips if the integration cannot be tested with mock servers.
func createTestProviderWithSpecialConfig(t *testing.T, name string) operation.Provider {
	t.Helper()

	// Skip integrations that require special transports (AWS SigV4, etc.)
	if transportType, special := integrationsRequiringSpecialTransport[name]; special {
		t.Skipf("%s integration requires %s transport, skipping", name, transportType)
		return nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}))
	t.Cleanup(server.Close)

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	config := &api.ProviderConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
		AdditionalAuth: map[string]string{
			"email":    "test@example.com",
			"username": "testuser",
			"password": "testpass",
		},
	}

	factory := BuiltinRegistry[name]
	provider, err := factory(config)
	if err != nil {
		t.Fatalf("failed to create %s integration: %v", name, err)
	}

	return provider
}
