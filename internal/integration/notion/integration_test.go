package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
)

func TestNewNotionIntegration(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		expectErr bool
	}{
		{
			name:      "valid config with custom base URL",
			baseURL:   "https://api.notion.com/v1",
			expectErr: false,
		},
		{
			name:      "valid config with default base URL",
			baseURL:   "",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
				BaseURL: "https://api.notion.com/v1",
			})
			if err != nil {
				t.Fatalf("failed to create transport: %v", err)
			}

			config := &api.ProviderConfig{
				BaseURL:   tt.baseURL,
				Token:     "test-token",
				Transport: httpTransport,
			}
			integration, err := NewNotionIntegration(config)

			if tt.expectErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.expectErr && integration == nil {
				t.Fatal("expected integration, got nil")
			}
		})
	}
}

func TestNotionIntegration_Operations(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://api.notion.com/v1",
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	config := &api.ProviderConfig{
		BaseURL:   "https://api.notion.com/v1",
		Token:     "test-token",
		Transport: httpTransport,
	}

	integration, err := NewNotionIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	notionIntegration := integration.(*NotionIntegration)
	ops := notionIntegration.Operations()

	expectedOps := map[string]bool{
		"create_page":          true,
		"get_page":             true,
		"update_page":          true,
		"upsert_page":          true,
		"append_blocks":        true,
		"query_database":       true,
		"create_database_item": true,
		"update_database_item": true,
	}

	if len(ops) != len(expectedOps) {
		t.Fatalf("expected %d operations, got %d", len(expectedOps), len(ops))
	}

	for _, op := range ops {
		if !expectedOps[op.Name] {
			t.Errorf("unexpected operation: %s", op.Name)
		}
	}
}

func TestNotionIntegration_CreatePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Notion-Version") != NotionAPIVersion {
			t.Errorf("expected Notion-Version header %s, got %s", NotionAPIVersion, r.Header.Get("Notion-Version"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization header 'Bearer test-token', got %s", r.Header.Get("Authorization"))
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "page",
			"id":     "abc123def456789012345678901234ab",
			"url":    "https://notion.so/page-abc123",
			"created_time": "2026-01-03T12:00:00.000Z",
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	config := &api.ProviderConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	integration, err := NewNotionIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	result, err := integration.Execute(context.Background(), "create_page", map[string]interface{}{
		"parent_id": "abc123def456789012345678901234ab",
		"title":     "Test Page",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	data, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatal("expected response to be map[string]interface{}")
	}

	if data["id"] != "abc123def456789012345678901234ab" {
		t.Errorf("expected id 'abc123def456789012345678901234ab', got %v", data["id"])
	}
}

func TestNotionIntegration_AppendBlocks(t *testing.T) {
	tests := []struct {
		name      string
		inputs    map[string]interface{}
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid blocks",
			inputs: map[string]interface{}{
				"page_id": "abc123def456789012345678901234ab",
				"blocks": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"text": "Hello world",
					},
					map[string]interface{}{
						"type": "heading_1",
						"text": "Main Title",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "too many blocks",
			inputs: map[string]interface{}{
				"page_id": "abc123def456789012345678901234ab",
				"blocks":  make([]interface{}, 101),
			},
			expectErr: true,
			errMsg:    "cannot append more than 100 blocks",
		},
		{
			name: "unsupported block type",
			inputs: map[string]interface{}{
				"page_id": "abc123def456789012345678901234ab",
				"blocks": []interface{}{
					map[string]interface{}{
						"type": "unsupported_type",
						"text": "Test",
					},
				},
			},
			expectErr: true,
			errMsg:    "unsupported block type",
		},
		{
			name: "paragraph text too long",
			inputs: map[string]interface{}{
				"page_id": "abc123def456789012345678901234ab",
				"blocks": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"text": string(make([]byte, 2001)),
					},
				},
			},
			expectErr: true,
			errMsg:    "exceeds 2000 character limit",
		},
		{
			name: "heading text too long",
			inputs: map[string]interface{}{
				"page_id": "abc123def456789012345678901234ab",
				"blocks": []interface{}{
					map[string]interface{}{
						"type": "heading_1",
						"text": string(make([]byte, 201)),
					},
				},
			},
			expectErr: true,
			errMsg:    "exceeds 200 character limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"object": "list",
					"results": []map[string]interface{}{
						{"object": "block", "id": "block1"},
					},
				})
			}))
			defer server.Close()

			httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
				BaseURL: server.URL,
			})
			if err != nil {
				t.Fatalf("failed to create transport: %v", err)
			}

			config := &api.ProviderConfig{
				BaseURL:   server.URL,
				Token:     "test-token",
				Transport: httpTransport,
			}

			integration, err := NewNotionIntegration(config)
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			_, err = integration.Execute(context.Background(), "append_blocks", tt.inputs)

			if tt.expectErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error to contain '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestNotionIntegration_QueryDatabase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object":  "list",
			"results": []interface{}{},
			"has_more": false,
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	config := &api.ProviderConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	integration, err := NewNotionIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	result, err := integration.Execute(context.Background(), "query_database", map[string]interface{}{
		"database_id": "abc123def456789012345678901234ab",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	data, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatal("expected response to be map[string]interface{}")
	}

	if data["has_more"] != false {
		t.Errorf("expected has_more false, got %v", data["has_more"])
	}
}

