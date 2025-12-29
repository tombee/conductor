package jira

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
)

// mockTransport is a simple mock for testing.
type mockTransport struct {
	response *transport.Response
	err      error
}

func (m *mockTransport) Execute(ctx context.Context, req *transport.Request) (*transport.Response, error) {
	return m.response, m.err
}

func (m *mockTransport) Name() string {
	return "mock"
}

func (m *mockTransport) SetRateLimiter(limiter transport.RateLimiter) {
	// no-op for mock
}

func TestNewJiraIntegration(t *testing.T) {
	tests := []struct {
		name        string
		config      *api.ProviderConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &api.ProviderConfig{
				BaseURL: "https://test.atlassian.net",
				Token:   "test-token",
				AdditionalAuth: map[string]string{
					"email": "test@example.com",
				},
				Transport: &mockTransport{},
			},
			expectError: false,
		},
		{
			name: "missing base URL",
			config: &api.ProviderConfig{
				Token: "test-token",
				AdditionalAuth: map[string]string{
					"email": "test@example.com",
				},
				Transport: &mockTransport{},
			},
			expectError: true,
		},
		{
			name: "missing email",
			config: &api.ProviderConfig{
				BaseURL: "https://test.atlassian.net",
				Token:   "test-token",
				Transport: &mockTransport{},
			},
			expectError: true,
		},
		{
			name: "missing token",
			config: &api.ProviderConfig{
				BaseURL: "https://test.atlassian.net",
				AdditionalAuth: map[string]string{
					"email": "test@example.com",
				},
				Transport: &mockTransport{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewJiraIntegration(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if conn == nil {
					t.Error("expected connector but got nil")
				}

				// Verify the connector implements the interface
				if jc, ok := conn.(*JiraIntegration); ok {
					if jc.Name() != "jira" {
						t.Errorf("expected name 'jira', got %s", jc.Name())
					}
				} else {
					t.Error("connector is not a JiraIntegration")
				}
			}
		})
	}
}

func TestJiraIntegrationOperations(t *testing.T) {
	config := &api.ProviderConfig{
		BaseURL: "https://test.atlassian.net",
		Token:   "test-token",
		AdditionalAuth: map[string]string{
			"email": "test@example.com",
		},
		Transport: &mockTransport{},
	}

	conn, err := NewJiraIntegration(config)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}

	jc, ok := conn.(*JiraIntegration)
	if !ok {
		t.Fatal("connector is not a JiraIntegration")
	}

	operations := jc.Operations()

	expectedOps := []string{
		"create_issue", "update_issue", "get_issue", "transition_issue", "get_transitions",
		"add_comment", "assign_issue", "search_issues", "list_projects",
		"add_attachment", "link_issues",
	}

	if len(operations) != len(expectedOps) {
		t.Errorf("expected %d operations, got %d", len(expectedOps), len(operations))
	}

	opMap := make(map[string]bool)
	for _, op := range operations {
		opMap[op.Name] = true
	}

	for _, expected := range expectedOps {
		if !opMap[expected] {
			t.Errorf("missing operation: %s", expected)
		}
	}
}

func TestJiraIntegrationGetIssue(t *testing.T) {
	issue := Issue{
		ID:  "10001",
		Key: "TEST-1",
		Self: "https://test.atlassian.net/rest/api/3/issue/10001",
		Fields: IssueFields{
			Summary: "Test Issue",
		},
	}

	issueJSON, _ := json.Marshal(issue)

	mockResp := &transport.Response{
		StatusCode: 200,
		Body:       issueJSON,
		Headers:    make(map[string][]string),
		Metadata:   make(map[string]interface{}),
	}

	config := &api.ProviderConfig{
		BaseURL: "https://test.atlassian.net",
		Token:   "test-token",
		AdditionalAuth: map[string]string{
			"email": "test@example.com",
		},
		Transport: &mockTransport{
			response: mockResp,
		},
	}

	conn, err := NewJiraIntegration(config)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}

	inputs := map[string]interface{}{
		"issue_key": "TEST-1",
	}

	result, err := conn.Execute(context.Background(), "get_issue", inputs)
	if err != nil {
		t.Fatalf("failed to execute get_issue: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}

	// Verify response contains expected fields
	response, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatal("response is not a map")
	}

	if response["key"] != "TEST-1" {
		t.Errorf("expected key TEST-1, got %v", response["key"])
	}
	if response["summary"] != "Test Issue" {
		t.Errorf("expected summary 'Test Issue', got %v", response["summary"])
	}
}

func TestJiraIntegrationUnknownOperation(t *testing.T) {
	config := &api.ProviderConfig{
		BaseURL: "https://test.atlassian.net",
		Token:   "test-token",
		AdditionalAuth: map[string]string{
			"email": "test@example.com",
		},
		Transport: &mockTransport{},
	}

	conn, err := NewJiraIntegration(config)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}

	_, err = conn.Execute(context.Background(), "unknown_operation", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for unknown operation")
	}
}
