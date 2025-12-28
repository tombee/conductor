package discord

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/transport"
)

// createWebhook creates a webhook in a Discord channel.
func (c *DiscordConnector) createWebhook(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel_id", "name"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/channels/{channel_id}/webhooks", inputs)
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
	var webhook Webhook
	if err := c.ParseJSONResponse(resp, &webhook); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"id":         webhook.ID,
		"name":       webhook.Name,
		"channel_id": webhook.ChannelID,
		"token":      webhook.Token,
		"url":        webhook.URL,
	}), nil
}

// sendWebhook sends a message via a Discord webhook.
func (c *DiscordConnector) sendWebhook(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"webhook_id", "webhook_token"}); err != nil {
		return nil, err
	}

	// Build URL for webhook execution (doesn't use Authorization header)
	url, err := c.BuildURL("/webhooks/{webhook_id}/{webhook_token}", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body (exclude path parameters)
	body, err := c.BuildRequestBody(inputs, []string{"webhook_id", "webhook_token"})
	if err != nil {
		return nil, err
	}

	// Execute request without Authorization header (webhooks use token in URL)
	// Create a copy of default headers but without Authorization
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Use the base ExecuteRequest but without the token (webhook auth is in URL)
	// We need to call the transport directly to avoid adding the Bearer token
	req := &transport.Request{
		Method:  "POST",
		URL:     url,
		Headers: headers,
		Body:    body,
	}

	resp, err := c.transport.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Webhook execute can return either a message or 204 No Content
	if resp.StatusCode == 204 {
		return c.ToConnectorResult(resp, map[string]interface{}{
			"success": true,
		}), nil
	}

	// Parse response if message is returned
	var message Message
	if err := c.ParseJSONResponse(resp, &message); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"id":         message.ID,
		"channel_id": message.ChannelID,
		"content":    message.Content,
		"timestamp":  message.Timestamp,
	}), nil
}
