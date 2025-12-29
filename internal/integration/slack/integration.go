package slack

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
)

// SlackIntegration implements the Provider interface for Slack API.
type SlackIntegration struct {
	*api.BaseProvider
}

// NewSlackIntegration creates a new Slack integration.
func NewSlackIntegration(config *api.ProviderConfig) (operation.Provider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://slack.com/api"
	}

	base := api.NewBaseProvider("slack", config)

	return &SlackIntegration{
		BaseProvider: base,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *SlackIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
	switch operation {
	// Messages
	case "post_message":
		return c.postMessage(ctx, inputs)
	case "update_message":
		return c.updateMessage(ctx, inputs)
	case "delete_message":
		return c.deleteMessage(ctx, inputs)
	case "add_reaction":
		return c.addReaction(ctx, inputs)

	// Files
	case "upload_file":
		return c.uploadFile(ctx, inputs)

	// Channels
	case "list_channels":
		return c.listChannels(ctx, inputs)
	case "create_channel":
		return c.createChannel(ctx, inputs)
	case "invite_to_channel":
		return c.inviteToChannel(ctx, inputs)

	// Users
	case "list_users":
		return c.listUsers(ctx, inputs)
	case "get_user":
		return c.getUser(ctx, inputs)

	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (c *SlackIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		// Messages
		{Name: "post_message", Description: "Send a message to a channel", Category: "messages", Tags: []string{"write"}},
		{Name: "update_message", Description: "Update an existing message", Category: "messages", Tags: []string{"write"}},
		{Name: "delete_message", Description: "Delete a message", Category: "messages", Tags: []string{"write", "destructive"}},
		{Name: "add_reaction", Description: "Add a reaction to a message", Category: "messages", Tags: []string{"write"}},

		// Files
		{Name: "upload_file", Description: "Upload a file to channels", Category: "files", Tags: []string{"write"}},

		// Channels
		{Name: "list_channels", Description: "List conversations", Category: "channels", Tags: []string{"read", "paginated"}},
		{Name: "create_channel", Description: "Create a new channel", Category: "channels", Tags: []string{"write"}},
		{Name: "invite_to_channel", Description: "Invite users to a channel", Category: "channels", Tags: []string{"write"}},

		// Users
		{Name: "list_users", Description: "List workspace members", Category: "users", Tags: []string{"read", "paginated"}},
		{Name: "get_user", Description: "Get user information", Category: "users", Tags: []string{"read"}},
	}
}

// OperationSchema returns the schema for an operation.
func (c *SlackIntegration) OperationSchema(operation string) *api.OperationSchema {
	// This would return detailed schema information for each operation
	// For now, returning nil (would be implemented based on requirements)
	return nil
}

// defaultHeaders returns default headers for Slack API requests.
func (c *SlackIntegration) defaultHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}
