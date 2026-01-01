package slack

import (
	"context"
	"fmt"
	"net/url"

	"github.com/tombee/conductor/internal/operation"
)

// searchMessages searches for messages matching a query.
// This is essential for PWA to find mentions of a user.
func (c *SlackIntegration) searchMessages(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"query"}); err != nil {
		return nil, err
	}

	// Build query parameters
	query := url.Values{}
	query.Set("query", fmt.Sprintf("%v", inputs["query"]))

	// Optional parameters
	if count, ok := inputs["count"]; ok {
		query.Set("count", fmt.Sprintf("%v", count))
	}
	if page, ok := inputs["page"]; ok {
		query.Set("page", fmt.Sprintf("%v", page))
	}
	if sort, ok := inputs["sort"]; ok {
		query.Set("sort", fmt.Sprintf("%v", sort))
	}
	if sortDir, ok := inputs["sort_dir"]; ok {
		query.Set("sort_dir", fmt.Sprintf("%v", sortDir))
	}

	// Build URL with query params
	baseURL, err := c.BuildURL("/search.messages", nil)
	if err != nil {
		return nil, err
	}
	fullURL := baseURL + "?" + query.Encode()

	// Execute GET request (search.messages uses GET with query params)
	resp, err := c.ExecuteRequest(ctx, "GET", fullURL, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var searchResp SearchMessagesResponse
	if err := c.ParseJSONResponse(resp, &searchResp); err != nil {
		return nil, err
	}

	// Convert matches to a more usable format
	matches := make([]map[string]interface{}, len(searchResp.Messages.Matches))
	for i, match := range searchResp.Messages.Matches {
		matches[i] = map[string]interface{}{
			"type":       match.Type,
			"channel":    match.Channel.Name,
			"channel_id": match.Channel.ID,
			"user":       match.User,
			"username":   match.Username,
			"text":       match.Text,
			"timestamp":  match.Timestamp,
			"permalink":  match.Permalink,
		}
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"query":      searchResp.Query,
		"total":      searchResp.Messages.Total,
		"matches":    matches,
		"ok":         searchResp.OK,
		"page":       searchResp.Messages.Pagination.Page,
		"page_count": searchResp.Messages.Pagination.PageCount,
	}), nil
}

// openConversation opens a direct message conversation with a user.
// Useful for PWA to send messages to oneself.
func (c *SlackIntegration) openConversation(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters - need users (comma-separated user IDs)
	if err := c.ValidateRequired(inputs, []string{"users"}); err != nil {
		return nil, err
	}

	// Build URL
	urlStr, err := c.BuildURL("/conversations.open", nil)
	if err != nil {
		return nil, err
	}

	// Build request body
	body, err := c.BuildRequestBody(inputs, nil)
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", urlStr, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var openResp ConversationsOpenResponse
	if err := c.ParseJSONResponse(resp, &openResp); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"channel_id": openResp.Channel.ID,
		"ok":         openResp.OK,
	}), nil
}

// getConversationHistory retrieves messages from a conversation.
func (c *SlackIntegration) getConversationHistory(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channel"}); err != nil {
		return nil, err
	}

	// Build query parameters
	query := url.Values{}
	query.Set("channel", fmt.Sprintf("%v", inputs["channel"]))

	// Optional parameters
	if limit, ok := inputs["limit"]; ok {
		query.Set("limit", fmt.Sprintf("%v", limit))
	}
	if cursor, ok := inputs["cursor"]; ok {
		query.Set("cursor", fmt.Sprintf("%v", cursor))
	}
	if oldest, ok := inputs["oldest"]; ok {
		query.Set("oldest", fmt.Sprintf("%v", oldest))
	}
	if latest, ok := inputs["latest"]; ok {
		query.Set("latest", fmt.Sprintf("%v", latest))
	}

	// Build URL with query params
	baseURL, err := c.BuildURL("/conversations.history", nil)
	if err != nil {
		return nil, err
	}
	fullURL := baseURL + "?" + query.Encode()

	// Execute GET request
	resp, err := c.ExecuteRequest(ctx, "GET", fullURL, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var historyResp ConversationsHistoryResponse
	if err := c.ParseJSONResponse(resp, &historyResp); err != nil {
		return nil, err
	}

	// Convert messages to a more usable format
	messages := make([]map[string]interface{}, len(historyResp.Messages))
	for i, msg := range historyResp.Messages {
		messages[i] = map[string]interface{}{
			"type":      msg.Type,
			"user":      msg.User,
			"text":      msg.Text,
			"timestamp": msg.Timestamp,
			"thread_ts": msg.ThreadTS,
		}
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"messages":    messages,
		"has_more":    historyResp.HasMore,
		"ok":          historyResp.OK,
		"next_cursor": historyResp.ResponseMetadata.NextCursor,
	}), nil
}
