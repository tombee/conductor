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
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/pkg/workflow"
)

// NewConnectorsCommand creates the connectors command for discoverability.
func NewConnectorsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connectors",
		Short: "Discover and explore available connectors",
		Long: `Discover and explore available connectors and their operations.

Connectors provide structured operations for external integrations.
Use these commands to explore builtin connectors (file, shell, http, transform)
and configured connectors defined in workflows.`,
	}

	cmd.AddCommand(newConnectorsListCommand())
	cmd.AddCommand(newConnectorsShowCommand())
	cmd.AddCommand(newConnectorsOperationCommand())

	return cmd
}

// newConnectorsListCommand creates the 'connectors list' command.
func newConnectorsListCommand() *cobra.Command {
	var workflowPath string

	cmd := &cobra.Command{
		Use:   "list [workflow]",
		Short: "List available connectors",
		Long: `List available connectors including builtins and configured connectors.

Without a workflow argument, lists only builtin connectors.
With a workflow, also shows connectors configured in that workflow.

See also: conductor connectors show, conductor examples list`,
		Example: `  # Example 1: List builtin connectors
  conductor connectors list

  # Example 2: List connectors from a workflow
  conductor connectors list workflow.yaml

  # Example 3: Get connector list as JSON
  conductor connectors list --json

  # Example 4: Extract connector names for scripting
  conductor connectors list --json | jq -r '.connectors[].name'`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				workflowPath = args[0]
			}
			return runConnectorsList(cmd, workflowPath)
		},
	}

	return cmd
}

// newConnectorsShowCommand creates the 'connectors show' command.
func newConnectorsShowCommand() *cobra.Command {
	var workflowPath string

	cmd := &cobra.Command{
		Use:   "show <name> [workflow]",
		Short: "Show operations for a connector",
		Long: `Show operations and parameters for a specific connector.

For builtin connectors (file, shell, http, transform), no workflow is needed.
For configured connectors, provide the workflow path.

See also: conductor connectors list, conductor connectors operation`,
		Example: `  # Example 1: Show builtin connector operations
  conductor connectors show file

  # Example 2: Show configured connector from workflow
  conductor connectors show github workflow.yaml

  # Example 3: Get connector details as JSON
  conductor connectors show http --json

  # Example 4: List all operations for a connector
  conductor connectors show transform --json | jq -r '.connector.operations[]'`,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completion.CompleteConnectorNames,
		SilenceUsage:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			connectorName := args[0]
			if len(args) > 1 {
				workflowPath = args[1]
			}
			return runConnectorsShow(cmd, connectorName, workflowPath)
		},
	}

	return cmd
}

// newConnectorsOperationCommand creates the 'connectors operation' command.
func newConnectorsOperationCommand() *cobra.Command {
	var workflowPath string

	cmd := &cobra.Command{
		Use:   "operation <connector.operation> [workflow]",
		Short: "Show detailed operation information",
		Long: `Show detailed information about a specific connector operation.

Displays parameters, input schema, descriptions, and usage examples.

Examples:
  conductor connectors operation file.read
  conductor connectors operation github.create_issue workflow.yaml
  conductor connectors operation shell.run --json
`,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completion.CompleteConnectorOperations,
		SilenceUsage:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			reference := args[0]
			if len(args) > 1 {
				workflowPath = args[1]
			}
			return runConnectorsOperation(cmd, reference, workflowPath)
		},
	}

	return cmd
}

// ConnectorInfo holds information about a connector for display.
type ConnectorInfo struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"` // "builtin" or "configured"
	Description    string   `json:"description,omitempty"`
	OperationCount int      `json:"operation_count"`
	Operations     []string `json:"operations,omitempty"`
}

