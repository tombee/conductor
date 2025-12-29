package discord

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
)

// sendMessage sends a message to a Discord channel.
func (c *DiscordIntegration) sendMessage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel_id", "content"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/channels/{channel_id}/messages", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body (exclude path parameters)
	body, err := c.BuildRequestBody(inputs, []string{"channel_id"})
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var message Message
	if err := c.ParseJSONResponse(resp, &message); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"id":         message.ID,
		"channel_id": message.ChannelID,
		"content":    message.Content,
		"timestamp":  message.Timestamp,
	}), nil
}

// editMessage edits an existing Discord message.
func (c *DiscordIntegration) editMessage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel_id", "message_id"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/channels/{channel_id}/messages/{message_id}", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body (exclude path parameters)
	body, err := c.BuildRequestBody(inputs, []string{"channel_id", "message_id"})
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "PATCH", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var message Message
	if err := c.ParseJSONResponse(resp, &message); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"id":               message.ID,
		"channel_id":       message.ChannelID,
		"content":          message.Content,
		"edited_timestamp": message.EditedTimestamp,
	}), nil
}

// deleteMessage deletes a Discord message.
func (c *DiscordIntegration) deleteMessage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel_id", "message_id"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/channels/{channel_id}/messages/{message_id}", inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "DELETE", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Return operation result (DELETE returns 204 No Content)
	return c.ToConnectorResult(resp, map[string]interface{}{
		"success": true,
	}), nil
}

// addReaction adds a reaction to a Discord message.
func (c *DiscordIntegration) addReaction(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel_id", "message_id", "emoji"}); err != nil {
		return nil, err
	}

	// Get emoji and URL-encode it
	emoji, ok := inputs["emoji"].(string)
	if !ok {
		return nil, fmt.Errorf("emoji must be a string")
	}

	// Build URL with emoji (Discord uses emoji as part of the path)
	// For custom emojis: name:id, for Unicode: the emoji itself
	url := fmt.Sprintf("%s/channels/%v/messages/%v/reactions/%s/@me",
		c.baseURL,
		inputs["channel_id"],
		inputs["message_id"],
		emoji)

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "PUT", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Return operation result (PUT returns 204 No Content)
	return c.ToConnectorResult(resp, map[string]interface{}{
		"success": true,
	}), nil
}

// createThread creates a thread from a message in a Discord channel.
func (c *DiscordIntegration) createThread(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel_id", "message_id", "name"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/channels/{channel_id}/messages/{message_id}/threads", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body (exclude path parameters)
	body, err := c.BuildRequestBody(inputs, []string{"channel_id", "message_id"})
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var thread Thread
	if err := c.ParseJSONResponse(resp, &thread); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"id":        thread.ID,
		"name":      thread.Name,
		"parent_id": thread.ParentID,
		"owner_id":  thread.OwnerID,
	}), nil
}

// sendEmbed sends a rich embed message to a Discord channel.
func (c *DiscordIntegration) sendEmbed(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel_id"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/channels/{channel_id}/messages", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body with embed
	// The inputs should contain "embeds" array or individual embed fields
	body, err := c.BuildRequestBody(inputs, []string{"channel_id"})
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var message Message
	if err := c.ParseJSONResponse(resp, &message); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"id":         message.ID,
		"channel_id": message.ChannelID,
		"embeds":     message.Embeds,
		"timestamp":  message.Timestamp,
	}), nil
}
