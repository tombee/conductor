package elasticsearch

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
)

// mockTransport is a mock implementation of transport.Transport for testing.
type mockTransport struct {
	lastRequest *transport.Request
	response    *transport.Response
	err         error
	rateLimiter transport.RateLimiter
}

func (m *mockTransport) Execute(ctx context.Context, req *transport.Request) (*transport.Response, error) {
	m.lastRequest = req
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockTransport) Name() string {
	return "mock"
}

func (m *mockTransport) SetRateLimiter(limiter transport.RateLimiter) {
	m.rateLimiter = limiter
}

func TestNewElasticsearchIntegration(t *testing.T) {
	tests := []struct {
		name        string
		config      *api.ProviderConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &api.ProviderConfig{
				Transport: &mockTransport{},
				BaseURL:   "https://elasticsearch.example.com:9200",
				Token:     "test-api-key",
			},
			expectError: false,
		},
		{
			name: "missing transport",
			config: &api.ProviderConfig{
				BaseURL: "https://elasticsearch.example.com:9200",
				Token:   "test-api-key",
			},
			expectError: true,
		},
		{
			name: "missing base URL",
			config: &api.ProviderConfig{
				Transport: &mockTransport{},
				Token:     "test-api-key",
			},
			expectError: true,
		},
		{
			name: "no API key (valid for unsecured cluster)",
			config: &api.ProviderConfig{
				Transport: &mockTransport{},
				BaseURL:   "https://elasticsearch.example.com:9200",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration, err := NewElasticsearchIntegration(tt.config)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && integration == nil {
				t.Error("expected integration but got nil")
			}
		})
	}
}

