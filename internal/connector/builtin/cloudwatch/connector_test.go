package cloudwatch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tombee/conductor/internal/connector/api"
	"github.com/tombee/conductor/internal/connector/transport"
)

// mockTransport is a mock implementation of transport.Transport for testing.
type mockTransport struct {
	name        string
	executeFunc func(ctx context.Context, req *transport.Request) (*transport.Response, error)
}

func (m *mockTransport) Execute(ctx context.Context, req *transport.Request) (*transport.Response, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return &transport.Response{
		StatusCode: 200,
		Body:       []byte(`{}`),
		Headers:    map[string][]string{},
		Metadata:   map[string]interface{}{},
	}, nil
}

func (m *mockTransport) Name() string {
	return m.name
}

func (m *mockTransport) SetRateLimiter(limiter transport.RateLimiter) {
	// No-op for mock
}

func TestNewCloudWatchConnector(t *testing.T) {
	tests := []struct {
		name    string
		config  *api.ConnectorConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with aws_sigv4 transport",
			config: &api.ConnectorConfig{
				Transport: &mockTransport{name: "aws_sigv4"},
			},
			wantErr: false,
		},
		{
			name: "missing transport",
			config: &api.ConnectorConfig{
				Transport: nil,
			},
			wantErr: true,
			errMsg:  "transport is required",
		},
		{
			name: "wrong transport type",
			config: &api.ConnectorConfig{
				Transport: &mockTransport{name: "http"},
			},
			wantErr: true,
			errMsg:  "requires aws_sigv4 transport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewCloudWatchConnector(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, conn)
				assert.Equal(t, "cloudwatch", conn.Name())
			}
		})
	}
}

func TestCloudWatchConnector_Operations(t *testing.T) {
	mock := &mockTransport{name: "aws_sigv4"}
	config := &api.ConnectorConfig{
		Transport: mock,
	}

	conn, err := NewCloudWatchConnector(config)
	require.NoError(t, err)

	ops := conn.(*CloudWatchConnector).Operations()
	assert.Len(t, ops, 2)

	opNames := make([]string, len(ops))
	for i, op := range ops {
		opNames[i] = op.Name
	}
	assert.Contains(t, opNames, "log")
	assert.Contains(t, opNames, "metric")
}

func TestCloudWatchConnector_Execute_InvalidOperation(t *testing.T) {
	mock := &mockTransport{name: "aws_sigv4"}
	config := &api.ConnectorConfig{
		Transport: mock,
	}

	conn, err := NewCloudWatchConnector(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = conn.Execute(ctx, "invalid_operation", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")
}

func TestCloudWatchConnector_SequenceTokenCache(t *testing.T) {
	mock := &mockTransport{name: "aws_sigv4"}
	config := &api.ConnectorConfig{
		Transport: mock,
	}

	conn, err := NewCloudWatchConnector(config)
	require.NoError(t, err)

	cw := conn.(*CloudWatchConnector)

	// Test sequence token caching
	logGroup := "/test/log-group"
	logStream := "test-stream"
	token := "test-token-123"

	// Initially no token
	assert.Empty(t, cw.getSequenceToken(logGroup, logStream))

	// Set token
	cw.setSequenceToken(logGroup, logStream, token)
	assert.Equal(t, token, cw.getSequenceToken(logGroup, logStream))

	// Clear token
	cw.clearSequenceToken(logGroup, logStream)
	assert.Empty(t, cw.getSequenceToken(logGroup, logStream))
}

func TestBuildMetricDatum(t *testing.T) {
	mock := &mockTransport{name: "aws_sigv4"}
	config := &api.ConnectorConfig{
		Transport: mock,
	}

	conn, err := NewCloudWatchConnector(config)
	require.NoError(t, err)

	cw := conn.(*CloudWatchConnector)

	tests := []struct {
		name    string
		inputs  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid metric with all fields",
			inputs: map[string]interface{}{
				"name":  "test.metric",
				"value": 42.0,
				"unit":  "Count",
				"dimensions": map[string]interface{}{
					"Environment": "test",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			inputs: map[string]interface{}{
				"value": 42.0,
			},
			wantErr: true,
		},
		{
			name: "missing value",
			inputs: map[string]interface{}{
				"name": "test.metric",
			},
			wantErr: true,
		},
		{
			name: "invalid unit",
			inputs: map[string]interface{}{
				"name":  "test.metric",
				"value": 42.0,
				"unit":  "InvalidUnit",
			},
			wantErr: true,
		},
		{
			name: "too many dimensions",
			inputs: map[string]interface{}{
				"name":  "test.metric",
				"value": 42.0,
				"dimensions": func() map[string]interface{} {
					dims := make(map[string]interface{})
					for i := 0; i < 31; i++ {
						dims[string(rune('A'+i))] = "value"
					}
					return dims
				}(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			datum, err := cw.buildMetricDatum(tt.inputs)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, datum)
				assert.Equal(t, tt.inputs["name"], datum["MetricName"])
			}
		})
	}
}

func TestExtractSequenceTokenFromError(t *testing.T) {
	mock := &mockTransport{name: "aws_sigv4"}
	config := &api.ConnectorConfig{
		Transport: mock,
	}

	conn, err := NewCloudWatchConnector(config)
	require.NoError(t, err)

	cw := conn.(*CloudWatchConnector)

	tests := []struct {
		name      string
		errMsg    string
		wantToken string
	}{
		{
			name:      "valid error message",
			errMsg:    "The next expected sequenceToken is: 49545716249838168516693949323510663759374363441972887554",
			wantToken: "49545716249838168516693949323510663759374363441972887554",
		},
		{
			name:      "error message with newline",
			errMsg:    "The next expected sequenceToken is: 12345\nadditional info",
			wantToken: "12345",
		},
		{
			name:      "no token in message",
			errMsg:    "Some other error",
			wantToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transportErr := &transport.TransportError{
				Message: tt.errMsg,
			}

			token := cw.extractSequenceTokenFromError(transportErr)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}