func TestNotionIntegration_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   map[string]interface{}
		expectedErrMsg string
	}{
		{
			name:       "401 unauthorized",
			statusCode: 401,
			responseBody: map[string]interface{}{
				"object":  "error",
				"status":  401,
				"code":    "unauthorized",
				"message": "API token is invalid",
			},
			expectedErrMsg: "401",
		},
		{
			name:       "403 forbidden",
			statusCode: 403,
			responseBody: map[string]interface{}{
				"object":  "error",
				"status":  403,
				"code":    "restricted_resource",
				"message": "Cannot access resource",
			},
			expectedErrMsg: "403",
		},
		{
			name:       "404 not found",
			statusCode: 404,
			responseBody: map[string]interface{}{
				"object":  "error",
				"status":  404,
				"code":    "object_not_found",
				"message": "Page not found",
			},
			expectedErrMsg: "404",
		},
		{
			name:       "429 rate limited",
			statusCode: 429,
			responseBody: map[string]interface{}{
				"object":  "error",
				"status":  429,
				"code":    "rate_limited",
				"message": "Rate limited",
			},
			expectedErrMsg: "429",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.responseBody)
			}))
			defer server.Close()

			httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
				BaseURL: server.URL,
			})
			if err != nil {
				t.Fatalf("failed to create transport: %v", err)
			}

			config := &api.ProviderConfig{
				BaseURL:   server.URL,
				Token:     "test-token",
				Transport: httpTransport,
			}

			integration, err := NewNotionIntegration(config)
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			_, err = integration.Execute(context.Background(), "get_page", map[string]interface{}{
				"page_id": "abc123def456789012345678901234ab",
			})

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !contains(err.Error(), tt.expectedErrMsg) {
				t.Errorf("expected error to contain '%s', got '%s'", tt.expectedErrMsg, err.Error())
			}
		})
	}
}

func TestNotionIntegration_ValidationErrors(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://api.notion.com/v1",
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	config := &api.ProviderConfig{
		BaseURL:   "https://api.notion.com/v1",
		Token:     "test-token",
		Transport: httpTransport,
	}

	integration, err := NewNotionIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name      string
		operation string
		inputs    map[string]interface{}
		errMsg    string
	}{
		{
			name:      "create_page missing parent_id",
			operation: "create_page",
			inputs: map[string]interface{}{
				"title": "Test",
			},
			errMsg: "parent_id",
		},
		{
			name:      "create_page missing title",
			operation: "create_page",
			inputs: map[string]interface{}{
				"parent_id": "abc123def456789012345678901234ab",
			},
			errMsg: "title",
		},
		{
			name:      "create_page invalid parent_id",
			operation: "create_page",
			inputs: map[string]interface{}{
				"parent_id": "invalid",
				"title":     "Test",
			},
			errMsg: "32-character Notion ID",
		},
		{
			name:      "append_blocks missing page_id",
			operation: "append_blocks",
			inputs: map[string]interface{}{
				"blocks": []interface{}{},
			},
			errMsg: "page_id",
		},
		{
			name:      "query_database missing database_id",
			operation: "query_database",
			inputs:    map[string]interface{}{},
			errMsg:    "database_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := integration.Execute(context.Background(), tt.operation, tt.inputs)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error to contain '%s', got '%s'", tt.errMsg, err.Error())
			}
		})
	}
}

func TestNotionIntegration_UnknownOperation(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://api.notion.com/v1",
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	config := &api.ProviderConfig{
		BaseURL:   "https://api.notion.com/v1",
		Token:     "test-token",
		Transport: httpTransport,
	}

	integration, err := NewNotionIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	_, err = integration.Execute(context.Background(), "unknown_operation", map[string]interface{}{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !contains(err.Error(), "unknown operation") {
		t.Errorf("expected 'unknown operation' error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
