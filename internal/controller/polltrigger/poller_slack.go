package polltrigger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SlackPoller implements polling for Slack conversations API.
type SlackPoller struct {
	botToken string
	baseURL  string
	client   *http.Client
}

// NewSlackPoller creates a new Slack poller.
func NewSlackPoller(botToken string) *SlackPoller {
	return &SlackPoller{
		botToken: botToken,
		baseURL:  "https://slack.com/api",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the integration name.
func (p *SlackPoller) Name() string {
	return "slack"
}

// Poll queries the Slack conversations API for new messages since the last poll time.
// Supports query parameters: mentions, channels, include_dms, include_threads, exclude_bots
func (p *SlackPoller) Poll(ctx context.Context, state *PollState, query map[string]interface{}) ([]map[string]interface{}, string, error) {
	// Get channels to monitor
	channels, err := p.getChannelsToMonitor(ctx, query)
	if err != nil {
		return nil, "", err
	}

	if len(channels) == 0 {
		return nil, "", fmt.Errorf("no channels found to monitor")
	}

	// Extract mentions filter (username to search for)
	mentions := ""
	if m, ok := query["mentions"].(string); ok && m != "" {
		mentions = strings.TrimPrefix(m, "@")
	}

	// Extract options
	includeThreads := true
	if it, ok := query["include_threads"].(bool); ok {
		includeThreads = it
	}

	excludeBots := true
	if eb, ok := query["exclude_bots"].(bool); ok {
		excludeBots = eb
	}

	// Collect events from all channels
	var allEvents []map[string]interface{}

	// Calculate oldest timestamp from last poll time
	oldest := "0"
	if !state.LastPollTime.IsZero() {
		oldest = fmt.Sprintf("%.6f", float64(state.LastPollTime.Unix()))
	}

	for _, channel := range channels {
		// Get conversation history
		messages, err := p.getConversationHistory(ctx, channel, oldest)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get history for channel %s: %w", channel, err)
		}

		// Filter messages
		for _, msg := range messages {
			// Filter by mentions if specified
			if mentions != "" && !p.messageContainsMention(msg, mentions) {
				continue
			}

			// Exclude bots if requested
			if excludeBots {
				if botID, ok := msg["bot_id"].(string); ok && botID != "" {
					continue
				}
				if subtype, ok := msg["subtype"].(string); ok && subtype == "bot_message" {
					continue
				}
			}

			// Build event from message
			event := p.messageToEvent(msg, channel)
			allEvents = append(allEvents, event)
		}

		// Include thread replies if requested
		if includeThreads {
			threadEvents, err := p.getThreadReplies(ctx, channel, messages, oldest, mentions, excludeBots)
			if err != nil {
				return nil, "", fmt.Errorf("failed to get thread replies for channel %s: %w", channel, err)
			}
			allEvents = append(allEvents, threadEvents...)
		}
	}

	// Slack doesn't use cursor-based pagination for this use case
	return allEvents, "", nil
}

// getChannelsToMonitor returns the list of channel IDs to monitor based on query parameters.
func (p *SlackPoller) getChannelsToMonitor(ctx context.Context, query map[string]interface{}) ([]string, error) {
	// If channels are explicitly specified, use those
	if channelList, ok := query["channels"].([]interface{}); ok && len(channelList) > 0 {
		channels := make([]string, 0, len(channelList))
		for _, ch := range channelList {
			if chStr, ok := ch.(string); ok {
				channels = append(channels, chStr)
			}
		}
		return channels, nil
	}

	// Otherwise, list all conversations the bot has access to
	includeDMs := false
	if dm, ok := query["include_dms"].(bool); ok {
		includeDMs = dm
	}

	return p.listConversations(ctx, includeDMs)
}

// listConversations retrieves the list of conversations the bot has access to.
func (p *SlackPoller) listConversations(ctx context.Context, includeDMs bool) ([]string, error) {
	params := url.Values{}
	params.Set("types", "public_channel,private_channel")
	if includeDMs {
		params.Set("types", "public_channel,private_channel,im,mpim")
	}
	params.Set("exclude_archived", "true")
	params.Set("limit", "200")

	apiURL := fmt.Sprintf("%s/conversations.list?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, wrapAPIError(err, "slack")
	}
	defer resp.Body.Close()

	if err := p.checkStatusCode(resp); err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var listResp slackConversationsListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !listResp.OK {
		return nil, fmt.Errorf("Slack API error: %s", listResp.Error)
	}

	channels := make([]string, 0, len(listResp.Channels))
	for _, ch := range listResp.Channels {
		channels = append(channels, ch.ID)
	}

	return channels, nil
}

// getConversationHistory retrieves messages from a channel since the given timestamp.
func (p *SlackPoller) getConversationHistory(ctx context.Context, channel, oldest string) ([]map[string]interface{}, error) {
	params := url.Values{}
	params.Set("channel", channel)
	params.Set("oldest", oldest)
	params.Set("limit", "100")

	apiURL := fmt.Sprintf("%s/conversations.history?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, wrapAPIError(err, "slack")
	}
	defer resp.Body.Close()

	if err := p.checkStatusCode(resp); err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var historyResp slackConversationsHistoryResponse
	if err := json.Unmarshal(body, &historyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !historyResp.OK {
		return nil, fmt.Errorf("Slack API error: %s", historyResp.Error)
	}

	return historyResp.Messages, nil
}

// getThreadReplies retrieves replies from threads for messages that have threads.
func (p *SlackPoller) getThreadReplies(ctx context.Context, channel string, messages []map[string]interface{}, oldest, mentions string, excludeBots bool) ([]map[string]interface{}, error) {
	var threadEvents []map[string]interface{}

	for _, msg := range messages {
		// Check if message has a thread
		threadTS, hasThread := msg["thread_ts"].(string)
		replyCount := 0
		if rc, ok := msg["reply_count"].(float64); ok {
			replyCount = int(rc)
		}

		if !hasThread || replyCount == 0 {
			continue
		}

		// Get thread replies
		params := url.Values{}
		params.Set("channel", channel)
		params.Set("ts", threadTS)
		params.Set("oldest", oldest)
		params.Set("limit", "100")

		apiURL := fmt.Sprintf("%s/conversations.replies?%s", p.baseURL, params.Encode())

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+p.botToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, wrapAPIError(err, "slack")
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		var repliesResp slackConversationsHistoryResponse
		if err := json.Unmarshal(body, &repliesResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		if !repliesResp.OK {
			return nil, fmt.Errorf("Slack API error: %s", repliesResp.Error)
		}

		// Process replies (skip first message which is the parent)
		for i, reply := range repliesResp.Messages {
			if i == 0 {
				continue // Skip parent message
			}

			// Filter by mentions if specified
			if mentions != "" && !p.messageContainsMention(reply, mentions) {
				continue
			}

			// Exclude bots if requested
			if excludeBots {
				if botID, ok := reply["bot_id"].(string); ok && botID != "" {
					continue
				}
				if subtype, ok := reply["subtype"].(string); ok && subtype == "bot_message" {
					continue
				}
			}

			event := p.messageToEvent(reply, channel)
			event["is_thread_reply"] = true
			threadEvents = append(threadEvents, event)
		}
	}

	return threadEvents, nil
}

// messageContainsMention checks if a message contains a mention of the given username.
func (p *SlackPoller) messageContainsMention(msg map[string]interface{}, username string) bool {
	text, ok := msg["text"].(string)
	if !ok {
		return false
	}

	// Check for @username or <@USERID> format
	return strings.Contains(text, "@"+username) || strings.Contains(text, "<@")
}

// messageToEvent converts a Slack message to a generic event map.
func (p *SlackPoller) messageToEvent(msg map[string]interface{}, channel string) map[string]interface{} {
	event := map[string]interface{}{
		"channel": channel,
	}

	// Copy relevant fields
	if ts, ok := msg["ts"].(string); ok {
		event["timestamp"] = ts
	}
	if user, ok := msg["user"].(string); ok {
		event["user"] = user
	}
	if text, ok := msg["text"].(string); ok {
		event["text"] = text
	}
	if msgType, ok := msg["type"].(string); ok {
		event["type"] = msgType
	}
	if threadTS, ok := msg["thread_ts"].(string); ok {
		event["thread_ts"] = threadTS
	}

	// Generate event ID from channel + timestamp
	if ts, ok := msg["ts"].(string); ok {
		event["id"] = fmt.Sprintf("%s:%s", channel, ts)
	}

	return event
}

// checkStatusCode validates the HTTP response status code.
func (p *SlackPoller) checkStatusCode(resp *http.Response) error {
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("Slack auth failed (%d). Bot token may be invalid or revoked", resp.StatusCode)
	}
	if resp.StatusCode == 429 {
		return fmt.Errorf("Slack rate limit exceeded (429)")
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("Slack API error (%d)", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Slack API returned status %d", resp.StatusCode)
	}
	return nil
}

// Slack API response types

type slackConversationsListResponse struct {
	OK       bool           `json:"ok"`
	Channels []slackChannel `json:"channels"`
	Error    string         `json:"error,omitempty"`
}

type slackChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type slackConversationsHistoryResponse struct {
	OK       bool                     `json:"ok"`
	Messages []map[string]interface{} `json:"messages"`
	Error    string                   `json:"error,omitempty"`
}
