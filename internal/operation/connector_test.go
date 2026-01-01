package operation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestHTTPIntegration_Execute(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", auth)
		}

		// Return test response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     123,
			"title":  "Test Issue",
			"number": 42,
		})
	}))
	defer server.Close()

	// Create operation definition
	def := &workflow.IntegrationDefinition{
		Name:    "test",
		BaseURL: server.URL,
		Auth: &workflow.AuthDefinition{
			Type:  "bearer",
			Token: "test-token",
		},
		Operations: map[string]workflow.OperationDefinition{
			"create_issue": {
				Method: "POST",
				Path:   "/repos/{owner}/{repo}/issues",
				RequestSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"owner": map[string]interface{}{"type": "string"},
						"repo":  map[string]interface{}{"type": "string"},
						"title": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"owner", "repo", "title"},
				},
				ResponseTransform: ".number",
			},
		},
	}

	// Create operation (allow localhost for testing)
	config := DefaultConfig()
	config.AllowedHosts = []string{"127.0.0.1", "localhost"}
	op, err := New(def, config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Execute operation
	result, err := op.Execute(context.Background(), "create_issue", map[string]interface{}{
		"owner": "test-org",
		"repo":  "test-repo",
		"title": "Test Issue",
	})

	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Verify result
	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}

	// Response should be transformed to just the number
	responseNum, ok := result.Response.(float64)
	if !ok {
		t.Fatalf("expected number response, got %T: %v", result.Response, result.Response)
	}

	if responseNum != 42 {
		t.Errorf("expected response 42, got %v", responseNum)
	}

	// Raw response should have full object
	if result.RawResponse == nil {
		t.Error("expected raw response to be present")
	}
}

func TestHTTPIntegration_AuthTypes(t *testing.T) {
	tests := []struct {
		name     string
		auth     *workflow.AuthDefinition
		wantAuth string
	}{
		{
			name: "bearer token",
			auth: &workflow.AuthDefinition{
				Type:  "bearer",
				Token: "test-token",
			},
			wantAuth: "Bearer test-token",
		},
		{
			name: "bearer token (inferred)",
			auth: &workflow.AuthDefinition{
				Token: "test-token",
			},
			wantAuth: "Bearer test-token",
		},
		{
			name: "basic auth",
			auth: &workflow.AuthDefinition{
				Type:     "basic",
				Username: "user",
				Password: "pass",
			},
			wantAuth: "Basic dXNlcjpwYXNz", // base64(user:pass)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			var gotAuth string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			}))
			defer server.Close()

			// Create operation
			def := &workflow.IntegrationDefinition{
				Name:    "test",
				BaseURL: server.URL,
				Auth:    tt.auth,
				Operations: map[string]workflow.OperationDefinition{
					"test": {
						Method: "GET",
						Path:   "/test",
					},
				},
			}

			config := DefaultConfig()
			config.AllowedHosts = []string{"127.0.0.1", "localhost"}
			op, err := New(def, config)
			if err != nil {
				t.Fatalf("failed to create provider: %v", err)
			}

			// Execute
			_, err = op.Execute(context.Background(), "test", map[string]interface{}{})
			if err != nil {
				t.Fatalf("execution failed: %v", err)
			}

			// Verify auth header
			if gotAuth != tt.wantAuth {
				t.Errorf("expected auth %q, got %q", tt.wantAuth, gotAuth)
			}
		})
	}
}

