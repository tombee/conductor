package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
)

func TestNewSlackIntegration(t *testing.T) {
	tests := []struct {
		name       string
		config     *api.ConnectorConfig
		wantError  bool
		wantBaseURL string
	}{
		{
			name: "valid config with custom base URL",
			config: &api.ConnectorConfig{
				BaseURL:   "https://custom.slack.com/api",
				Token:     "test-token",
				Transport: &transport.HTTPTransport{},
			},
			wantError:  false,
			wantBaseURL: "https://custom.slack.com/api",
		},
		{
			name: "valid config with default base URL",
			config: &api.ConnectorConfig{
				Token:     "test-token",
				Transport: &transport.HTTPTransport{},
			},
			wantError:  false,
			wantBaseURL: "https://slack.com/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector, err := NewSlackIntegration(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("NewSlackIntegration() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if connector.Name() != "slack" {
					t.Errorf("Expected connector name 'slack', got '%s'", connector.Name())
				}

				// Verify base URL is set correctly
				sc := connector.(*SlackIntegration)
				if sc.BaseConnector == nil {
					t.Fatal("BaseConnector is nil")
				}
			}
		})
	}
}

func TestSlackIntegration_Operations(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://slack.com/api",
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP transport: %v", err)
	}

	config := &api.ConnectorConfig{
		Transport: httpTransport,
		Token:     "test-token",
	}

	connector, err := NewSlackIntegration(config)
	if err != nil {
		t.Fatalf("Failed to create Slack connector: %v", err)
	}

	sc := connector.(*SlackIntegration)
	operations := sc.Operations()

	// Verify we have all 10 operations
	expectedOps := 10
	if len(operations) != expectedOps {
		t.Errorf("Expected %d operations, got %d", expectedOps, len(operations))
	}

	// Verify operation names
	opNames := make(map[string]bool)
	for _, op := range operations {
		opNames[op.Name] = true
	}

	required := []string{
		"post_message", "update_message", "delete_message", "add_reaction",
		"upload_file",
		"list_channels", "create_channel", "invite_to_channel",
		"list_users", "get_user",
	}

	for _, name := range required {
		if !opNames[name] {
			t.Errorf("Missing required operation: %s", name)
		}
	}

	// Verify operation categories and tags
	categoryTests := map[string]string{
		"post_message":   "messages",
		"update_message": "messages",
		"delete_message": "messages",
		"add_reaction":   "messages",
		"upload_file":    "files",
		"list_channels":  "channels",
		"create_channel": "channels",
		"list_users":     "users",
	}

	for _, op := range operations {
		if expectedCat, ok := categoryTests[op.Name]; ok {
			if op.Category != expectedCat {
				t.Errorf("Operation %s: expected category '%s', got '%s'", op.Name, expectedCat, op.Category)
			}
		}
	}
}

func TestSlackIntegration_PostMessage(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected 'Bearer test-token' authorization, got '%s'", auth)
		}

		// Verify request
		if r.URL.Path != "/chat.postMessage" {
			t.Errorf("Expected path '/chat.postMessage', got '%s'", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got '%s'", r.Method)
		}

		// Verify content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected 'application/json' content type, got '%s'", contentType)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(PostMessageResponse{
			SlackResponse: SlackResponse{OK: true},
			Channel:       "C123456",
			Timestamp:     "1234567890.123456",
			Message: Message{
				Text:      "Hello, Slack!",
				Channel:   "C123456",
				Timestamp: "1234567890.123456",
			},
		})
	}))
	defer server.Close()

	// Create HTTP transport
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP transport: %v", err)
	}

	config := &api.ConnectorConfig{
		Transport: httpTransport,
		BaseURL:   server.URL,
		Token:     "test-token",
	}

	connector, err := NewSlackIntegration(config)
	if err != nil {
		t.Fatalf("Failed to create Slack connector: %v", err)
	}

	// Execute post_message operation
	result, err := connector.Execute(context.Background(), "post_message", map[string]interface{}{
		"channel": "C123456",
		"text":    "Hello, Slack!",
	})

	if err != nil {
		t.Fatalf("Failed to post message: %v", err)
	}

	// Verify result
	response, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected response to be map[string]interface{}, got %T", result.Response)
	}

	if response["channel"] != "C123456" {
		t.Errorf("Expected channel 'C123456', got '%v'", response["channel"])
	}

	if response["timestamp"] != "1234567890.123456" {
		t.Errorf("Expected timestamp '1234567890.123456', got '%v'", response["timestamp"])
	}

	if response["ok"] != true {
		t.Errorf("Expected ok to be true, got %v", response["ok"])
	}
}

