package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/internal/connector/api"
	"github.com/tombee/conductor/internal/connector/transport"
)

func TestNewDiscordConnector(t *testing.T) {
	// Create HTTP transport
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://discord.com/api/v10",
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP transport: %v", err)
	}

	config := &api.ConnectorConfig{
		Transport: httpTransport,
		Token:     "test-token",
	}

	connector, err := NewDiscordConnector(config)
	if err != nil {
		t.Fatalf("Failed to create Discord connector: %v", err)
	}

	if connector.Name() != "discord" {
		t.Errorf("Expected connector name 'discord', got '%s'", connector.Name())
	}

	// Verify default base URL is set
	dc := connector.(*DiscordConnector)
	if dc.baseURL != "https://discord.com/api/v10" {
		t.Errorf("Expected default base URL 'https://discord.com/api/v10', got '%s'", dc.baseURL)
	}
}

func TestDiscordConnectorOperations(t *testing.T) {
	// Create HTTP transport
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://discord.com/api/v10",
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP transport: %v", err)
	}

	config := &api.ConnectorConfig{
		Transport: httpTransport,
		Token:     "test-token",
	}

	connector, err := NewDiscordConnector(config)
	if err != nil {
		t.Fatalf("Failed to create Discord connector: %v", err)
	}

	dc := connector.(*DiscordConnector)
	operations := dc.Operations()

	// Verify we have all 12 operations
	expectedOps := 12
	if len(operations) != expectedOps {
		t.Errorf("Expected %d operations, got %d", expectedOps, len(operations))
	}

	// Verify operation names
	opNames := make(map[string]bool)
	for _, op := range operations {
		opNames[op.Name] = true
	}

	required := []string{
		"send_message", "edit_message", "delete_message", "add_reaction",
		"create_thread", "send_embed",
		"list_channels", "get_channel",
		"list_members", "get_member",
		"create_webhook", "send_webhook",
	}

	for _, name := range required {
		if !opNames[name] {
			t.Errorf("Missing required operation: %s", name)
		}
	}
}

func TestSendMessage(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header uses Bot prefix
		auth := r.Header.Get("Authorization")
		if auth != "Bot test-token" {
			t.Errorf("Expected 'Bot test-token' authorization, got '%s'", auth)
		}

		// Verify request
		if r.URL.Path != "/channels/123456/messages" {
			t.Errorf("Expected path '/channels/123456/messages', got '%s'", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got '%s'", r.Method)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "789012",
			"channel_id": "123456",
			"content": "Hello, Discord!",
			"timestamp": "2023-01-01T00:00:00.000Z"
		}`))
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

	connector, err := NewDiscordConnector(config)
	if err != nil {
		t.Fatalf("Failed to create Discord connector: %v", err)
	}

	// Execute send_message operation
	result, err := connector.Execute(context.Background(), "send_message", map[string]interface{}{
		"channel_id": "123456",
		"content":    "Hello, Discord!",
	})

	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Verify result
	response, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected response to be map[string]interface{}, got %T", result.Response)
	}

	if response["id"] != "789012" {
		t.Errorf("Expected message ID '789012', got '%v'", response["id"])
	}

	if response["content"] != "Hello, Discord!" {
		t.Errorf("Expected content 'Hello, Discord!', got '%v'", response["content"])
	}
}

func TestUnknownOperation(t *testing.T) {
	// Create HTTP transport
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://discord.com/api/v10",
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP transport: %v", err)
	}

	config := &api.ConnectorConfig{
		Transport: httpTransport,
		Token:     "test-token",
	}

	connector, err := NewDiscordConnector(config)
	if err != nil {
		t.Fatalf("Failed to create Discord connector: %v", err)
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