func TestHTTPIntegration_PathParameters(t *testing.T) {
	// Create test server
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	// Create operation
	def := &workflow.IntegrationDefinition{
		Name:    "test",
		BaseURL: server.URL,
		Operations: map[string]workflow.OperationDefinition{
			"test": {
				Method: "GET",
				Path:   "/repos/{owner}/{repo}/issues/{number}",
			},
		},
	}

	config := DefaultConfig()
	config.AllowedHosts = []string{"127.0.0.1", "localhost"}
	op, err := New(def, config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Execute with path parameters
	_, err = op.Execute(context.Background(), "test", map[string]interface{}{
		"owner":  "test-org",
		"repo":   "test-repo",
		"number": 42,
	})

	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Verify path was correctly substituted
	wantPath := "/repos/test-org/test-repo/issues/42"
	if gotPath != wantPath {
		t.Errorf("expected path %q, got %q", wantPath, gotPath)
	}
}

func TestHTTPIntegration_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		wantErrorType ErrorType
		wantRetryable bool
	}{
		{
			name:          "401 Unauthorized",
			statusCode:    401,
			wantErrorType: ErrorTypeAuth,
			wantRetryable: false,
		},
		{
			name:          "404 Not Found",
			statusCode:    404,
			wantErrorType: ErrorTypeNotFound,
			wantRetryable: false,
		},
		{
			name:          "429 Rate Limited",
			statusCode:    429,
			wantErrorType: ErrorTypeRateLimit,
			wantRetryable: true,
		},
		{
			name:          "500 Server Error",
			statusCode:    500,
			wantErrorType: ErrorTypeServer,
			wantRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(map[string]string{"error": "test error"})
			}))
			defer server.Close()

			// Create operation
			def := &workflow.IntegrationDefinition{
				Name:    "test",
				BaseURL: server.URL,
				Operations: map[string]workflow.OperationDefinition{
					"test": {
						Method: "GET",
						Path:   "/test",
					},
				},
			}

			config := DefaultConfig()
			config.AllowedHosts = []string{"127.0.0.1", "localhost"}
			op, err := New(def, config)
			if err != nil {
				t.Fatalf("failed to create provider: %v", err)
			}

			// Execute (should fail)
			_, err = op.Execute(context.Background(), "test", map[string]interface{}{})

			// Verify error
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			connErr, ok := err.(*Error)
			if !ok {
				t.Fatalf("expected *Error, got %T", err)
			}

			if connErr.Type != tt.wantErrorType {
				t.Errorf("expected error type %s, got %s", tt.wantErrorType, connErr.Type)
			}

			if connErr.IsRetryable() != tt.wantRetryable {
				t.Errorf("expected retryable=%v, got %v", tt.wantRetryable, connErr.IsRetryable())
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	// Create workflow definition with operations
	def := &workflow.Definition{
		Name: "test-workflow",
		Integrations: map[string]workflow.IntegrationDefinition{
			"github": {
				Name:    "github",
				BaseURL: "https://api.github.com",
				Auth: &workflow.AuthDefinition{
					Token: "test-token",
				},
				Operations: map[string]workflow.OperationDefinition{
					"get_user": {
						Method: "GET",
						Path:   "/users/{username}",
					},
				},
			},
			"slack": {
				Name:    "slack",
				BaseURL: "https://slack.com/api",
				Auth: &workflow.AuthDefinition{
					Token: "slack-token",
				},
				Operations: map[string]workflow.OperationDefinition{
					"post_message": {
						Method: "POST",
						Path:   "/chat.postMessage",
					},
				},
			},
		},
	}

	// Create registry
	registry := NewRegistry(DefaultConfig())

	// Load operations
	err := registry.LoadFromDefinition(def)
	if err != nil {
		t.Fatalf("failed to load operations: %v", err)
	}

	// Test List
	names := registry.List()
	if len(names) != 2 {
		t.Errorf("expected 2 operations, got %d", len(names))
	}

	// Test Get
	op, err := registry.Get("github")
	if err != nil {
		t.Fatalf("failed to get provider: %v", err)
	}

	if op.Name() != "github" {
		t.Errorf("expected name 'github', got %s", op.Name())
	}

	// Test Get non-existent
	_, err = registry.Get("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent operation")
	}
}

func TestParseReference(t *testing.T) {
	tests := []struct {
		name            string
		reference       string
		wantIntegration string
		wantOperation   string
		wantError       bool
	}{
		{
			name:            "valid reference",
			reference:       "github.create_issue",
			wantIntegration: "github",
			wantOperation:   "create_issue",
			wantError:       false,
		},
		{
			name:            "valid reference with underscores",
			reference:       "my_integration.my_operation",
			wantIntegration: "my_integration",
			wantOperation:   "my_operation",
			wantError:       false,
		},
		{
			name:      "missing dot",
			reference: "githubcreate_issue",
			wantError: true,
		},
		{
			name:      "empty operation",
			reference: ".create_issue",
			wantError: true,
		},
		{
			name:      "empty operation",
			reference: "github.",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opName, operation, err := parseReference(tt.reference)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if opName != tt.wantIntegration {
				t.Errorf("expected integration %q, got %q", tt.wantIntegration, opName)
			}

			if operation != tt.wantOperation {
				t.Errorf("expected operation %q, got %q", tt.wantOperation, operation)
			}
		})
	}
}

