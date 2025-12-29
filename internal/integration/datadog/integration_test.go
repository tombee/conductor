package datadog

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
)

// mockTransport is a mock transport for testing.
type mockTransport struct {
	lastRequest  *transport.Request
	response     *transport.Response
	err          error
	rateLimiter  transport.RateLimiter
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

func TestNewDatadogIntegration(t *testing.T) {
	tests := []struct {
		name    string
		config  *api.ConnectorConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with default site",
			config: &api.ConnectorConfig{
				Transport: &mockTransport{},
				Token:     "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "valid config with custom site",
			config: &api.ConnectorConfig{
				Transport: &mockTransport{},
				Token:     "test-api-key",
				AdditionalAuth: map[string]string{
					"site": "datadoghq.eu",
				},
			},
			wantErr: false,
		},
		{
			name: "missing transport",
			config: &api.ConnectorConfig{
				Token: "test-api-key",
			},
			wantErr: true,
			errMsg:  "transport is required",
		},
		{
			name: "missing API key",
			config: &api.ConnectorConfig{
				Transport: &mockTransport{},
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
		{
			name: "invalid site",
			config: &api.ConnectorConfig{
				Transport: &mockTransport{},
				Token:     "test-api-key",
				AdditionalAuth: map[string]string{
					"site": "invalid.site.com",
				},
			},
			wantErr: true,
			errMsg:  "invalid Datadog site",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration, err := NewDatadogIntegration(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if integration == nil {
				t.Error("expected integration to be non-nil")
			}
		})
	}
}

func TestDatadogIntegration_SendLog(t *testing.T) {
	tests := []struct {
		name       string
		inputs     map[string]interface{}
		wantErr    bool
		errMsg     string
		validateFn func(*testing.T, *transport.Request)
	}{
		{
			name: "minimal log",
			inputs: map[string]interface{}{
				"message": "test log message",
			},
			wantErr: false,
			validateFn: func(t *testing.T, req *transport.Request) {
				if req.Method != "POST" {
					t.Errorf("expected POST, got %s", req.Method)
				}
				if !contains(req.URL, "/api/v2/logs") {
					t.Errorf("expected URL to contain /api/v2/logs, got %s", req.URL)
				}
				if !contains(req.URL, "http-intake.logs") {
					t.Errorf("expected URL to use logs intake endpoint, got %s", req.URL)
				}

				var body []map[string]interface{}
				if err := json.Unmarshal(req.Body, &body); err != nil {
					t.Fatalf("failed to unmarshal body: %v", err)
				}
				if len(body) != 1 {
					t.Errorf("expected 1 log entry, got %d", len(body))
				}
				if body[0]["message"] != "test log message" {
					t.Errorf("expected message 'test log message', got %v", body[0]["message"])
				}
			},
		},
		{
			name: "log with all fields",
			inputs: map[string]interface{}{
				"message":   "test log",
				"status":    "error",
				"service":   "my-service",
				"source":    "golang",
				"tags":      []interface{}{"env:prod", "version:1.0"},
				"hostname":  "host1",
				"timestamp": int64(1234567890),
				"attributes": map[string]interface{}{
					"user_id": "123",
					"action":  "login",
				},
			},
			wantErr: false,
			validateFn: func(t *testing.T, req *transport.Request) {
				var body []map[string]interface{}
				if err := json.Unmarshal(req.Body, &body); err != nil {
					t.Fatalf("failed to unmarshal body: %v", err)
				}
				if body[0]["status"] != "error" {
					t.Errorf("expected status 'error', got %v", body[0]["status"])
				}
				if body[0]["service"] != "my-service" {
					t.Errorf("expected service 'my-service', got %v", body[0]["service"])
				}
				if body[0]["ddsource"] != "golang" {
					t.Errorf("expected ddsource 'golang', got %v", body[0]["ddsource"])
				}
			},
		},
		{
			name:    "missing message",
			inputs:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "missing required parameter: message",
		},
		{
			name: "invalid status",
			inputs: map[string]interface{}{
				"message": "test",
				"status":  "invalid",
			},
			wantErr: true,
			errMsg:  "invalid status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTransport{
				response: &transport.Response{
					StatusCode: 200,
					Body:       []byte(`{}`),
				},
			}

			integration, err := NewDatadogIntegration(&api.ConnectorConfig{
				Transport: mock,
				Token:     "test-key",
			})
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			result, err := integration.Execute(context.Background(), "log", tt.inputs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("expected result to be non-nil")
				return
			}

			if tt.validateFn != nil {
				tt.validateFn(t, mock.lastRequest)
			}
		})
	}
}

func TestDatadogIntegration_SendMetric(t *testing.T) {
	tests := []struct {
		name       string
		inputs     map[string]interface{}
		wantErr    bool
		errMsg     string
		validateFn func(*testing.T, *transport.Request)
	}{
		{
			name: "single metric",
			inputs: map[string]interface{}{
				"metric": "my.metric",
				"value":  float64(42.5),
			},
			wantErr: false,
			validateFn: func(t *testing.T, req *transport.Request) {
				if req.Method != "POST" {
					t.Errorf("expected POST, got %s", req.Method)
				}
				if !contains(req.URL, "/api/v2/series") {
					t.Errorf("expected URL to contain /api/v2/series, got %s", req.URL)
				}

				var body map[string]interface{}
				if err := json.Unmarshal(req.Body, &body); err != nil {
					t.Fatalf("failed to unmarshal body: %v", err)
				}

				series, ok := body["series"].([]interface{})
				if !ok || len(series) != 1 {
					t.Errorf("expected 1 metric in series, got %v", series)
				}
			},
		},
		{
			name: "batch metrics",
			inputs: map[string]interface{}{
				"series": []interface{}{
					map[string]interface{}{
						"metric": "metric1",
						"value":  float64(10),
					},
					map[string]interface{}{
						"metric": "metric2",
						"value":  float64(20),
					},
				},
			},
			wantErr: false,
			validateFn: func(t *testing.T, req *transport.Request) {
				var body map[string]interface{}
				if err := json.Unmarshal(req.Body, &body); err != nil {
					t.Fatalf("failed to unmarshal body: %v", err)
				}

				series, ok := body["series"].([]interface{})
				if !ok || len(series) != 2 {
					t.Errorf("expected 2 metrics in series, got %v", series)
				}
			},
		},
		{
			name: "metric with type and tags",
			inputs: map[string]interface{}{
				"metric": "my.gauge",
				"value":  float64(100),
				"type":   "gauge",
				"tags":   []interface{}{"env:prod"},
			},
			wantErr: false,
		},
		{
			name:    "missing metric name",
			inputs:  map[string]interface{}{"value": float64(1)},
			wantErr: true,
			errMsg:  "missing required parameter: metric",
		},
		{
			name:    "missing value",
			inputs:  map[string]interface{}{"metric": "test"},
			wantErr: true,
			errMsg:  "missing required parameter: value",
		},
		{
			name: "invalid metric type",
			inputs: map[string]interface{}{
				"metric": "test",
				"value":  float64(1),
				"type":   "invalid",
			},
			wantErr: true,
			errMsg:  "invalid type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTransport{
				response: &transport.Response{
					StatusCode: 200,
					Body:       []byte(`{}`),
				},
			}

			integration, err := NewDatadogIntegration(&api.ConnectorConfig{
				Transport: mock,
				Token:     "test-key",
			})
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			result, err := integration.Execute(context.Background(), "metric", tt.inputs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("expected result to be non-nil")
				return
			}

			if tt.validateFn != nil {
				tt.validateFn(t, mock.lastRequest)
			}
		})
	}
}

