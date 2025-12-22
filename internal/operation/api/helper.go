package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/transport"
)

// BaseProvider provides common functionality for API integrations.
type BaseProvider struct {
	name      string
	transport transport.Transport
	baseURL   string
	token     string
}

// NewBaseProvider creates a new base provider.
func NewBaseProvider(name string, config *ProviderConfig) *BaseProvider {
	return &BaseProvider{
		name:      name,
		transport: config.Transport,
		baseURL:   config.BaseURL,
		token:     config.Token,
	}
}

// Name returns the integration identifier.
func (c *BaseProvider) Name() string {
	return c.name
}

// BuildURL constructs a full URL from a path template and inputs.
// Path templates use {param} syntax (e.g., "/repos/{owner}/{repo}/issues").
func (c *BaseProvider) BuildURL(pathTemplate string, inputs map[string]interface{}) (string, error) {
	path := pathTemplate

	// Replace path parameters
	for key, value := range inputs {
		placeholder := fmt.Sprintf("{%s}", key)
		if strings.Contains(path, placeholder) {
			path = strings.ReplaceAll(path, placeholder, fmt.Sprint(value))
		}
	}

	// Check for unreplaced parameters
	if strings.Contains(path, "{") && strings.Contains(path, "}") {
		start := strings.Index(path, "{")
		end := strings.Index(path, "}")
		missing := path[start+1 : end]
		return "", fmt.Errorf("missing required parameter: %s", missing)
	}

	// Combine with base URL
	fullURL := c.baseURL + path
	return fullURL, nil
}

// BuildQueryString constructs a query string from inputs.
// Parameters in pathParams are excluded from the query string.
func (c *BaseProvider) BuildQueryString(inputs map[string]interface{}, pathParams []string) string {
	values := url.Values{}

	pathParamSet := make(map[string]bool)
	for _, param := range pathParams {
		pathParamSet[param] = true
	}

	for key, value := range inputs {
		// Skip path parameters
		if pathParamSet[key] {
			continue
		}

		// Skip nil values
		if value == nil {
			continue
		}

		// Skip pagination control parameters
		if key == "paginate" || key == "max_results" {
			continue
		}

		// Add to query string
		values.Add(key, fmt.Sprint(value))
	}

	if len(values) == 0 {
		return ""
	}

	return "?" + values.Encode()
}

// BuildRequestBody constructs a JSON request body from inputs.
// Parameters in excludeParams are excluded from the body.
func (c *BaseProvider) BuildRequestBody(inputs map[string]interface{}, excludeParams []string) ([]byte, error) {
	excludeSet := make(map[string]bool)
	for _, param := range excludeParams {
		excludeSet[param] = true
	}

	body := make(map[string]interface{})
	for key, value := range inputs {
		if !excludeSet[key] && value != nil {
			body[key] = value
		}
	}

	if len(body) == 0 {
		return nil, nil
	}

	return json.Marshal(body)
}

// ExecuteRequest sends an HTTP request and returns the response.
func (c *BaseProvider) ExecuteRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (*transport.Response, error) {
	// Add authentication header
	if c.token != "" {
		if headers == nil {
			headers = make(map[string]string)
		}
		headers["Authorization"] = "Bearer " + c.token
	}

	req := &transport.Request{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}

	return c.transport.Execute(ctx, req)
}

// ParseJSONResponse parses a JSON response into a target struct.
func (c *BaseProvider) ParseJSONResponse(resp *transport.Response, target interface{}) error {
	if len(resp.Body) == 0 {
		return nil
	}

	return json.Unmarshal(resp.Body, target)
}

// ToResult converts a transport response to an operation result.
func (c *BaseProvider) ToResult(resp *transport.Response, response interface{}) *operation.Result {
	return &operation.Result{
		Response:    response,
		RawResponse: resp.Body,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata:    resp.Metadata,
	}
}

// ValidateRequired checks that all required parameters are present in inputs.
func (c *BaseProvider) ValidateRequired(inputs map[string]interface{}, required []string) error {
	for _, param := range required {
		if _, ok := inputs[param]; !ok {
			return fmt.Errorf("missing required parameter: %s", param)
		}
	}
	return nil
}
