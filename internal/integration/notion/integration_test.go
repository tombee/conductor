package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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
		"create_page":           true,
		"get_page":              true,
		"update_page":           true,
		"delete_page":           true,
		"upsert_page":           true,
		"get_blocks":            true,
		"append_blocks":         true,
		"replace_content":       true,
		"query_database":        true,
		"create_database_item":  true,
		"update_database_item":  true,
		"delete_database_item":  true,
		"list_databases":        true,
		"search":                true,
		"get_comments":          true,
		"add_comment":           true,
		"get_database_schema":   true,
		"update_database_schema": true,
		"batch_create_pages":    true,
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

func TestNotionIntegration_UpsertPageWithBlocks(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		// First call: get block children (to check for existing pages)
		if r.Method == "GET" && r.URL.Path == "/blocks/abc123def456789012345678901234ab/children" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object": "list",
				"results": []interface{}{},
			})
			return
		}

		// Second call: create page
		if r.Method == "POST" && r.URL.Path == "/pages" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object": "page",
				"id":     "newpage123def456789012345678901234",
				"url":    "https://notion.so/new-page",
				"created_time": "2026-01-06T12:00:00.000Z",
			})
			return
		}

		// Third call: append blocks (should NOT happen if blocks handling crashes)
		if r.Method == "PATCH" && r.URL.Path == "/blocks/newpage123def456789012345678901234/children" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object": "list",
				"results": []interface{}{
					map[string]interface{}{"object": "block", "id": "block1"},
					map[string]interface{}{"object": "block", "id": "block2"},
				},
			})
			return
		}

		// Unknown endpoint
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object":  "error",
			"status":  404,
			"message": "Unknown endpoint: " + r.Method + " " + r.URL.Path,
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

	result, err := integration.Execute(context.Background(), "upsert_page", map[string]interface{}{
		"parent_id": "abc123def456789012345678901234ab",
		"title":     "Test Page",
		"blocks": []interface{}{
			map[string]interface{}{
				"type": "heading_1",
				"text": "Hello World",
			},
			map[string]interface{}{
				"type": "paragraph",
				"text": "Test paragraph content",
			},
		},
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

	// Should have page ID from the create response
	if data["id"] == nil || data["id"] == "" {
		t.Error("expected page id in response")
	}

	// With blocks, we expect 3 calls: get children, create page, append blocks
	if callCount < 3 {
		t.Errorf("expected at least 3 API calls (got %d), blocks may not have been appended", callCount)
	}
}

func TestNotionIntegration_DeletePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH method, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/pages/") {
			t.Errorf("expected /pages/ path, got %s", r.URL.Path)
		}

		// Verify request body contains archived: true
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["archived"] != true {
			t.Errorf("expected archived=true in request body")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object":   "page",
			"id":       "abc123def456789012345678901234ab",
			"archived": true,
		})
	}))
	defer server.Close()

	httpTransport, _ := transport.NewHTTPTransport(&transport.HTTPTransportConfig{BaseURL: server.URL})
	config := &api.ProviderConfig{BaseURL: server.URL, Token: "test-token", Transport: httpTransport}
	integration, _ := NewNotionIntegration(config)

	result, err := integration.Execute(context.Background(), "delete_page", map[string]interface{}{
		"page_id": "abc123def456789012345678901234ab",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := result.Response.(map[string]interface{})
	if data["archived"] != true {
		t.Errorf("expected archived=true in response")
	}
}

func TestNotionIntegration_DeleteDatabaseItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object":   "page",
			"id":       "abc123def456789012345678901234ab",
			"archived": true,
		})
	}))
	defer server.Close()

	httpTransport, _ := transport.NewHTTPTransport(&transport.HTTPTransportConfig{BaseURL: server.URL})
	config := &api.ProviderConfig{BaseURL: server.URL, Token: "test-token", Transport: httpTransport}
	integration, _ := NewNotionIntegration(config)

	result, err := integration.Execute(context.Background(), "delete_database_item", map[string]interface{}{
		"item_id": "abc123def456789012345678901234ab",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := result.Response.(map[string]interface{})
	if data["archived"] != true {
		t.Errorf("expected archived=true in response")
	}
}

func TestNotionIntegration_ListDatabases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/search" {
			t.Errorf("expected POST /search, got %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"results": []map[string]interface{}{
				{
					"object":           "database",
					"id":               "db123def456789012345678901234ab",
					"created_time":     "2026-01-01T00:00:00.000Z",
					"last_edited_time": "2026-01-02T00:00:00.000Z",
					"title": []map[string]interface{}{
						{"plain_text": "Test Database"},
					},
				},
			},
			"has_more": false,
		})
	}))
	defer server.Close()

	httpTransport, _ := transport.NewHTTPTransport(&transport.HTTPTransportConfig{BaseURL: server.URL})
	config := &api.ProviderConfig{BaseURL: server.URL, Token: "test-token", Transport: httpTransport}
	integration, _ := NewNotionIntegration(config)

	result, err := integration.Execute(context.Background(), "list_databases", map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := result.Response.(map[string]interface{})
	databases, ok := data["databases"].([]map[string]interface{})
	if !ok || len(databases) != 1 {
		t.Errorf("expected 1 database, got %v", data["databases"])
	}
	if databases[0]["title"] != "Test Database" {
		t.Errorf("expected title 'Test Database', got %v", databases[0]["title"])
	}
}

