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

// PagerDutyPoller implements polling for PagerDuty incidents API.
type PagerDutyPoller struct {
	apiToken string
	baseURL  string
	client   *http.Client
}

// NewPagerDutyPoller creates a new PagerDuty poller.
func NewPagerDutyPoller(apiToken string) *PagerDutyPoller {
	return &PagerDutyPoller{
		apiToken: apiToken,
		baseURL:  "https://api.pagerduty.com",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the integration name.
func (p *PagerDutyPoller) Name() string {
	return "pagerduty"
}

// Poll queries the PagerDuty incidents API for new events since the last poll time.
// Supports query parameters: user_id, services, teams, statuses, urgencies
func (p *PagerDutyPoller) Poll(ctx context.Context, state *PollState, query map[string]interface{}) ([]map[string]interface{}, string, error) {
	// Build query parameters
	params := url.Values{}

	// Add timestamp filter (primary deduplication)
	if !state.LastPollTime.IsZero() {
		params.Set("since", state.LastPollTime.Format(time.RFC3339))
	}

	// Add user_id filter
	if userID, ok := query["user_id"].(string); ok && userID != "" {
		params.Add("user_ids[]", userID)
	}

	// Add services filter
	if services, ok := query["services"].([]interface{}); ok {
		for _, svc := range services {
			if svcID, ok := svc.(string); ok {
				params.Add("service_ids[]", svcID)
			}
		}
	}

	// Add teams filter
	if teams, ok := query["teams"].([]interface{}); ok {
		for _, team := range teams {
			if teamID, ok := team.(string); ok {
				params.Add("team_ids[]", teamID)
			}
		}
	}

	// Add statuses filter (default to triggered and acknowledged)
	statuses, ok := query["statuses"].([]interface{})
	if !ok || len(statuses) == 0 {
		statuses = []interface{}{"triggered", "acknowledged"}
	}
	for _, status := range statuses {
		if s, ok := status.(string); ok {
			params.Add("statuses[]", s)
		}
	}

	// Add urgencies filter
	if urgencies, ok := query["urgencies"].([]interface{}); ok {
		for _, urgency := range urgencies {
			if u, ok := urgency.(string); ok {
				params.Add("urgencies[]", u)
			}
		}
	}

	// Sort by created_at to get newest incidents first
	params.Set("sort_by", "created_at:desc")
	params.Set("limit", "100")

	// Build request
	apiURL := fmt.Sprintf("%s/incidents?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token token="+p.apiToken)

	// Execute request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", wrapAPIError(err, "pagerduty")
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, "", fmt.Errorf("PagerDuty auth failed (%d). Token may be expired or revoked", resp.StatusCode)
	}
	if resp.StatusCode == 429 {
		return nil, "", fmt.Errorf("PagerDuty rate limit exceeded (429)")
	}
	if resp.StatusCode >= 500 {
		return nil, "", fmt.Errorf("PagerDuty API error (%d)", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("PagerDuty API returned status %d", resp.StatusCode)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp pagerDutyIncidentsResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert incidents to events
	events := make([]map[string]interface{}, 0, len(apiResp.Incidents))
	for _, incident := range apiResp.Incidents {
		event := p.incidentToEvent(incident)
		events = append(events, event)
	}

	// Return events and cursor (PagerDuty uses offset pagination, not cursor)
	return events, "", nil
}

// incidentToEvent converts a PagerDuty incident to a generic event map.
func (p *PagerDutyPoller) incidentToEvent(incident pagerDutyIncident) map[string]interface{} {
	event := map[string]interface{}{
		"id":         incident.ID,
		"title":      incident.Title,
		"status":     incident.Status,
		"urgency":    incident.Urgency,
		"created_at": incident.CreatedAt,
		"html_url":   incident.HTMLURL,
	}

	// Add service info if available
	if incident.Service.ID != "" {
		event["service"] = map[string]interface{}{
			"id":   incident.Service.ID,
			"name": incident.Service.Summary,
		}
	}

	// Add assignments if available
	if len(incident.Assignments) > 0 {
		assignments := make([]map[string]interface{}, 0, len(incident.Assignments))
		for _, assignment := range incident.Assignments {
			assignments = append(assignments, map[string]interface{}{
				"assignee": map[string]interface{}{
					"id":   assignment.Assignee.ID,
					"name": assignment.Assignee.Summary,
				},
			})
		}
		event["assignments"] = assignments
	}

	return event
}

// wrapAPIError sanitizes error messages to prevent credential leakage.
func wrapAPIError(err error, integration string) error {
	// Remove any potential tokens or credentials from error message
	msg := err.Error()
	msg = strings.ReplaceAll(msg, "Token token=", "[REDACTED]")
	return fmt.Errorf("%s API error: %s", integration, msg)
}

// PagerDuty API response types

type pagerDutyIncidentsResponse struct {
	Incidents []pagerDutyIncident `json:"incidents"`
	Limit     int                 `json:"limit"`
	Offset    int                 `json:"offset"`
	More      bool                `json:"more"`
}

type pagerDutyIncident struct {
	ID          string                     `json:"id"`
	Type        string                     `json:"type"`
	Summary     string                     `json:"summary"`
	Title       string                     `json:"title"`
	Status      string                     `json:"status"`
	Urgency     string                     `json:"urgency"`
	CreatedAt   string                     `json:"created_at"`
	HTMLURL     string                     `json:"html_url"`
	Service     pagerDutyReference         `json:"service"`
	Assignments []pagerDutyAssignment      `json:"assignments"`
	Description string                     `json:"description,omitempty"`
}

type pagerDutyReference struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Summary string `json:"summary"`
	Self    string `json:"self"`
	HTMLURL string `json:"html_url"`
}

type pagerDutyAssignment struct {
	At       string             `json:"at"`
	Assignee pagerDutyReference `json:"assignee"`
}
