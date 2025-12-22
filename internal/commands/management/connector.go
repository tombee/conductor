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

package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/pkg/workflow"
)

// NewConnectorCommand creates the connector command with subcommands.
func NewConnectorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connector",
		Short: "Manage and test connectors",
		Long: `Manage and test connectors for external integrations.

Connectors provide deterministic, schema-validated operations for calling
external APIs without LLM involvement. Use these commands to test connectivity,
invoke operations, and view connector status.`,
	}

	cmd.AddCommand(newConnectorTestCommand())
	cmd.AddCommand(newConnectorInvokeCommand())
	cmd.AddCommand(newConnectorStatusCommand())

	return cmd
}

// newConnectorTestCommand creates the 'connector test' command.
func newConnectorTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <workflow> [connector]",
		Short: "Test connector connectivity and auth",
		Long: `Test connector connectivity and authentication.

If a connector name is provided, only that connector is tested.
Otherwise, all connectors in the workflow are tested.

Examples:
  conductor connector test workflow.yaml
  conductor connector test workflow.yaml github
`,
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE:         runConnectorTest,
	}

	return cmd
}

// newConnectorInvokeCommand creates the 'connector invoke' command.
func newConnectorInvokeCommand() *cobra.Command {
	var inputsFlag string

	cmd := &cobra.Command{
		Use:   "invoke <workflow> <connector.operation>",
		Short: "Invoke a connector operation",
		Long: `Invoke a connector operation with the specified inputs.

This executes a single connector operation and displays the result.
Use --inputs to provide operation inputs as JSON.

Examples:
  conductor connector invoke workflow.yaml github.list_repos --inputs '{"username":"octocat"}'
  conductor connector invoke workflow.yaml slack.post_message --inputs '{"channel":"#general","text":"Hello"}'
`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectorInvoke(cmd, args[0], args[1], inputsFlag)
		},
	}

	cmd.Flags().StringVar(&inputsFlag, "inputs", "{}", "Operation inputs as JSON")

	return cmd
}

// newConnectorStatusCommand creates the 'connector status' command.
func newConnectorStatusCommand() *cobra.Command {
	var showRateLimits bool

	cmd := &cobra.Command{
		Use:   "status <workflow>",
		Short: "Show connector status",
		Long: `Show status of all connectors in a workflow.

Displays connector information including operations, auth status,
and rate limit configuration.

Examples:
  conductor connector status workflow.yaml
  conductor connector status workflow.yaml --rate-limits
`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectorStatus(cmd, args[0], showRateLimits)
		},
	}

	cmd.Flags().BoolVar(&showRateLimits, "rate-limits", false, "Show detailed rate limit status")

	return cmd
}

// runConnectorTest tests connector connectivity.
func runConnectorTest(cmd *cobra.Command, args []string) error {
	workflowPath := args[0]
	targetConnector := ""
	if len(args) > 1 {
		targetConnector = args[1]
	}

	useJSON := shared.GetJSON()

	// Load workflow
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		if useJSON {
			shared.EmitJSONError("connector_test", []shared.JSONError{
				{
					Code:       shared.ErrorCodeFileNotFound,
					Message:    fmt.Sprintf("failed to read workflow file: %v", err),
					Suggestion: "Check that the file path is correct",
				},
			})
			return &shared.ExitError{Code: 2, Message: ""}
		}
		return &shared.ExitError{Code: 2, Message: fmt.Sprintf("failed to read workflow file: %v", err)}
	}

	def, err := workflow.ParseDefinition(data)
	if err != nil {
		if useJSON {
			shared.EmitJSONError("connector_test", []shared.JSONError{
				{
					Code:       shared.ErrorCodeSchemaViolation,
					Message:    fmt.Sprintf("failed to parse workflow: %v", err),
					Suggestion: "Run 'conductor validate' to check workflow syntax",
				},
			})
			return &shared.ExitError{Code: 1, Message: ""}
		}
		return &shared.ExitError{Code: 1, Message: fmt.Sprintf("failed to parse workflow: %v", err)}
	}

	// Filter connectors if specific one requested
	connectorsToTest := make(map[string]workflow.ConnectorDefinition)
	if targetConnector != "" {
		connDef, exists := def.Connectors[targetConnector]
		if !exists {
			if useJSON {
				shared.EmitJSONError("connector_test", []shared.JSONError{
					{
						Code:    shared.ErrorCodeNotFound,
						Message: fmt.Sprintf("connector %q not found in workflow", targetConnector),
						Suggestion: fmt.Sprintf("Available connectors: %v", getConnectorNames(def.Connectors)),
					},
				})
				return &shared.ExitError{Code: 1, Message: ""}
			}
			return &shared.ExitError{Code: 1, Message: fmt.Sprintf("connector %q not found in workflow", targetConnector)}
		}
		connectorsToTest[targetConnector] = connDef
	} else {
		connectorsToTest = def.Connectors
	}

	// Test each connector
	results := make([]connectorTestResult, 0, len(connectorsToTest))
	ctx := context.Background()

	for name, connDef := range connectorsToTest {
		result := testConnector(ctx, name, &connDef)
		results = append(results, result)
	}

	// Sort results by name for consistent output
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	// Output results
	if useJSON {
		type testResponse struct {
			shared.JSONResponse
			Results []connectorTestResult `json:"results"`
		}

		resp := testResponse{
			JSONResponse: shared.JSONResponse{
				Version: "1.0",
				Command: "connector_test",
				Success: allTestsPassed(results),
			},
			Results: results,
		}

		return shared.EmitJSON(resp)
	}

	// Human-readable output
	allPassed := true
	for _, result := range results {
		if result.Success {
			cmd.Printf("✓ %s: Connected (%s, %dms)\n", result.Name, result.BaseURL, result.LatencyMs)
			if result.RateLimitInfo != "" {
				cmd.Printf("  %s\n", result.RateLimitInfo)
			}
		} else {
			cmd.Printf("✗ %s: %s\n", result.Name, result.Error)
			allPassed = false
		}
	}

	if !allPassed {
		return &shared.ExitError{Code: 1, Message: "some connectors failed connectivity test"}
	}

	return nil
}