func TestDatadogIntegration_SendEvent(t *testing.T) {
	tests := []struct {
		name       string
		inputs     map[string]interface{}
		wantErr    bool
		errMsg     string
		validateFn func(*testing.T, *transport.Request)
	}{
		{
			name: "minimal event",
			inputs: map[string]interface{}{
				"title": "Test Event",
				"text":  "Event description",
			},
			wantErr: false,
			validateFn: func(t *testing.T, req *transport.Request) {
				if req.Method != "POST" {
					t.Errorf("expected POST, got %s", req.Method)
				}
				if !contains(req.URL, "/api/v1/events") {
					t.Errorf("expected URL to contain /api/v1/events, got %s", req.URL)
				}

				var body map[string]interface{}
				if err := json.Unmarshal(req.Body, &body); err != nil {
					t.Fatalf("failed to unmarshal body: %v", err)
				}
				if body["title"] != "Test Event" {
					t.Errorf("expected title 'Test Event', got %v", body["title"])
				}
			},
		},
		{
			name: "event with all fields",
			inputs: map[string]interface{}{
				"title":            "Deploy",
				"text":             "Version 2.0 deployed",
				"priority":         "normal",
				"alert_type":       "success",
				"tags":             []interface{}{"env:prod"},
				"aggregation_key":  "deploy",
				"source_type_name": "conductor",
				"host":             "host1",
			},
			wantErr: false,
		},
		{
			name:    "missing title",
			inputs:  map[string]interface{}{"text": "test"},
			wantErr: true,
			errMsg:  "missing required parameter: title",
		},
		{
			name:    "missing text",
			inputs:  map[string]interface{}{"title": "test"},
			wantErr: true,
			errMsg:  "missing required parameter: text",
		},
		{
			name: "invalid priority",
			inputs: map[string]interface{}{
				"title":    "test",
				"text":     "test",
				"priority": "invalid",
			},
			wantErr: true,
			errMsg:  "invalid priority",
		},
		{
			name: "invalid alert_type",
			inputs: map[string]interface{}{
				"title":      "test",
				"text":       "test",
				"alert_type": "invalid",
			},
			wantErr: true,
			errMsg:  "invalid alert_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTransport{
				response: &transport.Response{
					StatusCode: 200,
					Body:       []byte(`{"event":{"id":"123"}}`),
				},
			}

			integration, err := NewDatadogIntegration(&api.ConnectorConfig{
				Transport: mock,
				Token:     "test-key",
			})
			if err != nil {
				t.Fatalf("failed to create integration: %v", err)
			}

			result, err := integration.Execute(context.Background(), "event", tt.inputs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("expected result to be non-nil")
				return
			}

			if tt.validateFn != nil {
				tt.validateFn(t, mock.lastRequest)
			}
		})
	}
}

