// Package custom provides workflow-defined custom tools (HTTP and script).
package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/tombee/conductor/pkg/httpclient"
	"github.com/tombee/conductor/pkg/tools"
	"github.com/tombee/conductor/pkg/workflow"
)

// HTTPCustomTool implements the tools.Tool interface for HTTP endpoint tools.
// It makes HTTP requests based on workflow-defined configuration.
type HTTPCustomTool struct {
	name            string
	description     string
	method          string
	urlTemplate     string
	headers         map[string]string
	inputSchema     *tools.Schema
	timeout         time.Duration
	maxResponseSize int64
}

// NewHTTPCustomTool creates an HTTP custom tool from a function definition.
func NewHTTPCustomTool(def workflow.FunctionDefinition) (*HTTPCustomTool, error) {
	// Validate this is an HTTP function
	if def.Type != workflow.ToolTypeHTTP {
		return nil, fmt.Errorf("function type must be http, got: %s", def.Type)
	}

	// Apply defaults
	timeout := time.Duration(def.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxResponseSize := def.MaxResponseSize
	if maxResponseSize == 0 {
		maxResponseSize = 1024 * 1024 // 1MB default
	}

	// Convert input schema to tools.Schema
	var inputSchema *tools.Schema
	if def.InputSchema != nil {
		paramSchema, err := convertToParameterSchema(def.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("invalid input schema: %w", err)
		}
		inputSchema = &tools.Schema{
			Inputs: paramSchema,
		}
	}

	return &HTTPCustomTool{
		name:            def.Name,
		description:     def.Description,
		method:          def.Method,
		urlTemplate:     def.URL,
		headers:         def.Headers,
		inputSchema:     inputSchema,
		timeout:         timeout,
		maxResponseSize: maxResponseSize,
	}, nil
}

// Name returns the tool name.
func (h *HTTPCustomTool) Name() string {
	return h.name
}

// Description returns the tool description.
func (h *HTTPCustomTool) Description() string {
	return h.description
}

// Schema returns the tool's input/output schema.
func (h *HTTPCustomTool) Schema() *tools.Schema {
	if h.inputSchema == nil {
		// Return empty schema if none defined
		return &tools.Schema{
			Inputs: &tools.ParameterSchema{
				Type:       "object",
				Properties: make(map[string]*tools.Property),
			},
		}
	}
	return h.inputSchema
}

// Execute makes an HTTP request with the provided inputs.
func (h *HTTPCustomTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	// Interpolate URL template
	url, err := h.interpolateTemplate(h.urlTemplate, inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to interpolate URL: %w", err)
	}

	// Prepare request body if inputs are provided and method supports body
	var bodyReader io.Reader
	if len(inputs) > 0 && (h.method == "POST" || h.method == "PUT" || h.method == "PATCH") {
		bodyBytes, err := json.Marshal(inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal inputs: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, h.method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default Content-Type for body requests
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Interpolate and set headers
	for key, valueTemplate := range h.headers {
		value, err := h.interpolateTemplate(valueTemplate, inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to interpolate header %s: %w", key, err)
		}
		req.Header.Set(key, value)
	}

	// Execute request using shared httpclient package
	cfg := httpclient.DefaultConfig()
	cfg.Timeout = h.timeout
	cfg.UserAgent = fmt.Sprintf("conductor-custom-tool/%s", h.name)

	client, err := httpclient.New(cfg)
	if err != nil {
		// Fallback to basic client if creation fails
		client = &http.Client{Timeout: h.timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		// Read error body for context
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, h.maxResponseSize))
		return nil, fmt.Errorf("http error: %d %s - %s", resp.StatusCode, resp.Status, string(bodyBytes))
	}

	// Read response body with size limit
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, h.maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response based on Content-Type
	contentType := resp.Header.Get("Content-Type")
	var result interface{}

	if strings.Contains(contentType, "application/json") {
		// Parse as JSON
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON response: %w", err)
		}
	} else {
		// Return as string
		result = string(bodyBytes)
	}

	return map[string]interface{}{
		"response":     result,
		"status_code":  resp.StatusCode,
		"content_type": contentType,
	}, nil
}

// interpolateTemplate interpolates a template string with inputs and environment variables.
func (h *HTTPCustomTool) interpolateTemplate(tmplStr string, inputs map[string]interface{}) (string, error) {
	// Create template
	tmpl, err := template.New("url").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("invalid template: %w", err)
	}

	// Prepare template context with inputs and env
	data := map[string]interface{}{
		"inputs": inputs,
		"env":    envToMap(),
	}

	// Support direct field access (e.g., {{.id}} instead of {{.inputs.id}})
	for k, v := range inputs {
		data[k] = v
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	return buf.String(), nil
}

// convertToParameterSchema converts a generic map to a ParameterSchema.
func convertToParameterSchema(m map[string]interface{}) (*tools.ParameterSchema, error) {
	// Marshal and unmarshal to convert to proper type
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	var schema tools.ParameterSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}

	return &schema, nil
}
