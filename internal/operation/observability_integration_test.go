package operation_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/pkg/workflow"
)

// TestObservabilityIntegrationCreation tests that observability integrations can be created
// with proper configuration.
func TestObservabilityIntegrationCreation(t *testing.T) {
	tests := []struct {
		name          string
		integrationName string
		operations    map[string]workflow.OperationDefinition
	}{
		{
			name:          "datadog integration",
			integrationName: "datadog",
			operations: map[string]workflow.OperationDefinition{
				"log": {
					Method: "POST",
					Path:   "/api/v2/logs",
				},
			},
		},
		{
			name:          "splunk integration",
			integrationName: "splunk",
			operations: map[string]workflow.OperationDefinition{
				"log": {
					Method: "POST",
					Path:   "/services/collector/event",
				},
			},
		},
		{
			name:          "loki integration",
			integrationName: "loki",
			operations: map[string]workflow.OperationDefinition{
				"push": {
					Method: "POST",
					Path:   "/loki/api/v1/push",
				},
			},
		},
		{
			name:          "elasticsearch integration",
			integrationName: "elasticsearch",
			operations: map[string]workflow.OperationDefinition{
				"index": {
					Method: "POST",
					Path:   "/{index}/_doc",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &workflow.IntegrationDefinition{
				Name:       tt.integrationName,
				BaseURL:    "https://example.com",
				Operations: tt.operations,
			}

			config := operation.DefaultConfig()
			conn, err := operation.New(def, config)

			require.NoError(t, err)
			assert.NotNil(t, conn)
			assert.Equal(t, tt.integrationName, conn.Name())
		})
	}
}

// TestLokiLabelValidation tests Loki label validation integration.
func TestLokiLabelValidation(t *testing.T) {
	def := &workflow.IntegrationDefinition{
		Name:    "loki",
		BaseURL: "https://loki.example.com",
		Operations: map[string]workflow.OperationDefinition{
			"push": {
				Method: "POST",
				Path:   "/loki/api/v1/push",
			},
		},
	}

	config := operation.DefaultConfig()
	conn, err := operation.New(def, config)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("invalid label with dash", func(t *testing.T) {
		_, err := conn.Execute(ctx, "push", map[string]interface{}{
			"line": "Test log",
			"labels": map[string]interface{}{
				"app-name": "invalid", // Dashes not allowed
			},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid loki label name")
	})

	t.Run("reserved label with double underscore", func(t *testing.T) {
		_, err := conn.Execute(ctx, "push", map[string]interface{}{
			"line": "Test log",
			"labels": map[string]interface{}{
				"__reserved": "invalid", // Reserved prefix
			},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot start with '__'")
	})

	t.Run("empty label value", func(t *testing.T) {
		_, err := conn.Execute(ctx, "push", map[string]interface{}{
			"line": "Test log",
			"labels": map[string]interface{}{
				"app": "", // Empty value
			},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "label value cannot be empty")
	})
}

// TestElasticsearchIndexValidation tests Elasticsearch index validation integration.
func TestElasticsearchIndexValidation(t *testing.T) {
	def := &workflow.IntegrationDefinition{
		Name:    "elasticsearch",
		BaseURL: "https://elasticsearch.example.com",
		Operations: map[string]workflow.OperationDefinition{
			"index": {
				Method: "POST",
				Path:   "/{index}/_doc",
			},
		},
	}

	config := operation.DefaultConfig()
	conn, err := operation.New(def, config)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("uppercase index rejected", func(t *testing.T) {
		_, err := conn.Execute(ctx, "index", map[string]interface{}{
			"index": "MyIndex", // Uppercase not allowed
			"document": map[string]interface{}{
				"message": "Test",
			},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be lowercase")
	})

	t.Run("index with invalid character", func(t *testing.T) {
		_, err := conn.Execute(ctx, "index", map[string]interface{}{
			"index": "my*index", // Asterisk not allowed
			"document": map[string]interface{}{
				"message": "Test",
			},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("date math index allowed", func(t *testing.T) {
		// Date math expressions should skip validation
		// The error will be connection-related, not validation
		_, err := conn.Execute(ctx, "index", map[string]interface{}{
			"index": "<logs-{now/d}>",
			"document": map[string]interface{}{
				"message": "Test",
			},
		})

		// Should not be a validation error
		if err != nil {
			connErr, ok := err.(*operation.Error)
			if ok {
				assert.NotEqual(t, operation.ErrorTypeValidation, connErr.Type)
			}
		}
	})
}

// TestDefaultFieldInjectionIntegration tests that default fields are injected.
func TestDefaultFieldInjectionIntegration(t *testing.T) {
	tests := []struct {
		name          string
		integrationName string
		operation     string
		inputs        map[string]interface{}
		expectFields  []string
	}{
		{
			name:          "datadog adds timestamp and hostname",
			integrationName: "datadog",
			operation:     "log",
			inputs: map[string]interface{}{
				"message": "Test log",
			},
			expectFields: []string{"timestamp", "hostname"},
		},
		{
			name:          "loki adds timestamp",
			integrationName: "loki",
			operation:     "push",
			inputs: map[string]interface{}{
				"line": "Test log",
			},
			expectFields: []string{"timestamp"},
		},
		{
			name:          "cloudwatch adds timestamp",
			integrationName: "cloudwatch",
			operation:     "log",
			inputs: map[string]interface{}{
				"message": "Test log",
			},
			expectFields: []string{"timestamp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of inputs to check if fields were added
			inputsCopy := make(map[string]interface{})
			for k, v := range tt.inputs {
				inputsCopy[k] = v
			}

			// Inject defaults
			injector := operation.NewDefaultFieldInjector()
			injector.InjectDefaults(inputsCopy, tt.integrationName)

			// Verify expected fields were added
			for _, field := range tt.expectFields {
				assert.Contains(t, inputsCopy, field, "expected field %s to be injected", field)
			}
		})
	}
}
