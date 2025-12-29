package splunk

import (
	"context"
	"encoding/json"
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

func TestNewSplunkIntegration(t *testing.T) {
	tests := []struct {
		name        string
		config      *api.ProviderConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &api.ProviderConfig{
				Transport: &mockTransport{},
				BaseURL:   "https://splunk.example.com:8088",
				Token:     "test-hec-token",
			},
			expectError: false,
		},
		{
			name: "missing transport",
			config: &api.ProviderConfig{
				BaseURL: "https://splunk.example.com:8088",
				Token:   "test-hec-token",
			},
			expectError: true,
		},
		{
			name: "missing base URL",
			config: &api.ProviderConfig{
				Transport: &mockTransport{},
				Token:     "test-hec-token",
			},
			expectError: true,
		},
		{
			name: "missing HEC token",
			config: &api.ProviderConfig{
				Transport: &mockTransport{},
				BaseURL:   "https://splunk.example.com:8088",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration, err := NewSplunkIntegration(tt.config)
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

func TestSplunkIntegration_SendLog(t *testing.T) {
	tests := []struct {
		name        string
		inputs      map[string]interface{}
		expectError bool
		checkBody   func(t *testing.T, body []byte)
	}{
		{
			name: "send string log",
			inputs: map[string]interface{}{
				"event":      "test log message",
				"index":      "main",
				"source":     "test-source",
				"sourcetype": "test-sourcetype",
				"host":       "test-host",
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				if payload["event"] != "test log message" {
					t.Errorf("expected event 'test log message', got %v", payload["event"])
				}
				if payload["index"] != "main" {
					t.Errorf("expected index 'main', got %v", payload["index"])
				}
				if payload["source"] != "test-source" {
					t.Errorf("expected source 'test-source', got %v", payload["source"])
				}
			},
		},
		{
			name: "send structured log",
			inputs: map[string]interface{}{
				"event": map[string]interface{}{
					"message": "test message",
					"level":   "info",
					"user":    "test-user",
				},
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				event, ok := payload["event"].(map[string]interface{})
				if !ok {
					t.Fatal("expected event to be an object")
				}
				if event["message"] != "test message" {
					t.Errorf("expected message 'test message', got %v", event["message"])
				}
			},
		},
		{
			name: "send log with explicit time",
			inputs: map[string]interface{}{
				"event": "test log",
				"time":  1234567890,
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				if payload["time"].(float64) != 1234567890 {
					t.Errorf("expected time 1234567890, got %v", payload["time"])
				}
			},
		},
		{
			name: "send log with fields",
			inputs: map[string]interface{}{
				"event": "test log",
				"fields": map[string]interface{}{
					"severity": "high",
					"category": "security",
				},
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				fields, ok := payload["fields"].(map[string]interface{})
				if !ok {
					t.Fatal("expected fields to be an object")
				}
				if fields["severity"] != "high" {
					t.Errorf("expected severity 'high', got %v", fields["severity"])
				}
			},
		},
		{
			name: "send log without time (auto-populate)",
			inputs: map[string]interface{}{
				"event": "test log",
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				if _, ok := payload["time"]; !ok {
					t.Error("expected time to be auto-populated")
				}
			},
		},
		{
			name:        "missing event",
			inputs:      map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResp := &transport.Response{
				StatusCode: 200,
				Body: []byte(`{
					"text": "Success",
					"code": 0,
					"ackId": 123
				}`),
			}

			mock := &mockTransport{response: mockResp}
			config := &api.ProviderConfig{
				Transport: mock,
				BaseURL:   "https://splunk.example.com:8088",
				Token:     "test-hec-token",
			}

			integration, err := NewSplunkIntegration(config)
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			result, err := integration.Execute(context.Background(), "log", tt.inputs)

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
			}
		})
	}
}

func TestSplunkIntegration_SendEvent(t *testing.T) {
	tests := []struct {
		name        string
		inputs      map[string]interface{}
		expectError bool
		checkBody   func(t *testing.T, body []byte)
	}{
		{
			name: "send structured event",
			inputs: map[string]interface{}{
				"fields": map[string]interface{}{
					"severity": "critical",
					"category": "security",
					"user":     "admin",
				},
				"index":      "security",
				"source":     "security-system",
				"sourcetype": "security-event",
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				event, ok := payload["event"].(map[string]interface{})
				if !ok {
					t.Fatal("expected event to be an object")
				}
				if event["severity"] != "critical" {
					t.Errorf("expected severity 'critical', got %v", event["severity"])
				}
				if payload["index"] != "security" {
					t.Errorf("expected index 'security', got %v", payload["index"])
				}
			},
		},
		{
			name: "send event with explicit time",
			inputs: map[string]interface{}{
				"fields": map[string]interface{}{
					"action": "login",
				},
				"time": 1234567890,
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				if payload["time"].(float64) != 1234567890 {
					t.Errorf("expected time 1234567890, got %v", payload["time"])
				}
			},
		},
		{
			name: "send event without time (auto-populate)",
			inputs: map[string]interface{}{
				"fields": map[string]interface{}{
					"action": "logout",
				},
			},
			expectError: false,
			checkBody: func(t *testing.T, body []byte) {
				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}

				if _, ok := payload["time"]; !ok {
					t.Error("expected time to be auto-populated")
				}
			},
		},
		{
			name:        "missing fields",
			inputs:      map[string]interface{}{},
			expectError: true,
		},
		{
			name: "empty fields",
			inputs: map[string]interface{}{
				"fields": map[string]interface{}{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResp := &transport.Response{
				StatusCode: 200,
				Body: []byte(`{
					"text": "Success",
					"code": 0,
					"ackId": 456
				}`),
			}

			mock := &mockTransport{response: mockResp}
			config := &api.ProviderConfig{
				Transport: mock,
				BaseURL:   "https://splunk.example.com:8088",
				Token:     "test-hec-token",
			}

			integration, err := NewSplunkIntegration(config)
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			result, err := integration.Execute(context.Background(), "event", tt.inputs)

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
			}
		})
	}
}

func TestSplunkIntegration_Operations(t *testing.T) {
	mock := &mockTransport{}
	config := &api.ProviderConfig{
		Transport: mock,
		BaseURL:   "https://splunk.example.com:8088",
		Token:     "test-hec-token",
	}

	integration, err := NewSplunkIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	// Cast to concrete type to access Operations method
	splunkIntegration, ok := integration.(*SplunkIntegration)
	if !ok {
		t.Fatal("failed to cast to SplunkIntegration")
	}

	ops := splunkIntegration.Operations()
	if len(ops) != 2 {
		t.Errorf("expected 2 operations, got %d", len(ops))
	}

	// Verify operation names
	opNames := make(map[string]bool)
	for _, op := range ops {
		opNames[op.Name] = true
	}

	if !opNames["log"] {
		t.Error("expected 'log' operation")
	}
	if !opNames["event"] {
		t.Error("expected 'event' operation")
	}
}

func TestSplunkIntegration_UnknownOperation(t *testing.T) {
	mock := &mockTransport{}
	config := &api.ProviderConfig{
		Transport: mock,
		BaseURL:   "https://splunk.example.com:8088",
		Token:     "test-hec-token",
	}

	integration, err := NewSplunkIntegration(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	_, err = integration.Execute(context.Background(), "unknown", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for unknown operation")
	}
}
