package jenkins

import (
	"context"
	"fmt"
	"strconv"

	"github.com/tombee/conductor/internal/connector"
)

// triggerBuild triggers a build without parameters.
func (c *JenkinsConnector) triggerBuild(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"job_name"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/job/{job_name}/build", inputs)
	if err != nil {
		return nil, err
	}

	// Get CRUMB token if needed
	headers := c.defaultHeaders()
	if err := c.addCrumb(ctx, headers); err != nil {
		return nil, fmt.Errorf("failed to get CRUMB token: %w", err)
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", url, headers, nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Jenkins returns 201 Created with Location header pointing to queue item
	queueItemURL := ""
	if location, ok := resp.Headers["Location"]; ok && len(location) > 0 {
		queueItemURL = location[0]
	}

	return c.ToConnectorResult(resp, map[string]interface{}{
		"queue_item_url": queueItemURL,
		"triggered":      true,
	}), nil
}

// triggerBuildWithParameters triggers a parameterized build.
func (c *JenkinsConnector) triggerBuildWithParameters(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"job_name", "parameters"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/job/{job_name}/buildWithParameters", inputs)
	if err != nil {
		return nil, err
	}

	// Add parameters as query string
	params, ok := inputs["parameters"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("parameters must be a map")
	}

	queryParams := make(map[string]interface{})
	queryParams["job_name"] = inputs["job_name"]
	for k, v := range params {
		queryParams[k] = v
	}

	queryString := c.BuildQueryString(queryParams, []string{"job_name"})
	url += queryString

	// Get CRUMB token if needed
	headers := c.defaultHeaders()
	if err := c.addCrumb(ctx, headers); err != nil {
		return nil, fmt.Errorf("failed to get CRUMB token: %w", err)
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", url, headers, nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Jenkins returns 201 Created with Location header pointing to queue item
	queueItemURL := ""
	if location, ok := resp.Headers["Location"]; ok && len(location) > 0 {
		queueItemURL = location[0]
	}

	return c.ToConnectorResult(resp, map[string]interface{}{
		"queue_item_url": queueItemURL,
		"triggered":      true,
	}), nil
}

// getBuild gets details about a specific build.
func (c *JenkinsConnector) getBuild(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"job_name", "build_number"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/job/{job_name}/{build_number}/api/json", inputs)
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
	var build Build
	if err := c.ParseJSONResponse(resp, &build); err != nil {
		return nil, err
	}

	// Return simplified result
	result := map[string]interface{}{
		"number":   build.Number,
		"url":      build.URL,
		"result":   build.Result,
		"building": build.Building,
		"duration": build.Duration,
	}

	return c.ToConnectorResult(resp, result), nil
}

// getBuildLog gets the console output for a build.
func (c *JenkinsConnector) getBuildLog(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"job_name", "build_number"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/job/{job_name}/{build_number}/consoleText", inputs)
	if err != nil {
		return nil, err
	}

	// Execute request (console output is plain text)
	headers := map[string]string{
		"Accept": "text/plain",
	}
	resp, err := c.ExecuteRequest(ctx, "GET", url, headers, nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Return log as string
	return c.ToConnectorResult(resp, map[string]interface{}{
		"log": string(resp.Body),
	}), nil
}

// cancelBuild stops a running build.
func (c *JenkinsConnector) cancelBuild(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"job_name", "build_number"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/job/{job_name}/{build_number}/stop", inputs)
	if err != nil {
		return nil, err
	}

	// Get CRUMB token if needed
	headers := c.defaultHeaders()
	if err := c.addCrumb(ctx, headers); err != nil {
		return nil, fmt.Errorf("failed to get CRUMB token: %w", err)
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", url, headers, nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	return c.ToConnectorResult(resp, map[string]interface{}{
		"cancelled": true,
	}), nil
}

// getLastBuild gets the last build info.
func (c *JenkinsConnector) getLastBuild(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	return c.getBuildInfo(ctx, inputs, "lastBuild")
}

// getLastSuccessfulBuild gets the last successful build info.
func (c *JenkinsConnector) getLastSuccessfulBuild(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	return c.getBuildInfo(ctx, inputs, "lastSuccessfulBuild")
}

// getLastFailedBuild gets the last failed build info.
func (c *JenkinsConnector) getLastFailedBuild(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	return c.getBuildInfo(ctx, inputs, "lastFailedBuild")
}

// getBuildInfo is a helper to get specific build info types.
func (c *JenkinsConnector) getBuildInfo(ctx context.Context, inputs map[string]interface{}, buildType string) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"job_name"}); err != nil {
		return nil, err
	}

	// Build URL to get job info with specific build
	url, err := c.BuildURL("/job/{job_name}/api/json", inputs)
	if err != nil {
		return nil, err
	}

	// Add tree parameter to limit response
	url += fmt.Sprintf("?tree=%s[number,url,result,building,duration,timestamp]", buildType)

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response - we expect a job with the specific build field
	var jobResp struct {
		LastBuild           *Build `json:"lastBuild,omitempty"`
		LastSuccessfulBuild *Build `json:"lastSuccessfulBuild,omitempty"`
		LastFailedBuild     *Build `json:"lastFailedBuild,omitempty"`
	}

	if err := c.ParseJSONResponse(resp, &jobResp); err != nil {
		return nil, err
	}

	var build *Build
	switch buildType {
	case "lastBuild":
		build = jobResp.LastBuild
	case "lastSuccessfulBuild":
		build = jobResp.LastSuccessfulBuild
	case "lastFailedBuild":
		build = jobResp.LastFailedBuild
	}

	if build == nil {
		return c.ToConnectorResult(resp, map[string]interface{}{
			"exists": false,
		}), nil
	}

	// Return simplified result
	result := map[string]interface{}{
		"exists":   true,
		"number":   build.Number,
		"url":      build.URL,
		"result":   build.Result,
		"building": build.Building,
		"duration": build.Duration,
	}

	return c.ToConnectorResult(resp, result), nil
}

