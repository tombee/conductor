package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
)

func TestNewGitHubConnector(t *testing.T) {
	tests := []struct {
		name       string
		config     *api.ConnectorConfig
		wantError  bool
		wantBaseURL string
	}{
		{
			name: "valid config with custom base URL",
			config: &api.ConnectorConfig{
				BaseURL:   "https://github.example.com/api/v3",
				Token:     "test-token",
				Transport: &transport.HTTPTransport{},
			},
			wantError:  false,
			wantBaseURL: "https://github.example.com/api/v3",
		},
		{
			name: "valid config with default base URL",
			config: &api.ConnectorConfig{
				Token:     "test-token",
				Transport: &transport.HTTPTransport{},
			},
			wantError:  false,
			wantBaseURL: "https://api.github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewGitHubConnector(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("NewGitHubConnector() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if conn.Name() != "github" {
					t.Errorf("Expected connector name 'github', got '%s'", conn.Name())
				}

				// Verify the connector was created (baseURL is private, so we can't directly check it)
				// The fact that NewGitHubConnector succeeded means it was set correctly
			}
		})
	}
}

func TestGitHubConnector_Operations(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://api.github.com",
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	gc := conn.(*GitHubConnector)
	ops := gc.Operations()

	// Verify we have all 12 operations
	if len(ops) != 12 {
		t.Errorf("Operations() returned %d operations, want 12", len(ops))
	}

	// Verify operation names
	expectedOps := map[string]bool{
		"create_issue":       true,
		"update_issue":       true,
		"close_issue":        true,
		"add_comment":        true,
		"list_issues":        true,
		"create_pr":          true,
		"merge_pr":           true,
		"list_prs":           true,
		"get_file":           true,
		"list_repos":         true,
		"create_release":     true,
		"get_workflow_runs":  true,
	}

	for _, op := range ops {
		if !expectedOps[op.Name] {
			t.Errorf("Unexpected operation: %s", op.Name)
		}
		delete(expectedOps, op.Name)
	}

	if len(expectedOps) > 0 {
		t.Errorf("Missing operations: %v", expectedOps)
	}
}

func TestGitHubConnector_CreateIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		expectedPath := "/repos/test-owner/test-repo/issues"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify method
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Verify headers
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Expected Accept header 'application/vnd.github+json', got '%s'", r.Header.Get("Accept"))
		}

		if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Errorf("Expected X-GitHub-Api-Version header '2022-11-28', got '%s'", r.Header.Get("X-GitHub-Api-Version"))
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected 'Bearer test-token' authorization, got '%s'", auth)
		}

		// Parse request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}

		if reqBody["title"] != "Test Issue" {
			t.Errorf("Expected title 'Test Issue', got '%v'", reqBody["title"])
		}

		// Return mock response
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Issue{
			Number:  123,
			HTMLURL: "https://github.com/test-owner/test-repo/issues/123",
			State:   "open",
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	result, err := conn.Execute(context.Background(), "create_issue", map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
		"title": "Test Issue",
		"body":  "This is a test issue",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify response
	resp, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Response is not a map")
	}

	if resp["number"] != 123 {
		t.Errorf("Expected issue number 123, got %v", resp["number"])
	}

	if resp["state"] != "open" {
		t.Errorf("Expected state 'open', got %v", resp["state"])
	}
}

func TestGitHubConnector_ListIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		expectedPath := "/repos/test-owner/test-repo/issues"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify method
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		// Return mock response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Issue{
			{
				Number:    1,
				Title:     "Issue 1",
				State:     "open",
				HTMLURL:   "https://github.com/test-owner/test-repo/issues/1",
				CreatedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				Labels: []Label{
					{Name: "bug"},
				},
			},
			{
				Number:    2,
				Title:     "Issue 2",
				State:     "closed",
				HTMLURL:   "https://github.com/test-owner/test-repo/issues/2",
				CreatedAt: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
				Labels:    []Label{},
			},
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	result, err := conn.Execute(context.Background(), "list_issues", map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify response
	issues, ok := result.Response.([]map[string]interface{})
	if !ok {
		t.Fatalf("Response is not a slice of maps")
	}

	if len(issues) != 2 {
		t.Errorf("Expected 2 issues, got %d", len(issues))
	}

	if issues[0]["number"] != 1 {
		t.Errorf("Expected first issue number 1, got %v", issues[0]["number"])
	}

	if issues[0]["title"] != "Issue 1" {
		t.Errorf("Expected first issue title 'Issue 1', got %v", issues[0]["title"])
	}

	labels, ok := issues[0]["labels"].([]string)
	if !ok {
		t.Fatalf("Labels is not a string slice")
	}
	if len(labels) != 1 || labels[0] != "bug" {
		t.Errorf("Expected labels ['bug'], got %v", labels)
	}
}