func TestNewBuiltinRegistry(t *testing.T) {
	config := &BuiltinConfig{
		WorkflowDir: "/tmp/test",
	}

	registry, err := NewBuiltinRegistry(config)
	if err != nil {
		t.Fatalf("NewBuiltinRegistry failed: %v", err)
	}

	// Verify all builtin operations are registered
	names := registry.List()
	expected := []string{"file", "shell", "transform", "utility", "http"}

	if len(names) != len(expected) {
		t.Errorf("expected %d operations, got %d", len(expected), len(names))
	}

	// Check each builtin exists
	for _, name := range expected {
		op, err := registry.Get(name)
		if err != nil {
			t.Errorf("expected operation %q to exist: %v", name, err)
			continue
		}
		if op.Name() != name {
			t.Errorf("expected operation name %q, got %q", name, op.Name())
		}
	}
}

func TestBuiltinAction_ShellExecution(t *testing.T) {
	// Use a directory that actually exists
	config := &BuiltinConfig{
		WorkflowDir: "/tmp",
	}

	registry, err := NewBuiltinRegistry(config)
	if err != nil {
		t.Fatalf("NewBuiltinRegistry failed: %v", err)
	}

	// Execute a shell command through the registry
	result, err := registry.Execute(context.Background(), "shell.run", map[string]interface{}{
		"command": []string{"echo", "hello from shell"},
	})
	if err != nil {
		t.Fatalf("shell.run execution failed: %v", err)
	}

	response, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", result.Response)
	}

	stdout, ok := response["stdout"].(string)
	if !ok {
		t.Fatalf("expected stdout string, got %T", response["stdout"])
	}

	if stdout != "hello from shell" {
		t.Errorf("expected 'hello from shell', got %q", stdout)
	}
}

func TestBuiltinAction_FileExecution(t *testing.T) {
	config := &BuiltinConfig{
		WorkflowDir:   "/tmp",
		AllowAbsolute: true, // Allow absolute paths for this test
	}

	registry, err := NewBuiltinRegistry(config)
	if err != nil {
		t.Fatalf("NewBuiltinRegistry failed: %v", err)
	}

	// Check if a file exists (should return false for non-existent file)
	result, err := registry.Execute(context.Background(), "file.exists", map[string]interface{}{
		"path": "/tmp/nonexistent-test-file-xyz123",
	})
	if err != nil {
		t.Fatalf("file.exists execution failed: %v", err)
	}

	exists := result.Response.(bool)
	if exists {
		t.Error("expected file to not exist")
	}
}

func TestRegistryAsWorkflowRegistry(t *testing.T) {
	config := &BuiltinConfig{
		WorkflowDir: "/tmp", // Use existing directory
	}

	registry, err := NewBuiltinRegistry(config)
	if err != nil {
		t.Fatalf("NewBuiltinRegistry failed: %v", err)
	}

	// Get the workflow.OperationRegistry interface
	workflowRegistry := registry.AsWorkflowRegistry()

	// Execute through the interface
	result, err := workflowRegistry.Execute(context.Background(), "shell.run", map[string]interface{}{
		"command": []string{"echo", "interface test"},
	})
	if err != nil {
		t.Fatalf("execution through interface failed: %v", err)
	}

	// Verify result implements OperationResult interface
	response := result.GetResponse()
	if response == nil {
		t.Error("expected non-nil response")
	}

	responseMap := response.(map[string]interface{})
	stdout := responseMap["stdout"].(string)
	if stdout != "interface test" {
		t.Errorf("expected 'interface test', got %q", stdout)
	}
}
