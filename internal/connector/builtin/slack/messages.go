package slack

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// postMessage sends a message to a Slack channel.
func (c *SlackConnector) postMessage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel", "text"}); err != nil {
		return nil, err
	}

	// Build URL - Slack doesn't use path params, just append the endpoint
	url, err := c.BuildURL("/chat.postMessage", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	body, err := c.BuildRequestBody(inputs, nil)
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
	var msgResp PostMessageResponse
	if err := c.ParseJSONResponse(resp, &msgResp); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"channel":   msgResp.Channel,
		"timestamp": msgResp.Timestamp,
		"ok":        msgResp.OK,
	}), nil
}

// updateMessage updates an existing Slack message.
func (c *SlackConnector) updateMessage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel", "ts", "text"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/chat.update", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	body, err := c.BuildRequestBody(inputs, nil)
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
	var updateResp UpdateMessageResponse
	if err := c.ParseJSONResponse(resp, &updateResp); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"channel":   updateResp.Channel,
		"timestamp": updateResp.Timestamp,
		"text":      updateResp.Text,
		"ok":        updateResp.OK,
	}), nil
}

// deleteMessage deletes a Slack message.
func (c *SlackConnector) deleteMessage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel", "ts"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/chat.delete", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	body, err := c.BuildRequestBody(inputs, nil)
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
	var deleteResp DeleteMessageResponse
	if err := c.ParseJSONResponse(resp, &deleteResp); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"channel":   deleteResp.Channel,
		"timestamp": deleteResp.Timestamp,
		"ok":        deleteResp.OK,
	}), nil
}

// addReaction adds a reaction emoji to a message.
func (c *SlackConnector) addReaction(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel", "timestamp", "name"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/reactions.add", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	body, err := c.BuildRequestBody(inputs, nil)
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
	var reactionResp ReactionResponse
	if err := c.ParseJSONResponse(resp, &reactionResp); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"ok": reactionResp.OK,
	}), nil
}