func TestNotionIntegration_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/search" {
			t.Errorf("expected POST /search, got %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"results": []map[string]interface{}{
				{
					"object":           "page",
					"id":               "page123def456789012345678901234a",
					"url":              "https://notion.so/page",
					"created_time":     "2026-01-01T00:00:00.000Z",
					"last_edited_time": "2026-01-02T00:00:00.000Z",
				},
			},
			"has_more":    true,
			"next_cursor": "cursor123",
		})
	}))
	defer server.Close()

	httpTransport, _ := transport.NewHTTPTransport(&transport.HTTPTransportConfig{BaseURL: server.URL})
	config := &api.ProviderConfig{BaseURL: server.URL, Token: "test-token", Transport: httpTransport}
	integration, _ := NewNotionIntegration(config)

	result, err := integration.Execute(context.Background(), "search", map[string]interface{}{
		"query": "test query",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := result.Response.(map[string]interface{})
	if data["has_more"] != true {
		t.Errorf("expected has_more=true")
	}
	if data["next_cursor"] != "cursor123" {
		t.Errorf("expected next_cursor='cursor123'")
	}
}

func TestNotionIntegration_CheckedFieldValidation(t *testing.T) {
	// Test that 'checked' field is rejected on non-to_do block types
	httpTransport, _ := transport.NewHTTPTransport(&transport.HTTPTransportConfig{BaseURL: "https://api.notion.com/v1"})
	config := &api.ProviderConfig{BaseURL: "https://api.notion.com/v1", Token: "test-token", Transport: httpTransport}
	integration, _ := NewNotionIntegration(config)

	_, err := integration.Execute(context.Background(), "append_blocks", map[string]interface{}{
		"page_id": "abc123def456789012345678901234ab",
		"blocks": []interface{}{
			map[string]interface{}{
				"type":    "paragraph",
				"text":    "Test",
				"checked": true, // Invalid - checked only valid for to_do
			},
		},
	})

	if err == nil {
		t.Fatal("expected error for checked field on paragraph block")
	}
	if !contains(err.Error(), "checked") || !contains(err.Error(), "to_do") {
		t.Errorf("expected error about checked field, got: %v", err)
	}
}

func TestNotionIntegration_IDNormalization(t *testing.T) {
	// Test that IDs with hyphens are accepted and normalized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "page",
			"id":     "abc123de-f456-7890-1234-5678901234ab",
			"url":    "https://notion.so/page",
		})
	}))
	defer server.Close()

	httpTransport, _ := transport.NewHTTPTransport(&transport.HTTPTransportConfig{BaseURL: server.URL})
	config := &api.ProviderConfig{BaseURL: server.URL, Token: "test-token", Transport: httpTransport}
	integration, _ := NewNotionIntegration(config)

	// UUID with hyphens should be accepted
	result, err := integration.Execute(context.Background(), "get_page", map[string]interface{}{
		"page_id": "abc123de-f456-7890-1234-5678901234ab",
	})

	if err != nil {
		t.Fatalf("unexpected error for hyphenated ID: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

func TestNotionIntegration_AdditionalErrorCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errMsg     string
	}{
		{"400 bad request", 400, "400"},
		{"500 internal server error", 500, "500"},
		{"503 service unavailable", 503, "503"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"object":  "error",
					"status":  tt.statusCode,
					"code":    "error",
					"message": "Error",
				})
			}))
			defer server.Close()

			httpTransport, _ := transport.NewHTTPTransport(&transport.HTTPTransportConfig{BaseURL: server.URL})
			config := &api.ProviderConfig{BaseURL: server.URL, Token: "test-token", Transport: httpTransport}
			integration, _ := NewNotionIntegration(config)

			_, err := integration.Execute(context.Background(), "get_page", map[string]interface{}{
				"page_id": "abc123def456789012345678901234ab",
			})

			if err == nil {
				t.Fatal("expected error")
			}
			if !contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error to contain '%s', got '%s'", tt.errMsg, err.Error())
			}
		})
	}
}

