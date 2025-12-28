package slack

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// listChannels lists conversations (channels) in the workspace.
func (c *SlackConnector) listChannels(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Build URL
	url, err := c.BuildURL("/conversations.list", inputs)
	if err != nil {
		return nil, err
	}

	// Build query string from inputs (types, exclude_archived, cursor, limit)
	queryString := c.BuildQueryString(inputs, nil)
	url += queryString

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var listResp ListChannelsResponse
	if err := c.ParseJSONResponse(resp, &listResp); err != nil {
		return nil, err
	}

	// Transform to simplified format
	channels := make([]map[string]interface{}, len(listResp.Channels))
	for i, channel := range listResp.Channels {
		channels[i] = map[string]interface{}{
			"id":          channel.ID,
			"name":        channel.Name,
			"is_private":  channel.IsPrivate,
			"is_archived": channel.IsArchived,
			"num_members": channel.NumMembers,
		}
	}

	result := map[string]interface{}{
		"channels": channels,
		"ok":       listResp.OK,
	}

	// Include next cursor if available for pagination
	if listResp.ResponseMetadata.NextCursor != "" {
		result["next_cursor"] = listResp.ResponseMetadata.NextCursor
	}

	return c.ToConnectorResult(resp, result), nil
}

// createChannel creates a new Slack channel.
func (c *SlackConnector) createChannel(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"name"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/conversations.create", inputs)
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
	var createResp CreateChannelResponse
	if err := c.ParseJSONResponse(resp, &createResp); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"channel_id":  createResp.Channel.ID,
		"channel_name": createResp.Channel.Name,
		"is_private":  createResp.Channel.IsPrivate,
		"ok":          createResp.OK,
	}), nil
}

// inviteToChannel invites users to a channel.
func (c *SlackConnector) inviteToChannel(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel", "users"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/conversations.invite", inputs)
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
	var inviteResp InviteToChannelResponse
	if err := c.ParseJSONResponse(resp, &inviteResp); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"channel_id": inviteResp.Channel.ID,
		"ok":         inviteResp.OK,
	}), nil
}
