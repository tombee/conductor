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

package mock

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/tombee/conductor/internal/testing/fixture"
	"github.com/tombee/conductor/pkg/workflow"
)

// OperationRegistry is a mock operation registry that returns fixture-based responses.
type OperationRegistry struct {
	fixtureLoader *fixture.Loader
	realRegistry  workflow.OperationRegistry
	logger        *slog.Logger
}

// NewOperationRegistry creates a new mock operation registry.
func NewOperationRegistry(loader *fixture.Loader, realRegistry workflow.OperationRegistry, logger *slog.Logger) *OperationRegistry {
	return &OperationRegistry{
		fixtureLoader: loader,
		realRegistry:  realRegistry,
		logger:        logger,
	}
}

// Execute executes a mocked operation.
func (m *OperationRegistry) Execute(ctx context.Context, reference string, inputs map[string]interface{}) (workflow.OperationResult, error) {
	m.logger.Info("[MOCK] Operation execution", "reference", reference)

	// Extract step ID from context if available
	stepID := m.extractStepID(ctx, inputs)

	// Determine operation type from reference
	parts := strings.SplitN(reference, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid operation reference format: %q", reference)
	}

	operationType := parts[0]
	operationName := parts[1]

	// Route to appropriate mock handler based on type
	switch {
	case operationType == "http":
		return m.executeHTTPMock(stepID, operationName, inputs)
	case isIntegrationType(operationType):
		return m.executeIntegrationMock(stepID, operationType, operationName, inputs)
	default:
		// Unknown operation type - try real registry if available
		if m.realRegistry != nil {
			m.logger.Debug("[MOCK] Unknown operation type, using real registry", "type", operationType, "reference", reference)
			return m.realRegistry.Execute(ctx, reference, inputs)
		}
		return nil, fmt.Errorf("mock mode: unsupported operation type %q", operationType)
	}
}

// executeHTTPMock executes a mock HTTP action.
func (m *OperationRegistry) executeHTTPMock(stepID, operation string, inputs map[string]interface{}) (workflow.OperationResult, error) {
	// Load HTTP fixture
	fixtureData, err := m.fixtureLoader.LoadHTTPFixture(stepID)
	if err != nil {
		// If no fixture found and we have a real registry, fall back to it
		if m.realRegistry != nil {
			m.logger.Debug("[MOCK] No HTTP fixture found, using real registry", "step_id", stepID, "error", err)
			return m.realRegistry.Execute(context.Background(), "http."+operation, inputs)
		}
		return nil, fmt.Errorf("mock mode: %w", err)
	}

	// Find matching response
	responseData, err := m.findMatchingHTTPResponse(fixtureData, operation, inputs)
	if err != nil {
		return nil, err
	}

	// Build operation result
	result := &mockOperationResult{
		data: map[string]interface{}{
			"status_code": responseData.Status,
			"body":        responseData.Body,
			"headers":     responseData.Headers,
		},
		statusCode: responseData.Status,
	}

	return result, nil
}

// executeIntegrationMock executes a mock integration.
func (m *OperationRegistry) executeIntegrationMock(stepID, integrationType, operation string, inputs map[string]interface{}) (workflow.OperationResult, error) {
	// Load integration fixture
	fixtureData, err := m.fixtureLoader.LoadIntegrationFixture(stepID, integrationType)
	if err != nil {
		// If no fixture found and we have a real registry, fall back to it
		if m.realRegistry != nil {
			m.logger.Debug("[MOCK] No integration fixture found, using real registry", "step_id", stepID, "type", integrationType, "error", err)
			return m.realRegistry.Execute(context.Background(), integrationType+"."+operation, inputs)
		}
		return nil, fmt.Errorf("mock mode: %w", err)
	}

	// Find matching response
	responseData, err := m.findMatchingIntegrationResponse(fixtureData, operation, inputs)
	if err != nil {
		return nil, err
	}

	// Build operation result
	result := &mockOperationResult{
		data:       responseData,
		statusCode: 200, // Default success status for integrations
	}

	return result, nil
}