// OperationInfo holds detailed information about a connector operation.
type OperationInfo struct {
	Connector   string                 `json:"connector"`
	Operation   string                 `json:"operation"`
	Description string                 `json:"description"`
	Parameters  []ParameterInfo        `json:"parameters"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
	Examples    []string               `json:"examples,omitempty"`
}

// ParameterInfo holds information about an operation parameter.
type ParameterInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

// runConnectorsList lists available connectors.
func runConnectorsList(cmd *cobra.Command, workflowPath string) error {
	useJSON := shared.GetJSON()

	// Start with builtin connectors
	connectors := []ConnectorInfo{
		{
			Name:           "file",
			Type:           "builtin",
			Description:    operation.GetBuiltinDescription("file"),
			OperationCount: len(operation.GetBuiltinOperations("file")),
			Operations:     operation.GetBuiltinOperations("file"),
		},
		{
			Name:           "shell",
			Type:           "builtin",
			Description:    operation.GetBuiltinDescription("shell"),
			OperationCount: len(operation.GetBuiltinOperations("shell")),
			Operations:     operation.GetBuiltinOperations("shell"),
		},
		{
			Name:           "http",
			Type:           "builtin",
			Description:    "HTTP client operations (get, post, put, delete, patch)",
			OperationCount: 5,
			Operations:     []string{"get", "post", "put", "delete", "patch"},
		},
		{
			Name:           "transform",
			Type:           "builtin",
			Description:    operation.GetBuiltinDescription("transform"),
			OperationCount: len(operation.GetBuiltinOperations("transform")),
			Operations:     operation.GetBuiltinOperations("transform"),
		},
	}

	// Load workflow if provided to get configured connectors
	if workflowPath != "" {
		data, err := os.ReadFile(workflowPath)
		if err != nil {
			if useJSON {
				shared.EmitJSONError("connectors_list", []shared.JSONError{
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
				shared.EmitJSONError("connectors_list", []shared.JSONError{
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

		// Add configured connectors
		for name, connDef := range def.Connectors {
			ops := make([]string, 0, len(connDef.Operations))
			for opName := range connDef.Operations {
				ops = append(ops, opName)
			}
			sort.Strings(ops)

			connectors = append(connectors, ConnectorInfo{
				Name:           name,
				Type:           "configured",
				Description:    fmt.Sprintf("Configured connector from %s", connDef.From),
				OperationCount: len(ops),
				Operations:     ops,
			})
		}
	}

	// Sort connectors by name
	sort.Slice(connectors, func(i, j int) bool {
		return connectors[i].Name < connectors[j].Name
	})

	// Output results
	if useJSON {
		type listResponse struct {
			shared.JSONResponse
			Connectors []ConnectorInfo `json:"connectors"`
		}

		resp := listResponse{
			JSONResponse: shared.JSONResponse{
				Version: "1.0",
				Command: "connectors_list",
				Success: true,
			},
			Connectors: connectors,
		}

		return shared.EmitJSON(resp)
	}

	// Human-readable output
	cmd.Printf("Available Connectors (%d):\n\n", len(connectors))
	cmd.Printf("%-15s %-12s %-10s %s\n", "NAME", "TYPE", "OPERATIONS", "DESCRIPTION")
	cmd.Printf("%-15s %-12s %-10s %s\n", strings.Repeat("-", 15), strings.Repeat("-", 12), strings.Repeat("-", 10), strings.Repeat("-", 40))

	for _, conn := range connectors {
		cmd.Printf("%-15s %-12s %-10d %s\n",
			conn.Name,
			conn.Type,
			conn.OperationCount,
			conn.Description,
		)
	}

	if workflowPath == "" {
		cmd.Printf("\nTo see configured connectors, provide a workflow path:\n")
		cmd.Printf("  conductor connectors list <workflow.yaml>\n")
	}

	return nil
}

// runConnectorsShow shows operations for a specific connector.
func runConnectorsShow(cmd *cobra.Command, connectorName, workflowPath string) error {
	useJSON := shared.GetJSON()

	var info ConnectorInfo
	var found bool

	// Check if it's a builtin connector
	if operation.IsBuiltin(connectorName) {
		info = ConnectorInfo{
			Name:           connectorName,
			Type:           "builtin",
			Description:    operation.GetBuiltinDescription(connectorName),
			Operations:     operation.GetBuiltinOperations(connectorName),
			OperationCount: len(operation.GetBuiltinOperations(connectorName)),
		}
		found = true
	} else if workflowPath != "" {
		// Try to load from workflow
		data, err := os.ReadFile(workflowPath)
		if err != nil {
			if useJSON {
				shared.EmitJSONError("connectors_show", []shared.JSONError{
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
				shared.EmitJSONError("connectors_show", []shared.JSONError{
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

		if connDef, exists := def.Connectors[connectorName]; exists {
			ops := make([]string, 0, len(connDef.Operations))
			for opName := range connDef.Operations {
				ops = append(ops, opName)
			}
			sort.Strings(ops)

			info = ConnectorInfo{
				Name:           connectorName,
				Type:           "configured",
				Description:    fmt.Sprintf("Base URL: %s", connDef.BaseURL),
				Operations:     ops,
				OperationCount: len(ops),
			}
			found = true
		}
	}

	if !found {
		// Generate "did you mean?" suggestions
		suggestions := suggestConnectors(connectorName, workflowPath)
		suggestionText := ""
		if len(suggestions) > 0 {
			suggestionText = fmt.Sprintf("\n\nDid you mean?\n  %s", strings.Join(suggestions, "\n  "))
		}

		if useJSON {
			shared.EmitJSONError("connectors_show", []shared.JSONError{
				{
					Code:       shared.ErrorCodeNotFound,
					Message:    fmt.Sprintf("connector %q not found", connectorName),
					Suggestion: fmt.Sprintf("Available connectors: %s%s", strings.Join(suggestions, ", "), suggestionText),
				},
			})
			return &shared.ExitError{Code: 1, Message: ""}
		}

		msg := fmt.Sprintf("Connector %q not found%s", connectorName, suggestionText)
		if workflowPath == "" {
			msg += "\n\nFor configured connectors, provide a workflow path:\n  conductor connectors show <name> <workflow.yaml>"
		}
		return &shared.ExitError{Code: 1, Message: msg}
	}

	// Output results
	if useJSON {
		type showResponse struct {
			shared.JSONResponse
			Connector ConnectorInfo `json:"connector"`
		}

		resp := showResponse{
			JSONResponse: shared.JSONResponse{
				Version: "1.0",
				Command: "connectors_show",
				Success: true,
			},
			Connector: info,
		}

		return shared.EmitJSON(resp)
	}

	// Human-readable output
	cmd.Printf("Connector: %s\n", info.Name)
	cmd.Printf("Type: %s\n", info.Type)
	if info.Description != "" {
		cmd.Printf("Description: %s\n", info.Description)
	}
	cmd.Printf("\nOperations (%d):\n", len(info.Operations))
	for _, op := range info.Operations {
		cmd.Printf("  %s.%s\n", info.Name, op)
	}

	cmd.Printf("\nUse 'conductor connectors operation %s.<operation>' for detailed info\n", info.Name)

	return nil
}

// runConnectorsOperation shows detailed information about a specific operation.
func runConnectorsOperation(cmd *cobra.Command, reference, workflowPath string) error {
	useJSON := shared.GetJSON()

	// Parse connector.operation format
	parts := strings.Split(reference, ".")
	if len(parts) != 2 {
		if useJSON {
			shared.EmitJSONError("connectors_operation", []shared.JSONError{
				{
					Code:       shared.ErrorCodeInvalidInput,
					Message:    fmt.Sprintf("invalid operation reference: %q", reference),
					Suggestion: "Use format: connector.operation (e.g., file.read, github.create_issue)",
				},
			})
			return &shared.ExitError{Code: 2, Message: ""}
		}
		return &shared.ExitError{Code: 2, Message: fmt.Sprintf("invalid operation reference %q, use format: connector.operation", reference)}
	}

	connectorName := parts[0]
	operationName := parts[1]

	// Get operation info
	opInfo, err := getOperationInfo(connectorName, operationName, workflowPath)
	if err != nil {
		// Generate suggestions
		suggestions := suggestConnectors(connectorName, workflowPath)
		suggestionText := ""
		if len(suggestions) > 0 {
			suggestionText = fmt.Sprintf("\n\nDid you mean?\n  %s", strings.Join(suggestions, "\n  "))
		}

		if useJSON {
			shared.EmitJSONError("connectors_operation", []shared.JSONError{
				{
					Code:       shared.ErrorCodeNotFound,
					Message:    err.Error(),
					Suggestion: fmt.Sprintf("Available connectors: %s%s", strings.Join(suggestions, ", "), suggestionText),
				},
			})
			return &shared.ExitError{Code: 1, Message: ""}
		}
		return &shared.ExitError{Code: 1, Message: fmt.Sprintf("%v%s", err, suggestionText)}
	}

	// Output results
	if useJSON {
		type operationResponse struct {
			shared.JSONResponse
			Operation OperationInfo `json:"operation"`
		}

		resp := operationResponse{
			JSONResponse: shared.JSONResponse{
				Version: "1.0",
				Command: "connectors_operation",
				Success: true,
			},
			Operation: opInfo,
		}

		return shared.EmitJSON(resp)
	}

	// Human-readable output
	cmd.Printf("Operation: %s.%s\n", opInfo.Connector, opInfo.Operation)
	if opInfo.Description != "" {
		cmd.Printf("Description: %s\n", opInfo.Description)
	}

	cmd.Printf("\nParameters:\n")
	if len(opInfo.Parameters) == 0 {
		cmd.Printf("  (none)\n")
	} else {
		for _, param := range opInfo.Parameters {
			required := ""
			if param.Required {
				required = " (required)"
			}
			cmd.Printf("  %s: %s%s\n", param.Name, param.Type, required)
			if param.Description != "" {
				cmd.Printf("    %s\n", param.Description)
			}
		}
	}

	if len(opInfo.Examples) > 0 {
		cmd.Printf("\nExamples:\n")
		for _, example := range opInfo.Examples {
			cmd.Printf("  %s\n", example)
		}
	}

	return nil
}

// getOperationInfo retrieves detailed operation information.
func getOperationInfo(connectorName, operationName, workflowPath string) (OperationInfo, error) {
	info := OperationInfo{
		Connector: connectorName,
		Operation: operationName,
	}

	// Check builtin connectors first
	if operation.IsBuiltin(connectorName) {
		ops := operation.GetBuiltinOperations(connectorName)
		found := false
		for _, op := range ops {
			if op == operationName {
				found = true
				break
			}
		}

		if !found {
			return info, fmt.Errorf("operation %q not found for connector %q", operationName, connectorName)
		}

		// Add builtin-specific information
		info.Description = getBuiltinOperationDescription(connectorName, operationName)
		info.Parameters = getBuiltinOperationParameters(connectorName, operationName)
		info.Examples = getBuiltinOperationExamples(connectorName, operationName)

		return info, nil
	}

	// Try to load from workflow
	if workflowPath == "" {
		return info, fmt.Errorf("connector %q not found (provide workflow path for configured connectors)", connectorName)
	}

	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return info, fmt.Errorf("failed to read workflow file: %w", err)
	}

	def, err := workflow.ParseDefinition(data)
	if err != nil {
		return info, fmt.Errorf("failed to parse workflow: %w", err)
	}

	connDef, exists := def.Connectors[connectorName]
	if !exists {
		return info, fmt.Errorf("connector %q not found in workflow", connectorName)
	}

	opDef, exists := connDef.Operations[operationName]
	if !exists {
		return info, fmt.Errorf("operation %q not found for connector %q", operationName, connectorName)
	}

	// Extract operation information
	info.Description = fmt.Sprintf("%s %s", opDef.Method, opDef.Path)
	info.Parameters = extractParametersFromOperation(&opDef)
	info.Examples = []string{
		fmt.Sprintf("- %s.%s:", connectorName, operationName),
		"    param1: value1",
		"    param2: value2",
	}

	return info, nil
}

// extractParametersFromOperation extracts parameter information from an operation definition.
func extractParametersFromOperation(opDef *workflow.OperationDefinition) []ParameterInfo {
	params := []ParameterInfo{}

	// Extract from request schema if available
	if opDef.RequestSchema != nil {
		if props, ok := opDef.RequestSchema["properties"].(map[string]interface{}); ok {
			requiredList := []string{}
			if req, ok := opDef.RequestSchema["required"].([]interface{}); ok {
				for _, r := range req {
					if s, ok := r.(string); ok {
						requiredList = append(requiredList, s)
					}
				}
			}

			for name, propDef := range props {
				param := ParameterInfo{
					Name:     name,
					Required: stringSliceContains(requiredList, name),
				}

				if p, ok := propDef.(map[string]interface{}); ok {
					if t, ok := p["type"].(string); ok {
						param.Type = t
					}
					if d, ok := p["description"].(string); ok {
						param.Description = d
					}
				}

				params = append(params, param)
			}
		}
	}

	// Sort parameters by name
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	return params
}

// suggestConnectors generates "did you mean?" suggestions using Levenshtein distance.
func suggestConnectors(name, workflowPath string) []string {
	builtins := []string{"file", "shell", "http", "transform"}
	candidates := make([]string, len(builtins))
	copy(candidates, builtins)

	// Add configured connectors if workflow provided
	if workflowPath != "" {
		data, err := os.ReadFile(workflowPath)
		if err == nil {
			def, err := workflow.ParseDefinition(data)
			if err == nil {
				for connName := range def.Connectors {
					candidates = append(candidates, connName)
				}
			}
		}
	}

	// Calculate distances and find closest matches
	type suggestion struct {
		name     string
		distance int
	}

	suggestions := []suggestion{}
	for _, candidate := range candidates {
		dist := levenshteinDistance(name, candidate)
		if dist <= 3 { // Only suggest if distance is 3 or less
			suggestions = append(suggestions, suggestion{name: candidate, distance: dist})
		}
	}

	// Sort by distance, then alphabetically
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].distance != suggestions[j].distance {
			return suggestions[i].distance < suggestions[j].distance
		}
		return suggestions[i].name < suggestions[j].name
	})

	// Return top 3 suggestions
	result := []string{}
	for i := 0; i < len(suggestions) && i < 3; i++ {
		result = append(result, suggestions[i].name)
	}

	return result
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create distance matrix
	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Calculate distances
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

// Helper functions

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func stringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Builtin operation metadata

func getBuiltinOperationDescription(connector, operation string) string {
	descriptions := map[string]map[string]string{
		"file": {
			"read":       "Read file with auto-detected format (json, yaml, csv, or text)",
			"read_text":  "Read file as plain text",
			"read_json":  "Read and parse file as JSON",
			"read_yaml":  "Read and parse file as YAML",
			"read_csv":   "Read and parse file as CSV",
			"read_lines": "Read file as array of lines",
			"write":      "Write file with auto-detected format based on extension",
			"write_text": "Write plain text to file",
			"write_json": "Write data as pretty-printed JSON",
			"write_yaml": "Write data as YAML",
			"append":     "Append text to existing file",
			"render":     "Render Go template to file",
			"list":       "List directory contents",
			"exists":     "Check if path exists",
			"stat":       "Get file metadata (size, modified time, etc.)",
			"mkdir":      "Create directory (with parents if needed)",
			"copy":       "Copy file or directory",
			"move":       "Move or rename file or directory",
			"delete":     "Delete file or directory",
		},
		"shell": {
			"run": "Execute shell command (prefer array form for safety)",
		},
		"http": {
			"get":    "Send HTTP GET request",
			"post":   "Send HTTP POST request",
			"put":    "Send HTTP PUT request",
			"delete": "Send HTTP DELETE request",
			"patch":  "Send HTTP PATCH request",
		},
		"transform": {
			"parse_json": "Parse JSON from text (handles markdown code blocks)",
			"parse_xml":  "Parse XML to JSON structure (XXE-safe)",
			"extract":    "Extract nested fields using jq expressions",
			"split":      "Pass-through array for foreach iteration",
			"map":        "Transform array elements using jq expression",
			"filter":     "Filter array elements using jq predicate",
			"flatten":    "Flatten nested arrays",
			"sort":       "Sort array by value or expression",
			"group":      "Group array by key expression",
			"merge":      "Merge objects (shallow or deep)",
			"concat":     "Concatenate arrays",
		},
	}

	if ops, ok := descriptions[connector]; ok {
		if desc, ok := ops[operation]; ok {
			return desc
		}
	}

	return ""
}

func getBuiltinOperationParameters(connector, operation string) []ParameterInfo {
	params := map[string]map[string][]ParameterInfo{
		"file": {
			"read": {
				{Name: "path", Type: "string", Required: true, Description: "File path (supports ./, $out/, $temp/, ~/ prefixes)"},
				{Name: "extract", Type: "string", Required: false, Description: "JSONPath expression to extract data"},
			},
			"read_text": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
			},
			"read_json": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
				{Name: "extract", Type: "string", Required: false, Description: "JSONPath expression to extract data"},
			},
			"read_yaml": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
				{Name: "extract", Type: "string", Required: false, Description: "JSONPath expression to extract data"},
			},
			"read_csv": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
				{Name: "has_header", Type: "boolean", Required: false, Description: "Whether CSV has header row (default: true)"},
			},
			"read_lines": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
			},
			"write": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
				{Name: "content", Type: "string|object", Required: true, Description: "Content to write"},
			},
			"write_text": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
				{Name: "content", Type: "string", Required: true, Description: "Text content"},
			},
			"write_json": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
				{Name: "content", Type: "object", Required: true, Description: "Data to serialize as JSON"},
			},
			"write_yaml": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
				{Name: "content", Type: "object", Required: true, Description: "Data to serialize as YAML"},
			},
			"append": {
				{Name: "path", Type: "string", Required: true, Description: "File path"},
				{Name: "content", Type: "string", Required: true, Description: "Text to append"},
			},
			"render": {
				{Name: "path", Type: "string", Required: true, Description: "Output file path"},
				{Name: "template", Type: "string", Required: true, Description: "Go template string"},
				{Name: "data", Type: "object", Required: true, Description: "Data for template rendering"},
			},
			"list": {
				{Name: "path", Type: "string", Required: true, Description: "Directory path"},
				{Name: "recursive", Type: "boolean", Required: false, Description: "List recursively (default: false)"},
			},
			"exists": {
				{Name: "path", Type: "string", Required: true, Description: "File or directory path"},
			},
			"stat": {
				{Name: "path", Type: "string", Required: true, Description: "File or directory path"},
			},
			"mkdir": {
				{Name: "path", Type: "string", Required: true, Description: "Directory path"},
			},
			"copy": {
				{Name: "source", Type: "string", Required: true, Description: "Source path"},
				{Name: "destination", Type: "string", Required: true, Description: "Destination path"},
			},
			"move": {
				{Name: "source", Type: "string", Required: true, Description: "Source path"},
				{Name: "destination", Type: "string", Required: true, Description: "Destination path"},
			},
			"delete": {
				{Name: "path", Type: "string", Required: true, Description: "File or directory path"},
			},
		},
		"shell": {
			"run": {
				{Name: "command", Type: "string|array", Required: true, Description: "Command to execute (array form recommended for safety)"},
			},
		},
		"http": {
			"get": {
				{Name: "url", Type: "string", Required: true, Description: "Request URL"},
				{Name: "headers", Type: "object", Required: false, Description: "HTTP headers"},
			},
			"post": {
				{Name: "url", Type: "string", Required: true, Description: "Request URL"},
				{Name: "body", Type: "object", Required: false, Description: "Request body"},
				{Name: "headers", Type: "object", Required: false, Description: "HTTP headers"},
			},
			"put": {
				{Name: "url", Type: "string", Required: true, Description: "Request URL"},
				{Name: "body", Type: "object", Required: false, Description: "Request body"},
				{Name: "headers", Type: "object", Required: false, Description: "HTTP headers"},
			},
			"delete": {
				{Name: "url", Type: "string", Required: true, Description: "Request URL"},
				{Name: "headers", Type: "object", Required: false, Description: "HTTP headers"},
			},
			"patch": {
				{Name: "url", Type: "string", Required: true, Description: "Request URL"},
				{Name: "body", Type: "object", Required: false, Description: "Request body"},
				{Name: "headers", Type: "object", Required: false, Description: "HTTP headers"},
			},
		},
		"transform": {
			"parse_json": {
				{Name: "data", Type: "string", Required: true, Description: "JSON string or text containing JSON"},
			},
			"parse_xml": {
				{Name: "data", Type: "string", Required: true, Description: "XML string to parse"},
				{Name: "attribute_prefix", Type: "string", Required: false, Description: "Prefix for attributes (default: @)"},
				{Name: "strip_namespaces", Type: "boolean", Required: false, Description: "Strip namespace prefixes (default: true)"},
			},
			"extract": {
				{Name: "data", Type: "any", Required: true, Description: "Data to extract from"},
				{Name: "expr", Type: "string", Required: true, Description: "jq expression for extraction"},
			},
			"split": {
				{Name: "data", Type: "array", Required: true, Description: "Array to pass through for foreach"},
			},
			"map": {
				{Name: "data", Type: "array", Required: true, Description: "Array to transform"},
				{Name: "expr", Type: "string", Required: true, Description: "jq transformation expression"},
			},
			"filter": {
				{Name: "data", Type: "array", Required: true, Description: "Array to filter"},
				{Name: "expr", Type: "string", Required: true, Description: "jq predicate expression"},
			},
			"flatten": {
				{Name: "data", Type: "array", Required: true, Description: "Nested array to flatten"},
			},
			"sort": {
				{Name: "data", Type: "array", Required: true, Description: "Array to sort"},
				{Name: "expr", Type: "string", Required: false, Description: "jq expression for sort key (optional)"},
			},
			"group": {
				{Name: "data", Type: "array", Required: true, Description: "Array to group"},
				{Name: "expr", Type: "string", Required: true, Description: "jq expression for grouping key"},
			},
			"merge": {
				{Name: "data", Type: "array|object", Required: false, Description: "First object to merge (or array of objects)"},
				{Name: "sources", Type: "array", Required: false, Description: "Array of objects to merge"},
				{Name: "strategy", Type: "string", Required: false, Description: "Merge strategy: shallow (default) or deep"},
			},
			"concat": {
				{Name: "data", Type: "array", Required: false, Description: "First array (or array of arrays)"},
				{Name: "sources", Type: "array", Required: false, Description: "Arrays to concatenate"},
			},
		},
	}

	if ops, ok := params[connector]; ok {
		if paramList, ok := ops[operation]; ok {
			return paramList
		}
	}

	return []ParameterInfo{}
}

func getBuiltinOperationExamples(connector, operation string) []string {
	examples := map[string]map[string][]string{
		"file": {
			"read":       {"- file.read: ./config.json", "- file.read:\n    path: ./data.yaml\n    extract: $.database.url"},
			"read_text":  {"- file.read_text: ./README.md"},
			"read_json":  {"- file.read_json: ./package.json"},
			"read_yaml":  {"- file.read_yaml: ./config.yaml"},
			"read_csv":   {"- file.read_csv:\n    path: ./data.csv\n    has_header: true"},
			"read_lines": {"- file.read_lines: ./log.txt"},
			"write": {"- file.write:\n    path: $out/result.json\n    content:\n      status: success"},
			"write_text":  {"- file.write_text:\n    path: $out/output.txt\n    content: Hello World"},
			"write_json":  {"- file.write_json:\n    path: $out/data.json\n    content:\n      key: value"},
			"write_yaml":  {"- file.write_yaml:\n    path: $out/config.yaml\n    content:\n      setting: value"},
			"append":      {"- file.append:\n    path: $out/log.txt\n    content: Log entry\\n"},
			"list":        {"- file.list:\n    path: ./src\n    recursive: true"},
			"exists":      {"- file.exists: ./config.json"},
			"stat":        {"- file.stat: ./README.md"},
			"mkdir":       {"- file.mkdir: $out/reports"},
			"copy":        {"- file.copy:\n    source: ./template.txt\n    destination: $out/output.txt"},
			"move":        {"- file.move:\n    source: ./old.txt\n    destination: ./new.txt"},
			"delete":      {"- file.delete: $temp/temp-file.txt"},
		},
		"shell": {
			"run": {"- shell.run:\n    command: [git, status]", "- shell.run:\n    command: [git, commit, -m, \"{{.inputs.message}}\"]"},
		},
		"http": {
			"get":    {"- http.get:\n    url: https://api.example.com/data\n    headers:\n      Authorization: Bearer {{.env.API_TOKEN}}"},
			"post":   {"- http.post:\n    url: https://api.example.com/items\n    body:\n      name: Test\n    headers:\n      Content-Type: application/json"},
			"put":    {"- http.put:\n    url: https://api.example.com/items/1\n    body:\n      name: Updated"},
			"delete": {"- http.delete:\n    url: https://api.example.com/items/1"},
			"patch":  {"- http.patch:\n    url: https://api.example.com/items/1\n    body:\n      status: active"},
		},
		"transform": {
			"parse_json": {
				"- transform.parse_json: '{{.steps.llm_response.output}}'",
				"- transform.parse_json:\n    data: '{{.steps.http_response.body}}'",
			},
			"parse_xml": {
				"- transform.parse_xml:\n    data: '{{.steps.http_response.body}}'",
				"- transform.parse_xml:\n    data: '{{.steps.legacy_api.response}}'\n    attribute_prefix: $\n    strip_namespaces: false",
			},
			"extract": {
				"- transform.extract:\n    data: '{{.steps.json_data}}'\n    expr: '.issues[0].title'",
				"- transform.extract:\n    data: '{{.steps.nested_data}}'\n    expr: '.items | map(.name)'",
			},
			"split": {
				"- transform.split: '{{.steps.analyze.issues}}'",
			},
			"map": {
				"- transform.map:\n    data: '{{.steps.items}}'\n    expr: '.name'",
				"- transform.map:\n    data: '{{.steps.users}}'\n    expr: '{id: .id, email: .email}'",
			},
			"filter": {
				"- transform.filter:\n    data: '{{.steps.items}}'\n    expr: '.status == \"active\"'",
				"- transform.filter:\n    data: '{{.steps.users}}'\n    expr: '.age >= 18'",
			},
			"flatten": {
				"- transform.flatten: '{{.steps.nested_arrays}}'",
			},
			"sort": {
				"- transform.sort: '{{.steps.items}}'",
				"- transform.sort:\n    data: '{{.steps.users}}'\n    expr: '.name'",
			},
			"group": {
				"- transform.group:\n    data: '{{.steps.items}}'\n    expr: '.category'",
			},
			"merge": {
				"- transform.merge:\n    sources:\n      - '{{.steps.config}}'\n      - '{{.steps.overrides}}'",
				"- transform.merge:\n    sources:\n      - '{{.steps.base}}'\n      - '{{.steps.custom}}'\n    strategy: deep",
			},
			"concat": {
				"- transform.concat:\n    sources:\n      - '{{.steps.list1}}'\n      - '{{.steps.list2}}'",
			},
		},
	}

	if ops, ok := examples[connector]; ok {
		if exampleList, ok := ops[operation]; ok {
			return exampleList
		}
	}

	return []string{}
}
