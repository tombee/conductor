package jenkins

import (
	"context"

	"github.com/tombee/conductor/internal/connector"
)

// getJob gets job configuration and details.
func (c *JenkinsConnector) getJob(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"job_name"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/job/{job_name}/api/json", inputs)
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
	var job Job
	if err := c.ParseJSONResponse(resp, &job); err != nil {
		return nil, err
	}

	// Return simplified result
	result := map[string]interface{}{
		"name":        job.Name,
		"url":         job.URL,
		"description": job.Description,
		"buildable":   job.Buildable,
		"in_queue":    job.InQueue,
		"color":       job.Color,
	}

	// Add build references if they exist
	if job.LastBuild != nil {
		result["last_build_number"] = job.LastBuild.Number
		result["last_build_url"] = job.LastBuild.URL
	}

	if job.LastSuccessfulBuild != nil {
		result["last_successful_build_number"] = job.LastSuccessfulBuild.Number
	}

	if job.LastFailedBuild != nil {
		result["last_failed_build_number"] = job.LastFailedBuild.Number
	}

	return c.ToConnectorResult(resp, result), nil
}

// listJobs lists jobs in a folder (or root).
func (c *JenkinsConnector) listJobs(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// folder_path is optional, defaults to root
	folderPath := ""
	if fp, ok := inputs["folder_path"].(string); ok && fp != "" {
		folderPath = fp
	}

	// Build URL
	var url string
	var err error
	if folderPath == "" {
		// List jobs at root
		url = c.baseURL + "/api/json?tree=jobs[name,url,color,buildable]"
	} else {
		// List jobs in folder
		url, err = c.BuildURL("/job/{folder_path}/api/json?tree=jobs[name,url,color,buildable]", map[string]interface{}{
			"folder_path": folderPath,
		})
		if err != nil {
			return nil, err
		}
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
	var jobList JobListResponse
	if err := c.ParseJSONResponse(resp, &jobList); err != nil {
		return nil, err
	}

	// Transform to simplified format
	result := make([]map[string]interface{}, len(jobList.Jobs))
	for i, job := range jobList.Jobs {
		result[i] = map[string]interface{}{
			"name":      job.Name,
			"url":       job.URL,
			"color":     job.Color,
			"buildable": job.Buildable,
		}
	}

	return c.ToConnectorResult(resp, result), nil
}