// getTestReport gets test results for a build.
func (c *JenkinsConnector) getTestReport(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"job_name", "build_number"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/job/{job_name}/{build_number}/testReport/api/json", inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any - 404 means no test report exists
	if resp.StatusCode == 404 {
		return c.ToConnectorResult(resp, map[string]interface{}{
			"exists": false,
		}), nil
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var testReport TestReport
	if err := c.ParseJSONResponse(resp, &testReport); err != nil {
		return nil, err
	}

	// Return simplified result
	result := map[string]interface{}{
		"exists":     true,
		"duration":   testReport.Duration,
		"fail_count": testReport.FailCount,
		"pass_count": testReport.PassCount,
		"skip_count": testReport.SkipCount,
		"empty":      testReport.Empty,
	}

	return c.ToConnectorResult(resp, result), nil
}

// addCrumb fetches and adds a CRUMB token to headers if CSRF protection is enabled.
func (c *JenkinsConnector) addCrumb(ctx context.Context, headers map[string]string) error {
	// Try to get crumb
	crumbURL := c.baseURL + "/crumbIssuer/api/json"
	resp, err := c.ExecuteRequest(ctx, "GET", crumbURL, c.defaultHeaders(), nil)
	if err != nil {
		// If crumb endpoint doesn't exist, CSRF protection might be disabled
		// This is not an error, we can proceed without crumb
		return nil
	}

	// If we get 404, CSRF protection is disabled
	if resp.StatusCode == 404 {
		return nil
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		// If we can't get crumb, try to proceed anyway
		// Some Jenkins instances don't require it
		return nil
	}

	// Parse crumb
	var crumb Crumb
	if err := c.ParseJSONResponse(resp, &crumb); err != nil {
		// If we can't parse crumb, try to proceed anyway
		return nil
	}

	// Add crumb to headers
	if crumb.CrumbRequestField != "" && crumb.Crumb != "" {
		headers[crumb.CrumbRequestField] = crumb.Crumb
	}

	return nil
}

// defaultHeaders returns default headers for Jenkins API requests.
func (c *JenkinsConnector) defaultHeaders() map[string]string {
	return map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}
}

// buildNumberToString converts a build_number input to a string for URL building.
// Jenkins accepts both numbers and special values like "lastBuild", "lastSuccessfulBuild", etc.
func buildNumberToString(buildNumber interface{}) string {
	switch v := buildNumber.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return fmt.Sprint(v)
	}
}