// findMatchingHTTPResponse finds the appropriate HTTP response from fixture data.
func (m *OperationRegistry) findMatchingHTTPResponse(fixtureData *fixture.HTTPFixture, operation string, inputs map[string]interface{}) (*fixture.HTTPResponseData, error) {
	// If simple response is set, use it
	if fixtureData.Response != nil {
		return fixtureData.Response, nil
	}

	// Try conditional responses
	var defaultResponse *fixture.HTTPResponse
	for i := range fixtureData.Responses {
		resp := &fixtureData.Responses[i]

		// Check if this is the default response
		if resp.Default {
			defaultResponse = resp
			continue
		}

		// Check conditions
		if resp.When != nil {
			// Check URL match
			if resp.When.URL != "" {
				url, _ := inputs["url"].(string)
				if !matchPattern(resp.When.URL, url) {
					continue
				}
			}

			// Check method match
			if resp.When.Method != "" {
				method := strings.ToUpper(operation)
				if !strings.EqualFold(resp.When.Method, method) {
					continue
				}
			}
		}

		// All conditions matched, use this response
		m.logger.Debug("[MOCK] Matched conditional HTTP response")
		return resp.Return, nil
	}

	// Use default response if available
	if defaultResponse != nil {
		m.logger.Debug("[MOCK] Using default HTTP response")
		return defaultResponse.Return, nil
	}

	// No matching response found
	return nil, fmt.Errorf("no matching HTTP response in fixture for operation %q", operation)
}

// findMatchingIntegrationResponse finds the appropriate integration response from fixture data.
func (m *OperationRegistry) findMatchingIntegrationResponse(fixtureData *fixture.IntegrationFixture, operation string, inputs map[string]interface{}) (map[string]interface{}, error) {
	// If simple response is set, use it
	if fixtureData.Response != nil {
		if result, ok := fixtureData.Response.(map[string]interface{}); ok {
			return result, nil
		}
		// Wrap non-map response
		return map[string]interface{}{"result": fixtureData.Response}, nil
	}

	// Try conditional responses
	var defaultResponse *fixture.IntegrationResponse
	for i := range fixtureData.Responses {
		resp := &fixtureData.Responses[i]

		// Check if this is the default response
		if resp.Default {
			defaultResponse = resp
			continue
		}

		// Check conditions
		if resp.When != nil {
			// Check operation match
			if resp.When.Operation != "" && !strings.EqualFold(resp.When.Operation, operation) {
				continue
			}

			// Check repo match (GitHub-specific)
			if resp.When.Repo != "" {
				repo, _ := inputs["repo"].(string)
				if !matchPattern(resp.When.Repo, repo) {
					continue
				}
			}
		}

		// All conditions matched, use this response
		m.logger.Debug("[MOCK] Matched conditional integration response", "operation", operation)
		if result, ok := resp.Return.(map[string]interface{}); ok {
			return result, nil
		}
		// Wrap non-map response
		return map[string]interface{}{"result": resp.Return}, nil
	}

	// Use default response if available
	if defaultResponse != nil {
		m.logger.Debug("[MOCK] Using default integration response", "operation", operation)
		if result, ok := defaultResponse.Return.(map[string]interface{}); ok {
			return result, nil
		}
		// Wrap non-map response
		return map[string]interface{}{"result": defaultResponse.Return}, nil
	}

	// No matching response found
	return nil, fmt.Errorf("no matching integration response in fixture for operation %q", operation)
}

// extractStepID extracts the step ID from context or inputs.
func (m *OperationRegistry) extractStepID(ctx context.Context, inputs map[string]interface{}) string {
	// Try to get from inputs
	if stepID, ok := inputs["_step_id"].(string); ok {
		return stepID
	}
	return "unknown"
}

// matchPattern matches a string against a pattern with wildcard support.
// Supports "*" wildcards like "**users/*" or "*".
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}

	// Simple wildcard matching (not full glob)
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}

	return pattern == value
}

// isIntegrationType checks if the operation type is an integration.
func isIntegrationType(opType string) bool {
	integrations := []string{"github", "slack", "jira", "linear"}
	for _, integration := range integrations {
		if opType == integration {
			return true
		}
	}
	return false
}

// mockOperationResult implements workflow.OperationResult.
type mockOperationResult struct {
	data       map[string]interface{}
	statusCode int
}

// GetResponse returns the transformed response data.
func (m *mockOperationResult) GetResponse() interface{} {
	return m.data
}

// GetRawResponse returns the original response before transformation.
func (m *mockOperationResult) GetRawResponse() interface{} {
	return m.data
}

// GetStatusCode returns the HTTP status code (for HTTP integrations).
func (m *mockOperationResult) GetStatusCode() int {
	return m.statusCode
}

// GetMetadata returns execution metadata.
func (m *mockOperationResult) GetMetadata() map[string]interface{} {
	return map[string]interface{}{
		"mock": true,
	}
}
