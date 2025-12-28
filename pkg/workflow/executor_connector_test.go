package workflow

import (
	"context"
	"testing"
)

// mockConnectorRegistry implements ConnectorRegistry for testing.
type mockConnectorRegistry struct {
	executeFunc func(ctx context.Context, reference string, inputs map[string]interface{}) (ConnectorResult, error)
}

func (m *mockConnectorRegistry) Execute(ctx context.Context, reference string, inputs map[string]interface{}) (ConnectorResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, reference, inputs)
	}
	return &mockConnectorResult{
		response: map[string]interface{}{
			"success": true,
			"data":    "test response",
		},
		statusCode: 200,
	}, nil
}

// mockConnectorResult implements ConnectorResult for testing.
type mockConnectorResult struct {
	response    interface{}
	rawResponse interface{}
	statusCode  int
	metadata    map[string]interface{}
}

func (m *mockConnectorResult) GetResponse() interface{} {
	return m.response
}

func (m *mockConnectorResult) GetRawResponse() interface{} {
	return m.rawResponse
}

func (m *mockConnectorResult) GetStatusCode() int {
	return m.statusCode
}

func (m *mockConnectorResult) GetMetadata() map[string]interface{} {
	return m.metadata
}

func TestExecuteConnector(t *testing.T) {
	tests := []struct {
		name           string
		step           *StepDefinition
		inputs         map[string]interface{}
		mockRegistry   *mockConnectorRegistry
		expectedOutput map[string]interface{}
		expectError    bool
	}{
		{
			name: "successful connector execution",
			step: &StepDefinition{
				ID:        "test_step",
				Type:      StepTypeConnector,
				Connector: "github.create_issue",
			},
			inputs: map[string]interface{}{
				"title": "Test Issue",
				"body":  "Test description",
			},
			mockRegistry: &mockConnectorRegistry{
				executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (ConnectorResult, error) {
					if reference != "github.create_issue" {
						t.Errorf("unexpected reference: got %s, want github.create_issue", reference)
					}
					return &mockConnectorResult{
						response: map[string]interface{}{
							"id":     123,
							"number": 42,
							"url":    "https://api.github.com/repos/test/repo/issues/42",
						},
						statusCode: 201,
						metadata: map[string]interface{}{
							"request_id": "test-123",
						},
					}, nil
				},
			},
			expectedOutput: map[string]interface{}{
				"response": map[string]interface{}{
					"id":     123,
					"number": 42,
					"url":    "https://api.github.com/repos/test/repo/issues/42",
				},
				"status_code": 201,
				"metadata": map[string]interface{}{
					"request_id": "test-123",
				},
			},
			expectError: false,
		},
		{
			name: "connector step with missing connector field",
			step: &StepDefinition{
				ID:        "test_step",
				Type:      StepTypeConnector,
				Connector: "",
			},
			inputs:       map[string]interface{}{},
			mockRegistry: &mockConnectorRegistry{},
			expectError:  true,
		},
		{
			name: "connector step without registry",
			step: &StepDefinition{
				ID:        "test_step",
				Type:      StepTypeConnector,
				Connector: "github.create_issue",
			},
			inputs:       map[string]interface{}{},
			mockRegistry: nil,
			expectError:  true,
		},
		{
			name: "connector returns nil result without error (contract violation)",
			step: &StepDefinition{
				ID:        "test_step",
				Type:      StepTypeConnector,
				Connector: "broken.connector",
			},
			inputs: map[string]interface{}{},
			mockRegistry: &mockConnectorRegistry{
				executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (ConnectorResult, error) {
					// Return (nil, nil) - contract violation
					return nil, nil
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create executor
			executor := NewExecutor(nil, nil)
			if tt.mockRegistry != nil {
				executor = executor.WithConnectorRegistry(tt.mockRegistry)
			}

			// Execute connector step
			output, err := executor.executeConnector(context.Background(), tt.step, tt.inputs)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check output if no error expected
			if !tt.expectError && output != nil {
				// Verify response is present
				if _, ok := output["response"]; !ok {
					t.Errorf("expected response in output")
				}
			}
		})
	}
}

func TestMaskSensitiveInputs(t *testing.T) {
	tests := []struct {
		name     string
		inputs   map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "masks token field",
			inputs: map[string]interface{}{
				"token": "secret123",
				"title": "Test Issue",
			},
			expected: map[string]interface{}{
				"token": "***MASKED***",
				"title": "Test Issue",
			},
		},
		{
			name: "masks password field",
			inputs: map[string]interface{}{
				"password": "secret123",
				"username": "testuser",
			},
			expected: map[string]interface{}{
				"password": "***MASKED***",
				"username": "testuser",
			},
		},
		{
			name: "masks api_key field",
			inputs: map[string]interface{}{
				"api_key": "secret123",
				"data":    "test data",
			},
			expected: map[string]interface{}{
				"api_key": "***MASKED***",
				"data":    "test data",
			},
		},
		{
			name: "case insensitive masking",
			inputs: map[string]interface{}{
				"TOKEN":    "secret123",
				"Password": "secret456",
				"data":     "test data",
			},
			expected: map[string]interface{}{
				"TOKEN":    "***MASKED***",
				"Password": "***MASKED***",
				"data":     "test data",
			},
		},
		{
			name: "masks fields containing sensitive terms",
			inputs: map[string]interface{}{
				"github_token": "secret123",
				"api_password": "secret456",
				"title":        "Test",
			},
			expected: map[string]interface{}{
				"github_token": "***MASKED***",
				"api_password": "***MASKED***",
				"title":        "Test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveInputs(tt.inputs)

			// Check each expected key
			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("key %s: got %v, want %v", key, result[key], expectedValue)
				}
			}
		})
	}
}