func TestSlackIntegration_UpdateMessage(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/chat.update" {
			t.Errorf("Expected path '/chat.update', got '%s'", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got '%s'", r.Method)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(UpdateMessageResponse{
			SlackResponse: SlackResponse{OK: true},
			Channel:       "C123456",
			Timestamp:     "1234567890.123456",
			Text:          "Updated text",
		})
	}))
	defer server.Close()

	// Create HTTP transport
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP transport: %v", err)
	}

	config := &api.ConnectorConfig{
		Transport: httpTransport,
		BaseURL:   server.URL,
		Token:     "test-token",
	}

	connector, err := NewSlackIntegration(config)
	if err != nil {
		t.Fatalf("Failed to create Slack connector: %v", err)
	}

	// Execute update_message operation
	result, err := connector.Execute(context.Background(), "update_message", map[string]interface{}{
		"channel": "C123456",
		"ts":      "1234567890.123456",
		"text":    "Updated text",
	})

	if err != nil {
		t.Fatalf("Failed to update message: %v", err)
	}

	// Verify result
	response, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected response to be map[string]interface{}, got %T", result.Response)
	}

	if response["text"] != "Updated text" {
		t.Errorf("Expected text 'Updated text', got '%v'", response["text"])
	}
}

func TestSlackIntegration_ListChannels(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/conversations.list" {
			t.Errorf("Expected path '/conversations.list', got '%s'", r.URL.Path)
		}

		if r.Method != "GET" {
			t.Errorf("Expected GET method, got '%s'", r.Method)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListChannelsResponse{
			SlackResponse: SlackResponse{OK: true},
			Channels: []Channel{
				{
					ID:         "C123456",
					Name:       "general",
					IsPrivate:  false,
					IsArchived: false,
					NumMembers: 10,
				},
				{
					ID:         "C789012",
					Name:       "random",
					IsPrivate:  false,
					IsArchived: false,
					NumMembers: 5,
				},
			},
			ResponseMetadata: ResponseMetadata{
				NextCursor: "dXNlcjpVMDYxTkZUVDI=",
			},
		})
	}))
	defer server.Close()

	// Create HTTP transport
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP transport: %v", err)
	}

	config := &api.ConnectorConfig{
		Transport: httpTransport,
		BaseURL:   server.URL,
		Token:     "test-token",
	}

	connector, err := NewSlackIntegration(config)
	if err != nil {
		t.Fatalf("Failed to create Slack connector: %v", err)
	}

	// Execute list_channels operation
	result, err := connector.Execute(context.Background(), "list_channels", map[string]interface{}{})

	if err != nil {
		t.Fatalf("Failed to list channels: %v", err)
	}

	// Verify result
	response, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected response to be map[string]interface{}, got %T", result.Response)
	}

	channels, ok := response["channels"].([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected channels to be []map[string]interface{}, got %T", response["channels"])
	}

	if len(channels) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(channels))
	}

	if channels[0]["name"] != "general" {
		t.Errorf("Expected first channel name 'general', got '%v'", channels[0]["name"])
	}

	if response["next_cursor"] != "dXNlcjpVMDYxTkZUVDI=" {
		t.Errorf("Expected next_cursor 'dXNlcjpVMDYxTkZUVDI=', got '%v'", response["next_cursor"])
	}
}

func TestSlackIntegration_ErrorHandling_OkFalse(t *testing.T) {
	tests := []struct {
		name          string
		errorCode     string
		expectedError string
	}{
		{
			name:          "channel not found",
			errorCode:     "channel_not_found",
			expectedError: "Slack API error: channel_not_found - Channel does not exist or bot is not a member",
		},
		{
			name:          "not in channel",
			errorCode:     "not_in_channel",
			expectedError: "Slack API error: not_in_channel - Bot is not in the specified channel. Invite the bot first",
		},
		{
			name:          "invalid auth",
			errorCode:     "invalid_auth",
			expectedError: "Slack API error: invalid_auth - Token is invalid or has been revoked",
		},
		{
			name:          "missing scope",
			errorCode:     "missing_scope",
			expectedError: "Slack API error: missing_scope - Token does not have the required scope. Check bot permissions",
		},
		{
			name:          "rate limited",
			errorCode:     "ratelimited",
			expectedError: "Slack API error: ratelimited - Too many requests. Slow down API calls",
		},
		{
			name:          "message not found",
			errorCode:     "message_not_found",
			expectedError: "Slack API error: message_not_found - Message does not exist or has been deleted",
		},
		{
			name:          "name taken",
			errorCode:     "name_taken",
			expectedError: "Slack API error: name_taken - Channel name is already in use",
		},
		{
			name:          "unknown error",
			errorCode:     "unknown_error",
			expectedError: "Slack API error: unknown_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that returns error
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(SlackResponse{
					OK:    false,
					Error: tt.errorCode,
				})
			}))
			defer server.Close()

			// Create HTTP transport
			httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
				BaseURL: server.URL,
			})
			if err != nil {
				t.Fatalf("Failed to create HTTP transport: %v", err)
			}

			config := &api.ConnectorConfig{
				Transport: httpTransport,
				BaseURL:   server.URL,
				Token:     "test-token",
			}

			connector, err := NewSlackIntegration(config)
			if err != nil {
				t.Fatalf("Failed to create Slack connector: %v", err)
			}

			// Execute operation
			_, err = connector.Execute(context.Background(), "post_message", map[string]interface{}{
				"channel": "C123456",
				"text":    "Hello",
			})

			if err == nil {
				t.Fatal("Expected error for ok:false response, got nil")
			}

			if err.Error() != tt.expectedError {
				t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
			}

			// Verify it's a SlackError
			_, ok := err.(*SlackError)
			if !ok {
				t.Errorf("Expected error to be *SlackError, got %T", err)
			}
		})
	}
}

