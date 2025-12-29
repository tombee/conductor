// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SlackHandler handles Slack webhooks.
type SlackHandler struct{}

// Verify verifies the Slack webhook signature.
func (h *SlackHandler) Verify(r *http.Request, body []byte, secret string) error {
	// Get timestamp and signature
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")

	if timestamp == "" || signature == "" {
		return fmt.Errorf("missing required headers")
	}

	// Check timestamp to prevent replay attacks (5 minute window)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}

	if abs(time.Now().Unix()-ts) > 300 {
		return fmt.Errorf("request too old")
	}

	// Compute expected signature
	// sig_basestring = "v0:" + timestamp + ":" + request_body
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(baseString))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	// Compare signatures
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// ParseEvent parses the Slack event type from the request.
func (h *SlackHandler) ParseEvent(r *http.Request) string {
	// For Slack Events API, the event type is in the body
	// For Slack slash commands and interactions, we use different events

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		// Slash command or interactive component
		return "slash_command"
	}

	// Events API - event type will be in the body
	return "event_callback"
}

// ExtractPayload extracts the payload from a Slack webhook.
func (h *SlackHandler) ExtractPayload(body []byte) (map[string]any, error) {
	var payload map[string]any

	// Try JSON first
	if err := json.Unmarshal(body, &payload); err == nil {
		// Check for URL verification challenge
		if payload["type"] == "url_verification" {
			// Return the challenge for verification
			return payload, nil
		}
		return payload, nil
	}

	// Try form-encoded (slash commands)
	// Parse as URL-encoded form data
	payload = make(map[string]any)
	parts := strings.Split(string(body), "&")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := kv[0]
			value := kv[1]
			// URL decode would go here in production
			payload[key] = value
		}
	}

	if len(payload) == 0 {
		return nil, fmt.Errorf("failed to parse payload")
	}

	return payload, nil
}

// SlackEvent represents a Slack Events API payload.
type SlackEvent struct {
	Token       string           `json:"token"`
	TeamID      string           `json:"team_id"`
	APIAppID    string           `json:"api_app_id"`
	Event       SlackEventDetail `json:"event"`
	Type        string           `json:"type"`
	EventID     string           `json:"event_id"`
	EventTime   int64            `json:"event_time"`
	AuthedUsers []string         `json:"authed_users,omitempty"`

	// URL verification fields
	Challenge string `json:"challenge,omitempty"`
}

// SlackEventDetail contains the actual event data.
type SlackEventDetail struct {
	Type    string `json:"type"`
	User    string `json:"user"`
	Text    string `json:"text,omitempty"`
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
}

// SlackSlashCommand represents a Slack slash command payload.
type SlackSlashCommand struct {
	Token       string `json:"token"`
	TeamID      string `json:"team_id"`
	TeamDomain  string `json:"team_domain"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	Command     string `json:"command"`
	Text        string `json:"text"`
	ResponseURL string `json:"response_url"`
	TriggerID   string `json:"trigger_id"`
}

// Common Slack event types
const (
	SlackEventMessage        = "message"
	SlackEventAppMention     = "app_mention"
	SlackEventReactionAdded  = "reaction_added"
	SlackEventURLVerification = "url_verification"
)

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
