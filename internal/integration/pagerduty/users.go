package pagerduty

import (
	"context"
	"fmt"
	"net/url"

	"github.com/tombee/conductor/internal/operation"
)

// getCurrentUser gets the currently authenticated user.
func (c *PagerDutyIntegration) getCurrentUser(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	urlStr, err := c.BuildURL("/users/me", nil)
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

	var userResp GetCurrentUserResponse
	if err := c.ParseJSONResponse(resp, &userResp); err != nil {
		return nil, err
	}

	return c.ToResult(resp, map[string]interface{}{
		"id":        userResp.User.ID,
		"name":      userResp.User.Name,
		"email":     userResp.User.Email,
		"time_zone": userResp.User.TimeZone,
		"role":      userResp.User.Role,
		"html_url":  userResp.User.HTMLURL,
	}), nil
}

// listServices lists PagerDuty services.
func (c *PagerDutyIntegration) listServices(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	query := url.Values{}

	if limit, ok := inputs["limit"]; ok {
		query.Set("limit", fmt.Sprintf("%v", limit))
	}
	if offset, ok := inputs["offset"]; ok {
		query.Set("offset", fmt.Sprintf("%v", offset))
	}
	if teamIDs, ok := inputs["team_ids"].([]interface{}); ok {
		for _, id := range teamIDs {
			query.Add("team_ids[]", fmt.Sprintf("%v", id))
		}
	}
	if queryStr, ok := inputs["query"]; ok {
		query.Set("query", fmt.Sprintf("%v", queryStr))
	}

	baseURL, err := c.BuildURL("/services", nil)
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

	var servicesResp ListServicesResponse
	if err := c.ParseJSONResponse(resp, &servicesResp); err != nil {
		return nil, err
	}

	services := make([]map[string]interface{}, len(servicesResp.Services))
	for i, svc := range servicesResp.Services {
		services[i] = map[string]interface{}{
			"id":          svc.ID,
			"name":        svc.Name,
			"description": svc.Description,
			"status":      svc.Status,
			"html_url":    svc.HTMLURL,
		}
		if svc.EscalationPolicy != nil {
			services[i]["escalation_policy_id"] = svc.EscalationPolicy.ID
			services[i]["escalation_policy_name"] = svc.EscalationPolicy.Summary
		}
	}

	return c.ToResult(resp, map[string]interface{}{
		"services": services,
		"total":    servicesResp.Total,
		"more":     servicesResp.More,
	}), nil
}

// listOnCalls lists on-call entries.
func (c *PagerDutyIntegration) listOnCalls(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	query := url.Values{}

	if limit, ok := inputs["limit"]; ok {
		query.Set("limit", fmt.Sprintf("%v", limit))
	}
	if offset, ok := inputs["offset"]; ok {
		query.Set("offset", fmt.Sprintf("%v", offset))
	}
	if userIDs, ok := inputs["user_ids"].([]interface{}); ok {
		for _, id := range userIDs {
			query.Add("user_ids[]", fmt.Sprintf("%v", id))
		}
	}
	if scheduleIDs, ok := inputs["schedule_ids"].([]interface{}); ok {
		for _, id := range scheduleIDs {
			query.Add("schedule_ids[]", fmt.Sprintf("%v", id))
		}
	}
	if escalationPolicyIDs, ok := inputs["escalation_policy_ids"].([]interface{}); ok {
		for _, id := range escalationPolicyIDs {
			query.Add("escalation_policy_ids[]", fmt.Sprintf("%v", id))
		}
	}
	if since, ok := inputs["since"]; ok {
		query.Set("since", fmt.Sprintf("%v", since))
	}
	if until, ok := inputs["until"]; ok {
		query.Set("until", fmt.Sprintf("%v", until))
	}

	baseURL, err := c.BuildURL("/oncalls", nil)
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

	var onCallsResp ListOnCallsResponse
	if err := c.ParseJSONResponse(resp, &onCallsResp); err != nil {
		return nil, err
	}

	oncalls := make([]map[string]interface{}, len(onCallsResp.OnCalls))
	for i, oc := range onCallsResp.OnCalls {
		oncalls[i] = map[string]interface{}{
			"user_id":            oc.User.ID,
			"user_name":          oc.User.Summary,
			"escalation_level":   oc.EscalationLevel,
			"escalation_policy":  oc.EscalationPolicy.Summary,
			"start":              oc.Start,
			"end":                oc.End,
		}
		if oc.Schedule != nil {
			oncalls[i]["schedule_id"] = oc.Schedule.ID
			oncalls[i]["schedule_name"] = oc.Schedule.Summary
		}
	}

	return c.ToResult(resp, map[string]interface{}{
		"oncalls": oncalls,
		"more":    onCallsResp.More,
	}), nil
}
