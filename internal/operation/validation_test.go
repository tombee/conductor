package operation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_ValidateLokiLabels(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		labels  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil labels",
			labels:  nil,
			wantErr: false,
		},
		{
			name: "valid labels",
			labels: map[string]interface{}{
				"app":        "conductor",
				"env":        "prod",
				"workflow_1": "test",
				"_internal":  "value",
			},
			wantErr: false,
		},
		{
			name: "reserved label with double underscore",
			labels: map[string]interface{}{
				"__meta": "value",
			},
			wantErr: true,
			errMsg:  "cannot start with '__'",
		},
		{
			name: "invalid label name with dash",
			labels: map[string]interface{}{
				"app-name": "value",
			},
			wantErr: true,
			errMsg:  "invalid loki label name",
		},
		{
			name: "invalid label name starting with number",
			labels: map[string]interface{}{
				"1app": "value",
			},
			wantErr: true,
			errMsg:  "invalid loki label name",
		},
		{
			name: "empty label value",
			labels: map[string]interface{}{
				"app": "",
			},
			wantErr: true,
			errMsg:  "label value cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateLokiLabels(tt.labels)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateDatadogSite(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		site    string
		wantErr bool
	}{
		{
			name:    "empty site (default)",
			site:    "",
			wantErr: false,
		},
		{
			name:    "US1 site",
			site:    "datadoghq.com",
			wantErr: false,
		},
		{
			name:    "US3 site",
			site:    "us3.datadoghq.com",
			wantErr: false,
		},
		{
			name:    "US5 site",
			site:    "us5.datadoghq.com",
			wantErr: false,
		},
		{
			name:    "EU site",
			site:    "datadoghq.eu",
			wantErr: false,
		},
		{
			name:    "AP1 site",
			site:    "ap1.datadoghq.com",
			wantErr: false,
		},
		{
			name:    "Gov site",
			site:    "ddog-gov.com",
			wantErr: false,
		},
		{
			name:    "invalid site",
			site:    "invalid.datadoghq.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateDatadogSite(tt.site)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateElasticsearchIndex(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		index   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid index name",
			index:   "logs-2024-01-01",
			wantErr: false,
		},
		{
			name:    "valid index with underscore",
			index:   "my_index",
			wantErr: false,
		},
		{
			name:    "valid index with dots",
			index:   "logs.app.prod",
			wantErr: false,
		},
		{
			name:    "empty index",
			index:   "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "uppercase index",
			index:   "MyIndex",
			wantErr: true,
			errMsg:  "must be lowercase",
		},
		{
			name:    "index with space",
			index:   "my index",
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name:    "index with backslash",
			index:   "my\\index",
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name:    "index with asterisk",
			index:   "logs-*",
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name:    "index is dot",
			index:   ".",
			wantErr: true,
			errMsg:  "cannot be '.' or '..'",
		},
		{
			name:    "index is double dot",
			index:   "..",
			wantErr: true,
			errMsg:  "cannot be '.' or '..'",
		},
		{
			name:    "index starts with dash",
			index:   "-myindex",
			wantErr: true,
			errMsg:  "cannot start with",
		},
		{
			name:    "index starts with underscore",
			index:   "_myindex",
			wantErr: true,
			errMsg:  "cannot start with",
		},
		{
			name:    "index too long",
			index:   string(make([]byte, 256)),
			wantErr: true,
			errMsg:  "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateElasticsearchIndex(tt.index)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateIntegrationInputs(t *testing.T) {
	v := NewValidator()

	t.Run("loki connector with invalid labels", func(t *testing.T) {
		inputs := map[string]interface{}{
			"line": "test log",
			"labels": map[string]interface{}{
				"app-name": "value", // Invalid: contains dash
			},
		}

		err := v.ValidateIntegrationInputs("loki", "push", inputs)
		assert.Error(t, err)
	})

	t.Run("elasticsearch connector with invalid index", func(t *testing.T) {
		inputs := map[string]interface{}{
			"index": "MyIndex", // Invalid: uppercase
			"document": map[string]interface{}{
				"message": "test",
			},
		}

		err := v.ValidateIntegrationInputs("elasticsearch", "index", inputs)
		assert.Error(t, err)
	})

	t.Run("elasticsearch with date math index", func(t *testing.T) {
		inputs := map[string]interface{}{
			"index": "<logs-{now/d}>", // Date math - should skip validation
			"document": map[string]interface{}{
				"message": "test",
			},
		}

		err := v.ValidateIntegrationInputs("elasticsearch", "index", inputs)
		assert.NoError(t, err)
	})

	t.Run("unknown connector", func(t *testing.T) {
		inputs := map[string]interface{}{
			"any": "value",
		}

		err := v.ValidateIntegrationInputs("unknown", "op", inputs)
		assert.NoError(t, err) // No validation for unknown connectors
	})
}