// runConnectorInvoke invokes a connector operation.
func runConnectorInvoke(cmd *cobra.Command, workflowPath, reference, inputsJSON string) error {
	useJSON := shared.GetJSON()

	// Parse inputs
	var inputs map[string]interface{}
	if err := json.Unmarshal([]byte(inputsJSON), &inputs); err != nil {
		if useJSON {
			shared.EmitJSONError("connector_invoke", []shared.JSONError{
				{
					Code:       shared.ErrorCodeInvalidInput,
					Message:    fmt.Sprintf("invalid inputs JSON: %v", err),
					Suggestion: "Ensure --inputs is valid JSON",
				},
			})
			return &shared.ExitError{Code: 2, Message: ""}
		}
		return &shared.ExitError{Code: 2, Message: fmt.Sprintf("invalid inputs JSON: %v", err)}
	}

	// Load workflow
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		if useJSON {
			shared.EmitJSONError("connector_invoke", []shared.JSONError{
				{
					Code:       shared.ErrorCodeFileNotFound,
					Message:    fmt.Sprintf("failed to read workflow file: %v", err),
					Suggestion: "Check that the file path is correct",
				},
			})
			return &shared.ExitError{Code: 2, Message: ""}
		}
		return &shared.ExitError{Code: 2, Message: fmt.Sprintf("failed to read workflow file: %v", err)}
	}

	def, err := workflow.ParseDefinition(data)
	if err != nil {
		if useJSON {
			shared.EmitJSONError("connector_invoke", []shared.JSONError{
				{
					Code:       shared.ErrorCodeSchemaViolation,
					Message:    fmt.Sprintf("failed to parse workflow: %v", err),
					Suggestion: "Run 'conductor validate' to check workflow syntax",
				},
			})
			return &shared.ExitError{Code: 1, Message: ""}
		}
		return &shared.ExitError{Code: 1, Message: fmt.Sprintf("failed to parse workflow: %v", err)}
	}

	// Create registry and load connectors
	config := connector.DefaultConfig()
	registry := connector.NewRegistry(config)
	if err := registry.LoadFromDefinition(def); err != nil {
		if useJSON {
			shared.EmitJSONError("connector_invoke", []shared.JSONError{
				{
					Code:       shared.ErrorCodeInternal,
					Message:    fmt.Sprintf("failed to load connectors: %v", err),
					Suggestion: "Check connector definitions in workflow",
				},
			})
			return &shared.ExitError{Code: 1, Message: ""}
		}
		return &shared.ExitError{Code: 1, Message: fmt.Sprintf("failed to load connectors: %v", err)}
	}

	// Execute operation
	ctx := context.Background()
	result, err := registry.Execute(ctx, reference, inputs)
	if err != nil {
		if useJSON {
			shared.EmitJSONError("connector_invoke", []shared.JSONError{
				{
					Code:       shared.ErrorCodeExecutionFailed,
					Message:    fmt.Sprintf("operation failed: %v", err),
					Suggestion: "Check operation name and inputs",
				},
			})
			return &shared.ExitError{Code: 1, Message: ""}
		}
		return &shared.ExitError{Code: 1, Message: fmt.Sprintf("operation failed: %v", err)}
	}

	// Output result
	if useJSON {
		type invokeResponse struct {
			shared.JSONResponse
			Result   interface{}            `json:"result"`
			Metadata map[string]interface{} `json:"metadata,omitempty"`
		}

		resp := invokeResponse{
			JSONResponse: shared.JSONResponse{
				Version: "1.0",
				Command: "connector_invoke",
				Success: true,
			},
			Result:   result.Response,
			Metadata: result.Metadata,
		}

		return shared.EmitJSON(resp)
	}

	// Human-readable output
	cmd.Println("Operation Result:")
	resultJSON, err := json.MarshalIndent(result.Response, "  ", "  ")
	if err != nil {
		return &shared.ExitError{Code: 1, Message: fmt.Sprintf("failed to format result: %v", err)}
	}
	cmd.Printf("  %s\n", string(resultJSON))

	return nil
}