func TestElasticsearchIntegration_IndexDocument(t *testing.T) {
	tests := []struct {
		name        string
		inputs      map[string]interface{}
		expectError bool
		checkURL    func(t *testing.T, url string)
		checkBody   func(t *testing.T, body []byte)
	}{
		{
			name: "index document with ID",
			inputs: map[string]interface{}{
				"index": "logs",
				"id":    "doc123",
				"document": map[string]interface{}{
					"message": "test log",
					"level":   "info",
				},
			},
			expectError: false,
			checkURL: func(t *testing.T, url string) {
				if !strings.Contains(url, "/logs/_doc/doc123") {
					t.Errorf("expected URL to contain /logs/_doc/doc123, got %s", url)
				}
			},
			checkBody: func(t *testing.T, body []byte) {
				var doc map[string]interface{}
				if err := json.Unmarshal(body, &doc); err != nil {
					t.Fatalf("failed to unmarshal body: %v", err)
				}
				if doc["message"] != "test log" {
					t.Errorf("expected message 'test log', got %v", doc["message"])
				}
			},
		},
		{
			name: "index document without ID (auto-generate)",
			inputs: map[string]interface{}{
				"index": "logs",
				"document": map[string]interface{}{
					"message": "test log",
				},
			},
			expectError: false,
			checkURL: func(t *testing.T, url string) {
				if !strings.Contains(url, "/logs/_doc") {
					t.Errorf("expected URL to contain /logs/_doc, got %s", url)
				}
				if strings.Contains(url, "/logs/_doc/") && len(strings.Split(url, "/logs/_doc/")) > 1 && strings.Split(url, "/logs/_doc/")[1] != "" {
					t.Errorf("expected no document ID in URL, got %s", url)
				}
			},
		},
		{
			name: "index with refresh policy",
			inputs: map[string]interface{}{
				"index":   "logs",
				"id":      "doc123",
				"refresh": "wait_for",
				"document": map[string]interface{}{
					"message": "test log",
				},
			},
			expectError: false,
			checkURL: func(t *testing.T, url string) {
				if !strings.Contains(url, "refresh=wait_for") {
					t.Errorf("expected URL to contain refresh=wait_for, got %s", url)
				}
			},
		},
		{
			name: "index with pipeline",
			inputs: map[string]interface{}{
				"index":    "logs",
				"pipeline": "my-pipeline",
				"document": map[string]interface{}{
					"message": "test log",
				},
			},
			expectError: false,
			checkURL: func(t *testing.T, url string) {
				if !strings.Contains(url, "pipeline=my-pipeline") {
					t.Errorf("expected URL to contain pipeline=my-pipeline, got %s", url)
				}
			},
		},
		{
			name: "missing document",
			inputs: map[string]interface{}{
				"index": "logs",
			},
			expectError: true,
		},
		{
			name: "missing index",
			inputs: map[string]interface{}{
				"document": map[string]interface{}{
					"message": "test log",
				},
			},
			expectError: true,
		},
		{
			name: "invalid refresh policy",
			inputs: map[string]interface{}{
				"index":   "logs",
				"refresh": "invalid",
				"document": map[string]interface{}{
					"message": "test log",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResp := &transport.Response{
				StatusCode: 201,
				Body: []byte(`{
					"_index": "logs",
					"_id": "doc123",
					"_version": 1,
					"result": "created"
				}`),
			}

			mock := &mockTransport{response: mockResp}
			config := &api.ProviderConfig{
				Transport: mock,
				BaseURL:   "https://elasticsearch.example.com:9200",
				Token:     "test-api-key",
			}

			integration, err := NewElasticsearchIntegration(config)
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			result, err := integration.Execute(context.Background(), "index", tt.inputs)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && result != nil {
				if tt.checkURL != nil && mock.lastRequest != nil {
					tt.checkURL(t, mock.lastRequest.URL)
				}
				if tt.checkBody != nil && mock.lastRequest != nil {
					tt.checkBody(t, mock.lastRequest.Body)
				}

				// Verify response structure
				if result.Response == nil {
					t.Error("expected response but got nil")
				}
			}
		})
	}
}

func TestElasticsearchIntegration_BulkIndex(t *testing.T) {
	tests := []struct {
		name        string
		inputs      map[string]interface{}
		expectError bool
		checkBody   func(t *testing.T, body []byte)
	}{
		{
			name: "bulk index with default index",
			inputs: map[string]interface{}{
				"index": "logs",
				"documents": []interface{}{
					map[string]interface{}{
						"message": "log 1",
					},
					map[string]interface{}{
						"message": "log 2",
					},
				},
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				lines := strings.Split(string(body), "\n")
				if len(lines) < 4 {
					t.Errorf("expected at least 4 lines in NDJSON, got %d", len(lines))
				}

				// Check action line
				var action map[string]interface{}
				if err := json.Unmarshal([]byte(lines[0]), &action); err != nil {
					t.Fatalf("failed to unmarshal action: %v", err)
				}
				if _, ok := action["index"]; !ok {
					t.Error("expected 'index' action")
				}
			},
		},
		{
			name: "bulk index with per-document index override",
			inputs: map[string]interface{}{
				"documents": []interface{}{
					map[string]interface{}{
						"_index":  "logs-1",
						"_id":     "doc1",
						"message": "log 1",
					},
					map[string]interface{}{
						"_index":  "logs-2",
						"_id":     "doc2",
						"message": "log 2",
					},
				},
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				lines := strings.Split(string(body), "\n")

				// Check first action includes index and ID
				var action1 map[string]interface{}
				if err := json.Unmarshal([]byte(lines[0]), &action1); err != nil {
					t.Fatalf("failed to unmarshal action: %v", err)
				}
				indexData := action1["index"].(map[string]interface{})
				if indexData["_index"] != "logs-1" {
					t.Errorf("expected _index 'logs-1', got %v", indexData["_index"])
				}
				if indexData["_id"] != "doc1" {
					t.Errorf("expected _id 'doc1', got %v", indexData["_id"])
				}
			},
		},
		{
			name: "bulk index with refresh policy",
			inputs: map[string]interface{}{
				"refresh": "true",
				"documents": []interface{}{
					map[string]interface{}{
						"message": "log 1",
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing documents",
			inputs: map[string]interface{}{
				"index": "logs",
			},
			expectError: true,
		},
		{
			name: "empty documents array",
			inputs: map[string]interface{}{
				"index":     "logs",
				"documents": []interface{}{},
			},
			expectError: true,
		},
		{
			name: "invalid document type",
			inputs: map[string]interface{}{
				"documents": []interface{}{
					"not an object",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResp := &transport.Response{
				StatusCode: 200,
				Body: []byte(`{
					"took": 30,
					"errors": false,
					"items": [
						{
							"index": {
								"_index": "logs",
								"_id": "doc1",
								"_version": 1,
								"result": "created",
								"status": 201
							}
						}
					]
				}`),
			}

			mock := &mockTransport{response: mockResp}
			config := &api.ProviderConfig{
				Transport: mock,
				BaseURL:   "https://elasticsearch.example.com:9200",
				Token:     "test-api-key",
			}

			integration, err := NewElasticsearchIntegration(config)
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			result, err := integration.Execute(context.Background(), "bulk", tt.inputs)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && result != nil {
				if tt.checkBody != nil && mock.lastRequest != nil {
					tt.checkBody(t, mock.lastRequest.Body)
				}

				// Verify response structure
				if result.Response == nil {
					t.Error("expected response but got nil")
				}

				// Verify Content-Type header for bulk
				if mock.lastRequest != nil {
					contentType := mock.lastRequest.Headers["Content-Type"]
					if contentType != "application/x-ndjson" {
						t.Errorf("expected Content-Type 'application/x-ndjson', got %s", contentType)
					}
				}
			}
		})
	}
}

func TestElasticsearchIntegration_Operations(t *testing.T) {
	mock := &mockTransport{}
	config := &api.ProviderConfig{
		Transport: mock,
		BaseURL:   "https://elasticsearch.example.com:9200",
	}

	integration, err := NewElasticsearchIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	// Cast to concrete type to access Operations method
	esIntegration, ok := integration.(*ElasticsearchIntegration)
	if !ok {
		t.Fatal("failed to cast to ElasticsearchIntegration")
	}

	ops := esIntegration.Operations()
	if len(ops) != 2 {
		t.Errorf("expected 2 operations, got %d", len(ops))
	}

	// Verify operation names
	opNames := make(map[string]bool)
	for _, op := range ops {
		opNames[op.Name] = true
	}

	if !opNames["index"] {
		t.Error("expected 'index' operation")
	}
	if !opNames["bulk"] {
		t.Error("expected 'bulk' operation")
	}
}

func TestElasticsearchIntegration_UnknownOperation(t *testing.T) {
	mock := &mockTransport{}
	config := &api.ProviderConfig{
		Transport: mock,
		BaseURL:   "https://elasticsearch.example.com:9200",
	}

	integration, err := NewElasticsearchIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	_, err = integration.Execute(context.Background(), "unknown", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for unknown operation")
	}
}