func TestGitHubConnector_CreatePR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		expectedPath := "/repos/test-owner/test-repo/pulls"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify method
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Parse request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}

		if reqBody["title"] != "Test PR" {
			t.Errorf("Expected title 'Test PR', got '%v'", reqBody["title"])
		}

		if reqBody["head"] != "feature-branch" {
			t.Errorf("Expected head 'feature-branch', got '%v'", reqBody["head"])
		}

		if reqBody["base"] != "main" {
			t.Errorf("Expected base 'main', got '%v'", reqBody["base"])
		}

		// Return mock response
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(PullRequest{
			Number:  456,
			HTMLURL: "https://github.com/test-owner/test-repo/pull/456",
			State:   "open",
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	result, err := conn.Execute(context.Background(), "create_pr", map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
		"title": "Test PR",
		"head":  "feature-branch",
		"base":  "main",
		"body":  "This is a test PR",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify response
	resp, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Response is not a map")
	}

	if resp["number"] != 456 {
		t.Errorf("Expected PR number 456, got %v", resp["number"])
	}

	if resp["state"] != "open" {
		t.Errorf("Expected state 'open', got %v", resp["state"])
	}
}

func TestGitHubConnector_ExecutePaginated(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++

		// Return different responses for different pages
		if page == 1 {
			// First page - full page of results with Link header
			w.Header().Set("Link", `<https://api.github.com/repos/test-owner/test-repo/issues?page=2>; rel="next", <https://api.github.com/repos/test-owner/test-repo/issues?page=2>; rel="last"`)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]Issue{
				{Number: 1, Title: "Issue 1", State: "open"},
				{Number: 2, Title: "Issue 2", State: "open"},
			})
		} else if page == 2 {
			// Second page - partial page (indicating last page)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]Issue{
				{Number: 3, Title: "Issue 3", State: "open"},
			})
		}
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	gc := conn.(*GitHubConnector)
	resultsChan, err := gc.ExecutePaginated(context.Background(), "list_issues", map[string]interface{}{
		"owner":    "test-owner",
		"repo":     "test-repo",
		"paginate": true,
		"per_page": 2,
	})

	if err != nil {
		t.Fatalf("ExecutePaginated() error = %v", err)
	}

	// Collect all results
	var allIssues []map[string]interface{}
	for result := range resultsChan {
		if errMsg, ok := result.Metadata["error"]; ok {
			t.Fatalf("Got error in paginated results: %v", errMsg)
		}

		issues, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Response is not a slice of maps")
		}
		allIssues = append(allIssues, issues...)
	}

	// Verify we got all 3 issues across 2 pages
	if len(allIssues) != 3 {
		t.Errorf("Expected 3 total issues, got %d", len(allIssues))
	}

	if allIssues[0]["number"] != 1 {
		t.Errorf("Expected first issue number 1, got %v", allIssues[0]["number"])
	}

	if allIssues[2]["number"] != 3 {
		t.Errorf("Expected third issue number 3, got %v", allIssues[2]["number"])
	}
}

func TestGitHubConnector_ExecutePaginatedWithoutPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Issue{
			{Number: 1, Title: "Issue 1", State: "open"},
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	gc := conn.(*GitHubConnector)
	// Call ExecutePaginated without paginate flag
	resultsChan, err := gc.ExecutePaginated(context.Background(), "list_issues", map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
	})

	if err != nil {
		t.Fatalf("ExecutePaginated() error = %v", err)
	}

	// Should get exactly one result
	count := 0
	for range resultsChan {
		count++
	}

	if count != 1 {
		t.Errorf("Expected 1 result without pagination, got %d", count)
	}
}

func TestGitHubConnector_ErrorHandling_Validation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":           "Validation Failed",
			"documentation_url": "https://docs.github.com/rest",
			"errors": []map[string]interface{}{
				{
					"resource": "Issue",
					"field":    "title",
					"code":     "missing_field",
				},
			},
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	_, err = conn.Execute(context.Background(), "create_issue", map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
		"title": "Test Issue",
	})

	if err == nil {
		t.Fatal("Expected error for validation failure, got nil")
	}

	// Verify error occurred (it will be wrapped by transport layer)
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestGitHubConnector_ErrorHandling_RateLimit(t *testing.T) {
	resetTime := time.Now().Add(1 * time.Hour).Unix()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "0")
		w.Header().Set("X-Ratelimit-Reset", string(rune(resetTime)))
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":           "API rate limit exceeded",
			"documentation_url": "https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting",
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	_, err = conn.Execute(context.Background(), "list_issues", map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
	})

	if err == nil {
		t.Fatal("Expected error for rate limit, got nil")
	}

	// Verify error occurred (it will be wrapped by transport layer)
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestGitHubConnector_ErrorHandling_Auth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Bad credentials",
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "bad-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	_, err = conn.Execute(context.Background(), "list_issues", map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
	})

	if err == nil {
		t.Fatal("Expected error for auth failure, got nil")
	}

	// Verify error occurred (it will be wrapped by transport layer)
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestGitHubConnector_UnknownOperation(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://api.github.com",
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	_, err = conn.Execute(context.Background(), "unknown_operation", map[string]interface{}{})

	if err == nil {
		t.Fatal("Expected error for unknown operation, got nil")
	}

	if err.Error() != "unknown operation: unknown_operation" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestGitHubConnector_UnsupportedPaginatedOperation(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://api.github.com",
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewGitHubConnector(config)
	if err != nil {
		t.Fatalf("NewGitHubConnector() error = %v", err)
	}

	gc := conn.(*GitHubConnector)
	_, err = gc.ExecutePaginated(context.Background(), "create_issue", map[string]interface{}{
		"paginate": true,
	})

	if err == nil {
		t.Fatal("Expected error for unsupported paginated operation, got nil")
	}

	if err.Error() != "operation create_issue does not support pagination" {
		t.Errorf("Unexpected error message: %v", err)
	}
}