// runConnectorStatus shows connector status.
func runConnectorStatus(cmd *cobra.Command, workflowPath string, showRateLimits bool) error {
	useJSON := shared.GetJSON()

	// Load workflow
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		if useJSON {
			shared.EmitJSONError("connector_status", []shared.JSONError{
				{
					Code:       shared.ErrorCodeFileNotFound,
					Message:    fmt.Sprintf("failed to read workflow file: %v", err),
					Suggestion: "Check that the file path is correct",
				},
			})
			return &shared.ExitError{Code: 2, Message: ""}
		}
		return &shared.ExitError{Code: 2, Message: fmt.Sprintf("failed to read workflow file: %v", err)}
	}

	def, err := workflow.ParseDefinition(data)
	if err != nil {
		if useJSON {
			shared.EmitJSONError("connector_status", []shared.JSONError{
				{
					Code:       shared.ErrorCodeSchemaViolation,
					Message:    fmt.Sprintf("failed to parse workflow: %v", err),
					Suggestion: "Run 'conductor validate' to check workflow syntax",
				},
			})
			return &shared.ExitError{Code: 1, Message: ""}
		}
		return &shared.ExitError{Code: 1, Message: fmt.Sprintf("failed to parse workflow: %v", err)}
	}

	if len(def.Connectors) == 0 {
		if useJSON {
			type statusResponse struct {
				shared.JSONResponse
				Connectors []connectorStatus `json:"connectors"`
			}

			resp := statusResponse{
				JSONResponse: shared.JSONResponse{
					Version: "1.0",
					Command: "connector_status",
					Success: true,
				},
				Connectors: []connectorStatus{},
			}

			return shared.EmitJSON(resp)
		}

		cmd.Println("No connectors defined in workflow")
		return nil
	}

	// Build status for each connector
	statuses := make([]connectorStatus, 0, len(def.Connectors))
	for name, connDef := range def.Connectors {
		status := buildConnectorStatus(name, &connDef)
		statuses = append(statuses, status)
	}

	// Sort by name
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})

	// Output results
	if useJSON {
		type statusResponse struct {
			shared.JSONResponse
			Connectors []connectorStatus `json:"connectors"`
		}

		resp := statusResponse{
			JSONResponse: shared.JSONResponse{
				Version: "1.0",
				Command: "connector_status",
				Success: true,
			},
			Connectors: statuses,
		}

		return shared.EmitJSON(resp)
	}

	// Human-readable output
	cmd.Printf("Connectors (%d):\n\n", len(statuses))
	for _, status := range statuses {
		cmd.Printf("%s:\n", status.Name)
		if status.From != "" {
			cmd.Printf("  Package: %s\n", status.From)
		}
		cmd.Printf("  Base URL: %s\n", status.BaseURL)
		cmd.Printf("  Auth: %s\n", status.AuthType)
		cmd.Printf("  Operations: %d\n", len(status.Operations))
		for _, op := range status.Operations {
			cmd.Printf("    - %s (%s %s)\n", op, getOperationMethod(def.Connectors[status.Name], op), getOperationPath(def.Connectors[status.Name], op))
		}
		if showRateLimits && status.RateLimit != nil {
			cmd.Printf("  Rate Limit:\n")
			if status.RateLimit.RequestsPerSecond > 0 {
				cmd.Printf("    Per second: %d\n", status.RateLimit.RequestsPerSecond)
			}
			if status.RateLimit.RequestsPerMinute > 0 {
				cmd.Printf("    Per minute: %d\n", status.RateLimit.RequestsPerMinute)
			}
		}
		cmd.Println()
	}

	return nil
}

