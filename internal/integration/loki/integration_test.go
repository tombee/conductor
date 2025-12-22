package loki

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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

func TestNewLokiIntegration(t *testing.T) {
	tests := []struct {
		name        string
		config      *api.ProviderConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &api.ProviderConfig{
				Transport: &mockTransport{},
				BaseURL:   "https://loki.example.com:3100",
			},
			expectError: false,
		},
		{
			name: "missing transport",
			config: &api.ProviderConfig{
				BaseURL: "https://loki.example.com:3100",
			},
			expectError: true,
		},
		{
			name: "missing base URL",
			config: &api.ProviderConfig{
				Transport: &mockTransport{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration, err := NewLokiIntegration(tt.config)
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

func TestLokiIntegration_PushSingleLog(t *testing.T) {
	tests := []struct {
		name        string
		inputs      map[string]interface{}
		expectError bool
		checkBody   func(t *testing.T, body []byte)
	}{
		{
			name: "push single log with string timestamp",
			inputs: map[string]interface{}{
				"line": "test log message",
				"labels": map[string]interface{}{
					"job":      "test-job",
					"instance": "test-instance",
				},
				"timestamp": time.Now().Format(time.RFC3339Nano),
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				streams, ok := payload["streams"].([]interface{})
				if !ok || len(streams) == 0 {
					t.Fatal("expected streams array")
				}

				stream := streams[0].(map[string]interface{})
				labels := stream["stream"].(map[string]interface{})
				if labels["job"] != "test-job" {
					t.Errorf("expected job label 'test-job', got %v", labels["job"])
				}

				values := stream["values"].([]interface{})
				if len(values) != 1 {
					t.Errorf("expected 1 value, got %d", len(values))
				}
			},
		},
		{
			name: "push single log with integer timestamp",
			inputs: map[string]interface{}{
				"line": "test log message",
				"labels": map[string]interface{}{
					"job": "test-job",
				},
				"timestamp": time.Now().UnixNano(),
			},
			expectError: false,
		},
		{
			name: "push single log without timestamp (auto-populate)",
			inputs: map[string]interface{}{
				"line": "test log message",
				"labels": map[string]interface{}{
					"job": "test-job",
				},
			},
			expectError: false,
		},
		{
			name: "missing line",
			inputs: map[string]interface{}{
				"labels": map[string]interface{}{
					"job": "test-job",
				},
			},
			expectError: true,
		},
		{
			name: "missing labels",
			inputs: map[string]interface{}{
				"line": "test log message",
			},
			expectError: true,
		},
		{
			name: "invalid timestamp format",
			inputs: map[string]interface{}{
				"line": "test log message",
				"labels": map[string]interface{}{
					"job": "test-job",
				},
				"timestamp": "invalid-timestamp",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResp := &transport.Response{
				StatusCode: 204,
				Body:       []byte{},
			}

			mock := &mockTransport{response: mockResp}
			config := &api.ProviderConfig{
				Transport: mock,
				BaseURL:   "https://loki.example.com:3100",
			}

			integration, err := NewLokiIntegration(config)
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			result, err := integration.Execute(context.Background(), "push", tt.inputs)

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

				// Check status field
				status, ok := result.Response.(map[string]interface{})["status"]
				if !ok || status != "ok" {
					t.Errorf("expected status 'ok', got %v", status)
				}
			}
		})
	}
}

func TestLokiIntegration_PushBatchLogs(t *testing.T) {
	tests := []struct {
		name        string
		inputs      map[string]interface{}
		expectError bool
		checkBody   func(t *testing.T, body []byte)
	}{
		{
			name: "push batch logs",
			inputs: map[string]interface{}{
				"labels": map[string]interface{}{
					"job":      "test-job",
					"instance": "test-instance",
				},
				"entries": []interface{}{
					map[string]interface{}{
						"line": "log message 1",
					},
					map[string]interface{}{
						"line": "log message 2",
					},
					map[string]interface{}{
						"line":      "log message 3",
						"timestamp": time.Now().UnixNano(),
					},
				},
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				streams := payload["streams"].([]interface{})
				stream := streams[0].(map[string]interface{})
				values := stream["values"].([]interface{})

				if len(values) != 3 {
					t.Errorf("expected 3 values, got %d", len(values))
				}
			},
		},
		{
			name: "push batch with RFC3339Nano timestamps",
			inputs: map[string]interface{}{
				"labels": map[string]interface{}{
					"job": "test-job",
				},
				"entries": []interface{}{
					map[string]interface{}{
						"line":      "log message 1",
						"timestamp": time.Now().Format(time.RFC3339Nano),
					},
					map[string]interface{}{
						"line":      "log message 2",
						"timestamp": time.Now().Add(time.Second).Format(time.RFC3339Nano),
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing labels for batch",
			inputs: map[string]interface{}{
				"entries": []interface{}{
					map[string]interface{}{
						"line": "log message 1",
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid entry type",
			inputs: map[string]interface{}{
				"labels": map[string]interface{}{
					"job": "test-job",
				},
				"entries": []interface{}{
					"not an object",
				},
			},
			expectError: true,
		},
		{
			name: "missing line in entry",
			inputs: map[string]interface{}{
				"labels": map[string]interface{}{
					"job": "test-job",
				},
				"entries": []interface{}{
					map[string]interface{}{
						"timestamp": time.Now().UnixNano(),
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid timestamp in entry",
			inputs: map[string]interface{}{
				"labels": map[string]interface{}{
					"job": "test-job",
				},
				"entries": []interface{}{
					map[string]interface{}{
						"line":      "log message",
						"timestamp": "invalid",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResp := &transport.Response{
				StatusCode: 204,
				Body:       []byte{},
			}

			mock := &mockTransport{response: mockResp}
			config := &api.ProviderConfig{
				Transport: mock,
				BaseURL:   "https://loki.example.com:3100",
			}

			integration, err := NewLokiIntegration(config)
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			result, err := integration.Execute(context.Background(), "push", tt.inputs)

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
			}
		})
	}
}

func TestLokiIntegration_Operations(t *testing.T) {
	mock := &mockTransport{}
	config := &api.ProviderConfig{
		Transport: mock,
		BaseURL:   "https://loki.example.com:3100",
	}

	integration, err := NewLokiIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	// Cast to concrete type to access Operations method
	lokiIntegration, ok := integration.(*LokiIntegration)
	if !ok {
		t.Fatal("failed to cast to LokiIntegration")
	}

	ops := lokiIntegration.Operations()
	if len(ops) != 1 {
		t.Errorf("expected 1 operation, got %d", len(ops))
	}

	if ops[0].Name != "push" {
		t.Errorf("expected operation 'push', got %s", ops[0].Name)
	}
}

func TestLokiIntegration_UnknownOperation(t *testing.T) {
	mock := &mockTransport{}
	config := &api.ProviderConfig{
		Transport: mock,
		BaseURL:   "https://loki.example.com:3100",
	}

	integration, err := NewLokiIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	_, err = integration.Execute(context.Background(), "unknown", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for unknown operation")
	}
}
