package discord

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// listChannels lists all channels in a Discord guild.
func (c *DiscordIntegration) listChannels(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"guild_id"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/guilds/{guild_id}/channels", inputs)
	if err != nil {
		return nil, err
	}

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
	var channels []Channel
	if err := c.ParseJSONResponse(resp, &channels); err != nil {
		return nil, err
	}

	// Convert to simplified response
	channelList := make([]map[string]interface{}, len(channels))
	for i, ch := range channels {
		channelList[i] = map[string]interface{}{
			"id":       ch.ID,
			"name":     ch.Name,
			"type":     ch.Type,
			"position": ch.Position,
			"topic":    ch.Topic,
		}
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"channels": channelList,
	}), nil
}

// getChannel gets details about a specific Discord channel.
func (c *DiscordIntegration) getChannel(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel_id"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/channels/{channel_id}", inputs)
	if err != nil {
		return nil, err
	}

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
	var channel Channel
	if err := c.ParseJSONResponse(resp, &channel); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"id":       channel.ID,
		"name":     channel.Name,
		"type":     channel.Type,
		"guild_id": channel.GuildID,
		"position": channel.Position,
		"topic":    channel.Topic,
	}), nil
}