// connectorTestResult holds test results for a connector.
type connectorTestResult struct {
	Name          string `json:"name"`
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	BaseURL       string `json:"base_url,omitempty"`
	LatencyMs     int64  `json:"latency_ms,omitempty"`
	RateLimitInfo string `json:"rate_limit_info,omitempty"`
}

// connectorStatus holds status information for a connector.
type connectorStatus struct {
	Name       string              `json:"name"`
	From       string              `json:"from,omitempty"`
	BaseURL    string              `json:"base_url"`
	AuthType   string              `json:"auth_type"`
	Operations []string            `json:"operations"`
	RateLimit  *rateLimitStatus    `json:"rate_limit,omitempty"`
}

// rateLimitStatus holds rate limit configuration.
type rateLimitStatus struct {
	RequestsPerSecond int `json:"requests_per_second,omitempty"`
	RequestsPerMinute int `json:"requests_per_minute,omitempty"`
}

// testConnector tests connectivity for a single connector.
func testConnector(ctx context.Context, name string, def *workflow.ConnectorDefinition) connectorTestResult {
	result := connectorTestResult{
		Name:    name,
		BaseURL: def.BaseURL,
	}

	// Basic validation - check if base URL is accessible
	if def.BaseURL == "" {
		result.Error = "no base URL configured"
		return result
	}

	// Make a simple HEAD request to the base URL
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "HEAD", def.BaseURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		return result
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("connection failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.LatencyMs = time.Since(start).Milliseconds()
	result.Success = true

	// Add rate limit info if configured
	if def.RateLimit != nil {
		parts := []string{}
		if def.RateLimit.RequestsPerSecond > 0 {
			parts = append(parts, fmt.Sprintf("%.0f/sec", def.RateLimit.RequestsPerSecond))
		}
		if def.RateLimit.RequestsPerMinute > 0 {
			parts = append(parts, fmt.Sprintf("%d/min", def.RateLimit.RequestsPerMinute))
		}
		if len(parts) > 0 {
			result.RateLimitInfo = fmt.Sprintf("Rate limit: %s", strings.Join(parts, ", "))
		}
	}

	return result
}

// buildConnectorStatus builds status information for a connector.
func buildConnectorStatus(name string, def *workflow.ConnectorDefinition) connectorStatus {
	status := connectorStatus{
		Name:     name,
		From:     def.From,
		BaseURL:  def.BaseURL,
		AuthType: getAuthType(def),
	}

	// Extract operation names
	operations := make([]string, 0, len(def.Operations))
	for opName := range def.Operations {
		operations = append(operations, opName)
	}
	sort.Strings(operations)
	status.Operations = operations

	// Add rate limit if configured
	if def.RateLimit != nil {
		status.RateLimit = &rateLimitStatus{
			RequestsPerSecond: int(def.RateLimit.RequestsPerSecond),
			RequestsPerMinute: def.RateLimit.RequestsPerMinute,
		}
	}

	return status
}

// getAuthType returns a human-readable auth type.
func getAuthType(def *workflow.ConnectorDefinition) string {
	if def.Auth == nil {
		return "none"
	}

	authType := def.Auth.Type
	if authType == "" {
		// Infer type from fields
		if def.Auth.Token != "" {
			authType = "bearer"
		} else if def.Auth.Username != "" && def.Auth.Password != "" {
			authType = "basic"
		} else if def.Auth.Header != "" && def.Auth.Value != "" {
			authType = "api_key"
		} else {
			authType = "unknown"
		}
	}

	return authType
}

// getOperationMethod returns the HTTP method for an operation.
func getOperationMethod(connDef workflow.ConnectorDefinition, opName string) string {
	if op, exists := connDef.Operations[opName]; exists {
		return op.Method
	}
	return ""
}

// getOperationPath returns the path template for an operation.
func getOperationPath(connDef workflow.ConnectorDefinition, opName string) string {
	if op, exists := connDef.Operations[opName]; exists {
		return op.Path
	}
	return ""
}

// getConnectorNames returns a sorted list of connector names.
func getConnectorNames(connectors map[string]workflow.ConnectorDefinition) []string {
	names := make([]string, 0, len(connectors))
	for name := range connectors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// allTestsPassed returns true if all test results passed.
func allTestsPassed(results []connectorTestResult) bool {
	for _, result := range results {
		if !result.Success {
			return false
		}
	}
	return true
}
