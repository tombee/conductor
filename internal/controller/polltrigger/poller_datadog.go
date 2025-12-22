package polltrigger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DatadogPoller implements polling for Datadog monitor alerts API.
type DatadogPoller struct {
	apiKey string
	appKey string
	site   string
	client *http.Client
}

// NewDatadogPoller creates a new Datadog poller.
// site should be the Datadog site (e.g., "datadoghq.com", "datadoghq.eu")
func NewDatadogPoller(apiKey, appKey, site string) *DatadogPoller {
	if site == "" {
		site = "datadoghq.com"
	}

	return &DatadogPoller{
		apiKey: apiKey,
		appKey: appKey,
		site:   site,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the integration name.
func (p *DatadogPoller) Name() string {
	return "datadog"
}

// Poll queries the Datadog monitors API for alerts matching the query parameters.
// Supports query parameters: tags, monitor_ids, groups, statuses
func (p *DatadogPoller) Poll(ctx context.Context, state *PollState, query map[string]interface{}) ([]map[string]interface{}, string, error) {
	// Build query parameters
	params := url.Values{}

	// Add tags filter
	if tags, ok := query["tags"].([]interface{}); ok && len(tags) > 0 {
		var tagStrs []string
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				tagStrs = append(tagStrs, tagStr)
			}
		}
		if len(tagStrs) > 0 {
			params.Set("tags", strings.Join(tagStrs, ","))
		}
	}

	// Add monitor_ids filter
	if monitorIDs, ok := query["monitor_ids"].([]interface{}); ok && len(monitorIDs) > 0 {
		var idStrs []string
		for _, id := range monitorIDs {
			switch v := id.(type) {
			case int:
				idStrs = append(idStrs, strconv.Itoa(v))
			case float64:
				idStrs = append(idStrs, strconv.Itoa(int(v)))
			case string:
				idStrs = append(idStrs, v)
			}
		}
		if len(idStrs) > 0 {
			params.Set("monitor_ids", strings.Join(idStrs, ","))
		}
	}

	// Add group_states filter
	if groups, ok := query["groups"].([]interface{}); ok && len(groups) > 0 {
		var groupStrs []string
		for _, group := range groups {
			if groupStr, ok := group.(string); ok {
				groupStrs = append(groupStrs, groupStr)
			}
		}
		if len(groupStrs) > 0 {
			params.Set("group_states", strings.Join(groupStrs, ","))
		}
	}

	// Build API URL
	baseURL := fmt.Sprintf("https://api.%s/api/v1/monitor", p.site)
	apiURL := baseURL

	if len(params) > 0 {
		apiURL = fmt.Sprintf("%s?%s", baseURL, params.Encode())
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set auth headers
	req.Header.Set("DD-API-KEY", p.apiKey)
	req.Header.Set("DD-APPLICATION-KEY", p.appKey)
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", wrapAPIError(err, "datadog")
	}
	defer resp.Body.Close()

	// Check status code
	if err := p.checkStatusCode(resp); err != nil {
		return nil, "", err
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	var monitors []datadogMonitor
	if err := json.Unmarshal(body, &monitors); err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract statuses filter from query (default to triggered and warn)
	allowedStatuses := map[string]bool{
		"Alert": true,
		"Warn":  true,
	}
	if statuses, ok := query["statuses"].([]interface{}); ok && len(statuses) > 0 {
		allowedStatuses = make(map[string]bool)
		for _, s := range statuses {
			if statusStr, ok := s.(string); ok {
				// Normalize status names
				normalized := normalizeDatadogStatus(statusStr)
				allowedStatuses[normalized] = true
			}
		}
	}

	// Convert monitors to events, filtering by status and timestamp
	var events []map[string]interface{}
	for _, monitor := range monitors {
		// Filter by overall state
		if !allowedStatuses[monitor.OverallState] {
			continue
		}

		// Check if monitor state changed since last poll
		if !state.LastPollTime.IsZero() {
			// Parse modified timestamp
			modifiedTime, err := time.Parse(time.RFC3339, monitor.Modified)
			if err == nil && modifiedTime.Before(state.LastPollTime) {
				continue
			}
		}

		// Build event from monitor
		event := p.monitorToEvent(monitor)
		events = append(events, event)
	}

	// Datadog doesn't use cursor-based pagination for this endpoint
	return events, "", nil
}

// normalizeDatadogStatus normalizes status names to Datadog's canonical form.
func normalizeDatadogStatus(status string) string {
	lower := strings.ToLower(status)
	switch lower {
	case "triggered", "alert":
		return "Alert"
	case "warn", "warning":
		return "Warn"
	case "ok", "no data", "ignored", "skipped":
		return strings.Title(lower)
	default:
		return status
	}
}

// monitorToEvent converts a Datadog monitor to a generic event map.
func (p *DatadogPoller) monitorToEvent(monitor datadogMonitor) map[string]interface{} {
	event := map[string]interface{}{
		"id":       monitor.ID,
		"name":     monitor.Name,
		"status":   monitor.OverallState,
		"type":     monitor.Type,
		"message":  monitor.Message,
		"modified": monitor.Modified,
	}

	// Add query
	if monitor.Query != "" {
		event["query"] = monitor.Query
	}

	// Add tags
	if len(monitor.Tags) > 0 {
		event["tags"] = monitor.Tags
	}

	// Add options
	if monitor.Options != nil {
		options := make(map[string]interface{})

		if monitor.Options.Thresholds != nil {
			thresholds := make(map[string]interface{})
			if monitor.Options.Thresholds.Critical != nil {
				thresholds["critical"] = *monitor.Options.Thresholds.Critical
			}
			if monitor.Options.Thresholds.Warning != nil {
				thresholds["warning"] = *monitor.Options.Thresholds.Warning
			}
			if len(thresholds) > 0 {
				options["thresholds"] = thresholds
			}
		}

		if len(options) > 0 {
			event["options"] = options
		}
	}

	// Add creator info
	if monitor.Creator != nil {
		event["creator"] = map[string]interface{}{
			"email": monitor.Creator.Email,
			"name":  monitor.Creator.Name,
		}
	}

	return event
}

// checkStatusCode validates the HTTP response status code.
func (p *DatadogPoller) checkStatusCode(resp *http.Response) error {
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("Datadog auth failed (%d). API/App key may be invalid or expired", resp.StatusCode)
	}
	if resp.StatusCode == 429 {
		return fmt.Errorf("Datadog rate limit exceeded (429)")
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("Datadog API error (%d)", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Datadog API returned status %d", resp.StatusCode)
	}
	return nil
}

// Datadog API response types

type datadogMonitor struct {
	ID           int64                  `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Query        string                 `json:"query"`
	Message      string                 `json:"message"`
	Tags         []string               `json:"tags"`
	Options      *datadogMonitorOptions `json:"options"`
	OverallState string                 `json:"overall_state"`
	Creator      *datadogUser           `json:"creator"`
	Created      string                 `json:"created"`
	Modified     string                 `json:"modified"`
}

type datadogMonitorOptions struct {
	Thresholds *datadogThresholds `json:"thresholds"`
}

type datadogThresholds struct {
	Critical *float64 `json:"critical"`
	Warning  *float64 `json:"warning"`
}

type datadogUser struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}