func TestDatadogIntegration_Operations(t *testing.T) {
	integration, err := NewDatadogIntegration(&api.ConnectorConfig{
		Transport: &mockTransport{},
		Token:     "test-key",
	})
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	// Cast to DatadogIntegration to access Operations method
	datadogIntegration, ok := integration.(*DatadogIntegration)
	if !ok {
		t.Fatal("expected *DatadogIntegration")
	}

	ops := datadogIntegration.Operations()
	if len(ops) != 3 {
		t.Errorf("expected 3 operations, got %d", len(ops))
	}

	opNames := make(map[string]bool)
	for _, op := range ops {
		opNames[op.Name] = true
	}

	expectedOps := []string{"log", "metric", "event"}
	for _, name := range expectedOps {
		if !opNames[name] {
			t.Errorf("expected operation %q not found", name)
		}
	}
}

func TestDatadogIntegration_UnknownOperation(t *testing.T) {
	integration, err := NewDatadogIntegration(&api.ConnectorConfig{
		Transport: &mockTransport{},
		Token:     "test-key",
	})
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	_, err = integration.Execute(context.Background(), "unknown", nil)
	if err == nil {
		t.Error("expected error for unknown operation")
	}
	if !contains(err.Error(), "unknown operation") {
		t.Errorf("expected 'unknown operation' error, got %v", err)
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
