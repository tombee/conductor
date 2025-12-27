package discord

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/connector/api"
	"github.com/tombee/conductor/internal/connector/transport"
)

// DiscordConnector implements the Connector interface for Discord API.
type DiscordConnector struct {
	*api.BaseConnector
	token     string
	transport transport.Transport
	baseURL   string
}

// NewDiscordConnector creates a new Discord connector.
func NewDiscordConnector(config *api.ConnectorConfig) (connector.Connector, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://discord.com/api/v10"
	}

	base := api.NewBaseConnector("discord", config)

	return &DiscordConnector{
		BaseConnector: base,
		token:         config.Token,
		transport:     config.Transport,
		baseURL:       config.BaseURL,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *DiscordConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*connector.Result, error) {
	switch operation {
	// Messages
	case "send_message":
		return c.sendMessage(ctx, inputs)
	case "edit_message":
		return c.editMessage(ctx, inputs)
	case "delete_message":
		return c.deleteMessage(ctx, inputs)
	case "add_reaction":
		return c.addReaction(ctx, inputs)

	// Threads
	case "create_thread":
		return c.createThread(ctx, inputs)

	// Embeds
	case "send_embed":
		return c.sendEmbed(ctx, inputs)

	// Channels
	case "list_channels":
		return c.listChannels(ctx, inputs)
	case "get_channel":
		return c.getChannel(ctx, inputs)

	// Members
	case "list_members":
		return c.listMembers(ctx, inputs)
	case "get_member":
		return c.getMember(ctx, inputs)

	// Webhooks
	case "create_webhook":
		return c.createWebhook(ctx, inputs)
	case "send_webhook":
		return c.sendWebhook(ctx, inputs)

	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (c *DiscordConnector) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		// Messages
		{Name: "send_message", Description: "Send a message to a channel", Category: "messages", Tags: []string{"write"}},
		{Name: "edit_message", Description: "Edit an existing message", Category: "messages", Tags: []string{"write"}},
		{Name: "delete_message", Description: "Delete a message", Category: "messages", Tags: []string{"write", "destructive"}},
		{Name: "add_reaction", Description: "Add a reaction to a message", Category: "messages", Tags: []string{"write"}},

		// Threads
		{Name: "create_thread", Description: "Create a thread from a message", Category: "threads", Tags: []string{"write"}},

		// Embeds
		{Name: "send_embed", Description: "Send a rich embed message", Category: "embeds", Tags: []string{"write"}},

		// Channels
		{Name: "list_channels", Description: "List guild channels", Category: "channels", Tags: []string{"read"}},
		{Name: "get_channel", Description: "Get channel details", Category: "channels", Tags: []string{"read"}},

		// Members
		{Name: "list_members", Description: "List guild members", Category: "members", Tags: []string{"read", "paginated"}},
		{Name: "get_member", Description: "Get member details", Category: "members", Tags: []string{"read"}},

		// Webhooks
		{Name: "create_webhook", Description: "Create a webhook", Category: "webhooks", Tags: []string{"write"}},
		{Name: "send_webhook", Description: "Send a message via webhook", Category: "webhooks", Tags: []string{"write"}},
	}
}

// OperationSchema returns the schema for an operation.
func (c *DiscordConnector) OperationSchema(operation string) *api.OperationSchema {
	// This would return detailed schema information for each operation
	// For now, returning nil (would be implemented based on requirements)
	return nil
}

// defaultHeaders returns default headers for Discord API requests.
func (c *DiscordConnector) defaultHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}

// ExecuteRequest overrides the base implementation to use "Bot" prefix for Discord auth.
func (c *DiscordConnector) ExecuteRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (*transport.Response, error) {
	// Add Discord-specific authentication header (Bot prefix instead of Bearer)
	if c.token != "" {
		if headers == nil {
			headers = make(map[string]string)
		}
		headers["Authorization"] = "Bot " + c.token
	}

	req := &transport.Request{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}

	return c.transport.Execute(ctx, req)
}