func TestNotionIntegration_OperationSchema(t *testing.T) {
	httpTransport, _ := transport.NewHTTPTransport(&transport.HTTPTransportConfig{BaseURL: "https://api.notion.com/v1"})
	config := &api.ProviderConfig{BaseURL: "https://api.notion.com/v1", Token: "test-token", Transport: httpTransport}
	integration, _ := NewNotionIntegration(config)
	notionIntegration := integration.(*NotionIntegration)

	// Test that all operations have schemas
	ops := notionIntegration.Operations()
	for _, op := range ops {
		schema := notionIntegration.OperationSchema(op.Name)
		if schema == nil {
			t.Errorf("missing schema for operation: %s", op.Name)
			continue
		}
		if schema.Description == "" {
			t.Errorf("missing description in schema for operation: %s", op.Name)
		}
	}

	// Test specific schema content
	createPageSchema := notionIntegration.OperationSchema("create_page")
	if createPageSchema == nil {
		t.Fatal("missing create_page schema")
	}

	// Should have parent_id and title as required
	hasParentID := false
	hasTitle := false
	for _, param := range createPageSchema.Parameters {
		if param.Name == "parent_id" && param.Required {
			hasParentID = true
		}
		if param.Name == "title" && param.Required {
			hasTitle = true
		}
	}
	if !hasParentID {
		t.Error("create_page schema missing required parent_id parameter")
	}
	if !hasTitle {
		t.Error("create_page schema missing required title parameter")
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

// =============================================================================
// REAL API TESTS
// =============================================================================
// These tests run against the real Notion API when credentials are provided.
// They verify actual API behavior that mocks cannot capture.
//
// Run with:
//   NOTION_TOKEN=secret_xxx NOTION_TEST_PAGE_ID=xxx go test ./internal/integration/notion/... -run "TestRealAPI" -v
//
// Environment variables:
//   NOTION_TOKEN       - Notion integration token (required)
//   NOTION_TEST_PAGE_ID - ID of a page the integration has access to (required)
// =============================================================================

// TestRealAPI_NotionOperations comprehensively tests all Notion operations
// against the real API.
func TestRealAPI_NotionOperations(t *testing.T) {
	token := os.Getenv("NOTION_TOKEN")
	pageID := os.Getenv("NOTION_TEST_PAGE_ID")

	if token == "" {
		t.Skip("NOTION_TOKEN not set, skipping real API test")
	}
	if pageID == "" {
		t.Skip("NOTION_TEST_PAGE_ID not set, skipping real API test")
	}

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://api.notion.com/v1",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	config := &api.ProviderConfig{
		BaseURL:   "https://api.notion.com/v1",
		Token:     token,
		Transport: httpTransport,
	}

	integration, err := NewNotionIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var createdPageID string

	// Test: get_page
	t.Run("get_page", func(t *testing.T) {
		result, err := integration.Execute(ctx, "get_page", map[string]interface{}{
			"page_id": pageID,
		})
		if err != nil {
			t.Fatalf("get_page failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		if response["id"] == nil {
			t.Error("response missing 'id'")
		}
		if response["url"] == nil {
			t.Error("response missing 'url'")
		}
		t.Logf("get_page: Retrieved page %v", response["id"])
	})

	// Test: get_blocks
	t.Run("get_blocks", func(t *testing.T) {
		result, err := integration.Execute(ctx, "get_blocks", map[string]interface{}{
			"page_id": pageID,
		})
		if err != nil {
			t.Fatalf("get_blocks failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		if response["block_count"] == nil {
			t.Error("response missing 'block_count'")
		}
		t.Logf("get_blocks: Retrieved %v blocks", response["block_count"])
	})

	// Test: create_page
	t.Run("create_page", func(t *testing.T) {
		testTitle := "API Test - " + time.Now().Format("15:04:05")

		result, err := integration.Execute(ctx, "create_page", map[string]interface{}{
			"parent_id": pageID,
			"title":     testTitle,
		})
		if err != nil {
			t.Fatalf("create_page failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		id, _ := response["id"].(string)
		if id == "" {
			t.Fatal("create_page did not return page id")
		}

		createdPageID = id
		t.Logf("create_page: Created page %s", id)
	})

	// Test: append_blocks
	t.Run("append_blocks", func(t *testing.T) {
		if createdPageID == "" {
			t.Skip("no page created")
		}

		result, err := integration.Execute(ctx, "append_blocks", map[string]interface{}{
			"page_id": createdPageID,
			"blocks": []interface{}{
				map[string]interface{}{"type": "heading_1", "text": "Test Heading"},
				map[string]interface{}{"type": "paragraph", "text": "Test paragraph content."},
				map[string]interface{}{"type": "bulleted_list_item", "text": "Bullet 1"},
				map[string]interface{}{"type": "code", "text": "console.log('test');", "language": "javascript"},
				map[string]interface{}{"type": "divider"},
				map[string]interface{}{"type": "quote", "text": "A quote"},
			},
		})
		if err != nil {
			t.Fatalf("append_blocks failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		t.Logf("append_blocks: Added %v blocks", response["blocks_added"])
	})

	// Test: upsert_page (create new)
	t.Run("upsert_page_create", func(t *testing.T) {
		testTitle := "Upsert Test - " + time.Now().Format("15:04:05")

		result, err := integration.Execute(ctx, "upsert_page", map[string]interface{}{
			"parent_id": pageID,
			"title":     testTitle,
			"blocks": []interface{}{
				map[string]interface{}{"type": "paragraph", "text": "Created via upsert"},
			},
		})
		if err != nil {
			t.Fatalf("upsert_page failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		isNew, _ := response["is_new"].(bool)
		if !isNew {
			t.Error("expected is_new=true for new page")
		}
		t.Logf("upsert_page: Created new page %v", response["id"])
	})

	// Test: replace_content
	t.Run("replace_content", func(t *testing.T) {
		if createdPageID == "" {
			t.Skip("no page created")
		}

		result, err := integration.Execute(ctx, "replace_content", map[string]interface{}{
			"page_id": createdPageID,
			"blocks": []interface{}{
				map[string]interface{}{"type": "heading_1", "text": "Replaced"},
				map[string]interface{}{"type": "paragraph", "text": "Content was replaced."},
			},
		})
		if err != nil {
			t.Fatalf("replace_content failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		t.Logf("replace_content: Replaced with %v blocks", response["blocks_added"])
	})

	// Test: verify replace worked
	t.Run("verify_replace", func(t *testing.T) {
		if createdPageID == "" {
			t.Skip("no page created")
		}

		result, err := integration.Execute(ctx, "get_blocks", map[string]interface{}{
			"page_id": createdPageID,
		})
		if err != nil {
			t.Fatalf("get_blocks failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		content, _ := response["content"].(string)
		if !strings.Contains(content, "Replaced") {
			t.Errorf("expected 'Replaced' in content, got: %s", content)
		}
	})

	// Test: search
	t.Run("search", func(t *testing.T) {
		result, err := integration.Execute(ctx, "search", map[string]interface{}{
			"query":     "Test",
			"page_size": 5,
		})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		results, _ := response["results"].([]map[string]interface{})
		t.Logf("search: Found %d results for 'Test'", len(results))

		// Verify response structure
		if _, ok := response["has_more"]; !ok {
			t.Error("expected has_more field in response")
		}
	})

	// Test: search with filter
	t.Run("search_with_filter", func(t *testing.T) {
		result, err := integration.Execute(ctx, "search", map[string]interface{}{
			"filter": map[string]interface{}{
				"property": "object",
				"value":    "page",
			},
			"page_size": 5,
		})
		if err != nil {
			t.Fatalf("search with filter failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		t.Logf("search_with_filter: Found results with page filter")
		_ = response // verify we got a valid response
	})

	// Test: list_databases
	t.Run("list_databases", func(t *testing.T) {
		result, err := integration.Execute(ctx, "list_databases", map[string]interface{}{})
		if err != nil {
			t.Fatalf("list_databases failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		databases, _ := response["databases"].([]map[string]interface{})
		t.Logf("list_databases: Found %d accessible databases", len(databases))

		// Log database titles if any
		for i, db := range databases {
			if i >= 3 {
				break // Only log first 3
			}
			title, _ := db["title"].(string)
			id, _ := db["id"].(string)
			t.Logf("  - %s (ID: %s...)", title, id[:8])
		}
	})

	// Test: delete_page (cleanup - run last)
	t.Run("delete_page", func(t *testing.T) {
		if createdPageID == "" {
			t.Skip("no page created to delete")
		}

		result, err := integration.Execute(ctx, "delete_page", map[string]interface{}{
			"page_id": createdPageID,
		})
		if err != nil {
			t.Fatalf("delete_page failed: %v", err)
		}

		response, ok := result.Response.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected response type: %T", result.Response)
		}

		archived, _ := response["archived"].(bool)
		if !archived {
			t.Error("expected archived=true after delete")
		}

		t.Logf("delete_page: Archived page %s", createdPageID)
	})

	// Test: error handling - invalid ID
	t.Run("error_invalid_id", func(t *testing.T) {
		_, err := integration.Execute(ctx, "get_page", map[string]interface{}{
			"page_id": "invalid",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "32-character") {
			t.Errorf("expected validation error, got: %v", err)
		}
	})

	// Test: error handling - missing param
	t.Run("error_missing_param", func(t *testing.T) {
		_, err := integration.Execute(ctx, "create_page", map[string]interface{}{
			"parent_id": pageID,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "title") {
			t.Errorf("expected title error, got: %v", err)
		}
	})
}
