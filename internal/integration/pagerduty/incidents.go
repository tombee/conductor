package pagerduty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tombee/conductor/internal/operation"
)

// listIncidents lists incidents with optional filters.
func (c *PagerDutyIntegration) listIncidents(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Build query parameters
	query := url.Values{}

	// Optional filters
	if statuses, ok := inputs["statuses"].([]interface{}); ok {
		for _, s := range statuses {
			query.Add("statuses[]", fmt.Sprintf("%v", s))
		}
	}
	if status, ok := inputs["status"].(string); ok {
		query.Add("statuses[]", status)
	}
	if userIDs, ok := inputs["user_ids"].([]interface{}); ok {
		for _, id := range userIDs {
			query.Add("user_ids[]", fmt.Sprintf("%v", id))
		}
	}
	if userID, ok := inputs["user_id"].(string); ok {
		query.Add("user_ids[]", userID)
	}
	if serviceIDs, ok := inputs["service_ids"].([]interface{}); ok {
		for _, id := range serviceIDs {
			query.Add("service_ids[]", fmt.Sprintf("%v", id))
		}
	}
	if since, ok := inputs["since"]; ok {
		query.Set("since", fmt.Sprintf("%v", since))
	}
	if until, ok := inputs["until"]; ok {
		query.Set("until", fmt.Sprintf("%v", until))
	}
	if limit, ok := inputs["limit"]; ok {
		query.Set("limit", fmt.Sprintf("%v", limit))
	}
	if offset, ok := inputs["offset"]; ok {
		query.Set("offset", fmt.Sprintf("%v", offset))
	}
	if sortBy, ok := inputs["sort_by"]; ok {
		query.Set("sort_by", fmt.Sprintf("%v", sortBy))
	}

	// Build URL
	baseURL, err := c.BuildURL("/incidents", nil)
	if err != nil {
		return nil, err
	}
	fullURL := baseURL
	if len(query) > 0 {
		fullURL = baseURL + "?" + query.Encode()
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "GET", fullURL, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var listResp ListIncidentsResponse
	if err := c.ParseJSONResponse(resp, &listResp); err != nil {
		return nil, err
	}

	// Convert incidents to a more usable format
	incidents := make([]map[string]interface{}, len(listResp.Incidents))
	for i, inc := range listResp.Incidents {
		incidents[i] = incidentToMap(inc)
	}

	return c.ToResult(resp, map[string]interface{}{
		"incidents": incidents,
		"total":     listResp.Total,
		"more":      listResp.More,
		"limit":     listResp.Limit,
		"offset":    listResp.Offset,
	}), nil
}

// getIncident gets a single incident by ID.
func (c *PagerDutyIntegration) getIncident(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	if err := c.ValidateRequired(inputs, []string{"id"}); err != nil {
		return nil, err
	}

	incidentID := fmt.Sprintf("%v", inputs["id"])
	urlStr, err := c.BuildURL("/incidents/"+incidentID, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "GET", urlStr, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var getResp GetIncidentResponse
	if err := c.ParseJSONResponse(resp, &getResp); err != nil {
		return nil, err
	}

	return c.ToResult(resp, incidentToMap(getResp.Incident)), nil
}

// updateIncident updates an incident.
func (c *PagerDutyIntegration) updateIncident(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	if err := c.ValidateRequired(inputs, []string{"id"}); err != nil {
		return nil, err
	}

	incidentID := fmt.Sprintf("%v", inputs["id"])
	urlStr, err := c.BuildURL("/incidents/"+incidentID, nil)
	if err != nil {
		return nil, err
	}

	// Build the update body
	incidentUpdate := make(map[string]interface{})
	incidentUpdate["id"] = incidentID
	incidentUpdate["type"] = "incident"

	if status, ok := inputs["status"]; ok {
		incidentUpdate["status"] = status
	}
	if title, ok := inputs["title"]; ok {
		incidentUpdate["title"] = title
	}
	if urgency, ok := inputs["urgency"]; ok {
		incidentUpdate["urgency"] = urgency
	}
	if escalationLevel, ok := inputs["escalation_level"]; ok {
		incidentUpdate["escalation_level"] = escalationLevel
	}
	if resolution, ok := inputs["resolution"]; ok {
		incidentUpdate["resolution"] = resolution
	}

	bodyMap := map[string]interface{}{
		"incident": incidentUpdate,
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.ExecuteRequest(ctx, "PUT", urlStr, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var updateResp GetIncidentResponse
	if err := c.ParseJSONResponse(resp, &updateResp); err != nil {
		return nil, err
	}

	return c.ToResult(resp, incidentToMap(updateResp.Incident)), nil
}

// acknowledgeIncident acknowledges an incident.
func (c *PagerDutyIntegration) acknowledgeIncident(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	inputs["status"] = "acknowledged"
	return c.updateIncident(ctx, inputs)
}

// resolveIncident resolves an incident.
func (c *PagerDutyIntegration) resolveIncident(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	inputs["status"] = "resolved"
	return c.updateIncident(ctx, inputs)
}

// listIncidentNotes lists notes for an incident.
func (c *PagerDutyIntegration) listIncidentNotes(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	if err := c.ValidateRequired(inputs, []string{"incident_id"}); err != nil {
		return nil, err
	}

	incidentID := fmt.Sprintf("%v", inputs["incident_id"])
	urlStr, err := c.BuildURL("/incidents/"+incidentID+"/notes", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "GET", urlStr, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var notesResp ListIncidentNotesResponse
	if err := c.ParseJSONResponse(resp, &notesResp); err != nil {
		return nil, err
	}

	notes := make([]map[string]interface{}, len(notesResp.Notes))
	for i, note := range notesResp.Notes {
		notes[i] = map[string]interface{}{
			"id":         note.ID,
			"content":    note.Content,
			"created_at": note.CreatedAt,
			"user_id":    note.User.ID,
			"user_name":  note.User.Summary,
		}
	}

	return c.ToResult(resp, map[string]interface{}{
		"notes": notes,
	}), nil
}

// createIncidentNote creates a note on an incident.
func (c *PagerDutyIntegration) createIncidentNote(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	if err := c.ValidateRequired(inputs, []string{"incident_id", "content"}); err != nil {
		return nil, err
	}

	incidentID := fmt.Sprintf("%v", inputs["incident_id"])
	urlStr, err := c.BuildURL("/incidents/"+incidentID+"/notes", nil)
	if err != nil {
		return nil, err
	}

	bodyMap := map[string]interface{}{
		"note": map[string]interface{}{
			"content": inputs["content"],
		},
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.ExecuteRequest(ctx, "POST", urlStr, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var noteResp CreateIncidentNoteResponse
	if err := c.ParseJSONResponse(resp, &noteResp); err != nil {
		return nil, err
	}

	return c.ToResult(resp, map[string]interface{}{
		"id":         noteResp.Note.ID,
		"content":    noteResp.Note.Content,
		"created_at": noteResp.Note.CreatedAt,
	}), nil
}

// listIncidentLogEntries lists log entries for an incident.
func (c *PagerDutyIntegration) listIncidentLogEntries(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	if err := c.ValidateRequired(inputs, []string{"incident_id"}); err != nil {
		return nil, err
	}

	incidentID := fmt.Sprintf("%v", inputs["incident_id"])

	query := url.Values{}
	if limit, ok := inputs["limit"]; ok {
		query.Set("limit", fmt.Sprintf("%v", limit))
	}
	if offset, ok := inputs["offset"]; ok {
		query.Set("offset", fmt.Sprintf("%v", offset))
	}

	baseURL, err := c.BuildURL("/incidents/"+incidentID+"/log_entries", nil)
	if err != nil {
		return nil, err
	}
	fullURL := baseURL
	if len(query) > 0 {
		fullURL = baseURL + "?" + query.Encode()
	}

	resp, err := c.ExecuteRequest(ctx, "GET", fullURL, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var logResp ListLogEntriesResponse
	if err := c.ParseJSONResponse(resp, &logResp); err != nil {
		return nil, err
	}

	entries := make([]map[string]interface{}, len(logResp.LogEntries))
	for i, entry := range logResp.LogEntries {
		entries[i] = map[string]interface{}{
			"id":         entry.ID,
			"type":       entry.Type,
			"summary":    entry.Summary,
			"created_at": entry.CreatedAt,
		}
		if entry.Agent != nil {
			entries[i]["agent_id"] = entry.Agent.ID
			entries[i]["agent_name"] = entry.Agent.Summary
		}
	}

	return c.ToResult(resp, map[string]interface{}{
		"log_entries": entries,
		"more":        logResp.More,
	}), nil
}

// incidentToMap converts an Incident to a map for result output.
func incidentToMap(inc Incident) map[string]interface{} {
	result := map[string]interface{}{
		"id":              inc.ID,
		"incident_number": inc.IncidentNumber,
		"title":           inc.Title,
		"summary":         inc.Summary,
		"status":          inc.Status,
		"urgency":         inc.Urgency,
		"created_at":      inc.CreatedAt,
		"updated_at":      inc.UpdatedAt,
		"html_url":        inc.HTMLURL,
	}

	if inc.Service != nil {
		result["service_id"] = inc.Service.ID
		result["service_name"] = inc.Service.Summary
	}

	if inc.Priority != nil {
		result["priority_id"] = inc.Priority.ID
		result["priority_name"] = inc.Priority.Name
	}

	if inc.EscalationPolicy != nil {
		result["escalation_policy_id"] = inc.EscalationPolicy.ID
		result["escalation_policy_name"] = inc.EscalationPolicy.Summary
	}

	if len(inc.Assignments) > 0 {
		assignees := make([]map[string]interface{}, len(inc.Assignments))
		for i, a := range inc.Assignments {
			assignees[i] = map[string]interface{}{
				"at":      a.At,
				"user_id": a.Assignee.ID,
				"name":    a.Assignee.Summary,
			}
		}
		result["assignments"] = assignees
	}

	if inc.AlertCounts != nil {
		result["alert_counts"] = map[string]interface{}{
			"all":       inc.AlertCounts.All,
			"triggered": inc.AlertCounts.Triggered,
			"resolved":  inc.AlertCounts.Resolved,
		}
	}

	if inc.LastStatusChangeAt != "" {
		result["last_status_change_at"] = inc.LastStatusChangeAt
	}

	return result
}