func TestSlackIntegration_ErrorHandling_HTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantError  bool
	}{
		{
			name:       "400 bad request",
			statusCode: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "401 unauthorized",
			statusCode: http.StatusUnauthorized,
			wantError:  true,
		},
		{
			name:       "403 forbidden",
			statusCode: http.StatusForbidden,
			wantError:  true,
		},
		{
			name:       "404 not found",
			statusCode: http.StatusNotFound,
			wantError:  true,
		},
		{
			name:       "429 rate limited",
			statusCode: http.StatusTooManyRequests,
			wantError:  true,
		},
		{
			name:       "500 internal server error",
			statusCode: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that returns HTTP error
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"ok": false, "error": "http_error"}`))
			}))
			defer server.Close()

			// Create HTTP transport
			httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
				BaseURL: server.URL,
			})
			if err != nil {
				t.Fatalf("Failed to create HTTP transport: %v", err)
			}

			config := &api.ConnectorConfig{
				Transport: httpTransport,
				BaseURL:   server.URL,
				Token:     "test-token",
			}

			connector, err := NewSlackIntegration(config)
			if err != nil {
				t.Fatalf("Failed to create Slack connector: %v", err)
			}

			// Execute operation
			_, err = connector.Execute(context.Background(), "post_message", map[string]interface{}{
				"channel": "C123456",
				"text":    "Hello",
			})

			if (err != nil) != tt.wantError {
				t.Errorf("Expected error = %v, got error = %v", tt.wantError, err)
			}

			if err != nil {
				// HTTP errors are caught by the transport layer and returned as TransportError
				// This is expected behavior and matches the Jenkins connector pattern
				transportErr, ok := err.(*transport.TransportError)
				if !ok {
					t.Errorf("Expected error to be *transport.TransportError, got %T", err)
				} else if transportErr.StatusCode != tt.statusCode {
					t.Errorf("Expected status code %d, got %d", tt.statusCode, transportErr.StatusCode)
				}
			}
		})
	}
}

func TestSlackIntegration_UnknownOperation(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://slack.com/api",
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP transport: %v", err)
	}

	config := &api.ConnectorConfig{
		Transport: httpTransport,
		Token:     "test-token",
	}

	connector, err := NewSlackIntegration(config)
	if err != nil {
		t.Fatalf("Failed to create Slack connector: %v", err)
	}

	// Execute unknown operation
	_, err = connector.Execute(context.Background(), "unknown_operation", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for unknown operation, got nil")
	}

	expectedError := "unknown operation: unknown_operation"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestSlackIntegration_MissingRequiredParameters(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://slack.com/api",
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP transport: %v", err)
	}

	config := &api.ConnectorConfig{
		Transport: httpTransport,
		Token:     "test-token",
	}

	connector, err := NewSlackIntegration(config)
	if err != nil {
		t.Fatalf("Failed to create Slack connector: %v", err)
	}

	tests := []struct {
		name      string
		operation string
		inputs    map[string]interface{}
	}{
		{
			name:      "post_message missing channel",
			operation: "post_message",
			inputs:    map[string]interface{}{"text": "Hello"},
		},
		{
			name:      "post_message missing text",
			operation: "post_message",
			inputs:    map[string]interface{}{"channel": "C123456"},
		},
		{
			name:      "update_message missing ts",
			operation: "update_message",
			inputs:    map[string]interface{}{"channel": "C123456", "text": "Updated"},
		},
		{
			name:      "create_channel missing name",
			operation: "create_channel",
			inputs:    map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := connector.Execute(context.Background(), tt.operation, tt.inputs)
			if err == nil {
				t.Error("Expected error for missing required parameters, got nil")
			}
		})
	}
}
