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

package validate

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/output"
	"github.com/tombee/conductor/internal/permissions"
	"github.com/tombee/conductor/pkg/workflow"
	workflowschema "github.com/tombee/conductor/pkg/workflow/schema"
	"gopkg.in/yaml.v3"
)

// NewCommand creates the validate command
func NewCommand() *cobra.Command {
	var (
		schemaPath       string
		workspace        string
		profile          string
		checkPermissions bool
		providerName     string
	)

	cmd := &cobra.Command{
		Use:   "validate <workflow>",
		Short: "Validate workflow YAML syntax and schema",
		Annotations: map[string]string{
			"group": "execution",
		},
		Long: `Validate checks that a workflow file has valid YAML syntax and conforms
to the Conductor workflow schema. This validation does not require provider
configuration and only checks the workflow structure itself.

Profile Validation:
  --workspace, -w <name>   Workspace for profile resolution
  --profile, -p <name>     Profile to validate against workflow requirements

When --profile is specified, validates that all workflow requirements
are satisfied by the profile bindings.

Permission Validation (SPEC-141):
  --check-permissions      Validate permission enforcement capabilities
  --provider <name>        Provider to check against (default: anthropic)

When --check-permissions is specified, validates that the configured permissions
can be enforced by the LLM provider. Warns about unenforceable permissions.

See also: conductor run, conductor schema`,
		Example: `  # Example 1: Basic validation
  conductor validate workflow.yaml

  # Example 2: Validate with JSON output for parsing
  conductor validate workflow.yaml --json

  # Example 3: Validate and extract workflow metadata
  conductor validate workflow.yaml --json | jq '.workflow'

  # Example 4: Validate with profile configuration
  conductor validate workflow.yaml --workspace prod --profile default

  # Example 5: Use custom schema for validation
  conductor validate workflow.yaml --schema custom-schema.json`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true, // Don't print usage on validation errors
		SilenceErrors: true, // Don't print error message (we handle it ourselves)
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd, args, schemaPath, workspace, profile, checkPermissions, providerName)
		},
	}

	cmd.Flags().StringVar(&schemaPath, "schema", "", "Path to custom schema (default: embedded schema)")
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace for profile resolution")
	cmd.Flags().StringVarP(&profile, "profile", "p", "", "Profile to validate against workflow requirements")
	cmd.Flags().BoolVar(&checkPermissions, "check-permissions", false, "Validate permission enforcement capabilities")
	cmd.Flags().StringVar(&providerName, "provider", "anthropic", "Provider to check permissions against")

	return cmd
}

func runValidate(cmd *cobra.Command, args []string, schemaPath string, workspace, profile string, checkPermissions bool, providerName string) error {
	workflowPath := args[0]

	// Use global --json flag
	useJSON := shared.GetJSON()

	// Apply environment variable defaults for workspace and profile
	if workspace == "" {
		workspace = os.Getenv("CONDUCTOR_WORKSPACE")
	}
	if profile == "" {
		profile = os.Getenv("CONDUCTOR_PROFILE")
	}

	// Read workflow file
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		if useJSON {
			errors := []output.JSONError{
				{
					Code:       shared.ErrorCodeFileNotFound,
					Message:    fmt.Sprintf("failed to read workflow file: %v", err),
					Suggestion: "Check that the file path is correct and the file exists",
				},
			}
			output.EmitJSONError("validate", errors)
			return &shared.ExitError{Code: 2, Message: ""}
		}
		return &shared.ExitError{Code: 2, Message: fmt.Sprintf("failed to read workflow file: %v", err)}
	}

	// Collect all validation errors
	var validationErrors []output.JSONError

	// Step 1: Validate YAML syntax by parsing as YAML
	var yamlData interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		// Try to extract line number from YAML error
		line, col := extractYAMLErrorLocation(err)
		validationErrors = append(validationErrors, output.JSONError{
			Code:    shared.ErrorCodeInvalidYAML,
			Message: fmt.Sprintf("YAML syntax error: %v", err),
			Location: &output.JSONLocation{
				Line:   line,
				Column: col,
			},
			Suggestion: "Check YAML syntax and indentation",
		})
	}

	// Step 2: Validate against JSON Schema (if YAML is valid)
	if len(validationErrors) == 0 {
		schemaErrors := validateAgainstSchema(yamlData, schemaPath)
		validationErrors = append(validationErrors, schemaErrors...)
	}

	// Step 3: Validate semantic rules via Go validation (if schema is valid)
	var def *workflow.Definition
	if len(validationErrors) == 0 {
		def, err = workflow.ParseDefinition(data)
		if err != nil {
			validationErrors = append(validationErrors, output.JSONError{
				Code:       shared.ErrorCodeSchemaViolation,
				Message:    err.Error(),
				Suggestion: "Review workflow definition for semantic errors",
			})
		}
	}

	// Step 4: Run security validation (if semantic validation passed)
	var securityResult *workflow.SecurityValidationResult
	if len(validationErrors) == 0 && def != nil {
		securityResult = workflow.ValidateSecurity(def)

		// Convert security errors to validation errors
		for _, secErr := range securityResult.Errors {
			validationErrors = append(validationErrors, output.JSONError{
				Code:       shared.ErrorCodeSchemaViolation,
				Message:    fmt.Sprintf("[%s] %s", secErr.StepID, secErr.Message),
				Suggestion: secErr.Suggestion,
			})
		}
	}

	// Step 5: Run permission validation if requested (SPEC-141)
	var permissionResult *permissions.ValidationResult
	if checkPermissions && def != nil && def.Permissions != nil {
		// Merge workflow-level permissions to create effective context
		permCtx := permissions.Merge(def.Permissions, nil)
		permissionResult = permissions.ValidateEnforcement(providerName, permCtx)
	}

	// Report errors
	if len(validationErrors) > 0 {
		if useJSON {
			output.EmitJSONError("validate", validationErrors)
			return &shared.ExitError{Code: 1, Message: ""}
		} else {
			for _, ve := range validationErrors {
				if ve.Location != nil && ve.Location.Line > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s:%d: error: %s\n", workflowPath, ve.Location.Line, ve.Message)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s: error: %s\n", workflowPath, ve.Message)
				}
				if ve.Suggestion != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "  Suggestion: %s\n", ve.Suggestion)
				}
			}
		}
		return &shared.ExitError{Code: 1, Message: "validation failed"}
	}

	// Success - print validation results
	if useJSON {
		// Extract workflow metadata
		inputs := []string{}
		outputs := []string{}
		if def != nil {
			for _, input := range def.Inputs {
				inputs = append(inputs, input.Name)
			}
			for _, output := range def.Outputs {
				outputs = append(outputs, output.Name)
			}
		}

		// Extract security warnings for JSON output
		var securityWarnings []map[string]string
		if securityResult != nil && len(securityResult.Warnings) > 0 {
			securityWarnings = make([]map[string]string, 0, len(securityResult.Warnings))
			for _, w := range securityResult.Warnings {
				securityWarnings = append(securityWarnings, map[string]string{
					"step_id":    w.StepID,
					"type":       w.Type,
					"message":    w.Message,
					"suggestion": w.Suggestion,
					"severity":   w.Severity,
				})
			}
		}

		type workflowMetadata struct {
			Name             string              `json:"name"`
			Steps            int                 `json:"steps"`
			ModelTiers       []string            `json:"model_tiers"`
			Inputs           []string            `json:"inputs"`
			Outputs          []string            `json:"outputs"`
			Connectors       []string            `json:"connectors,omitempty"`
			SecurityWarnings []map[string]string `json:"security_warnings,omitempty"`
		}

		type profileInfo struct {
			Workspace string `json:"workspace,omitempty"`
			Profile   string `json:"profile,omitempty"`
		}

		type permissionInfo struct {
			Provider       string   `json:"provider"`
			AllEnforceable bool     `json:"all_enforceable"`
			Warnings       []string `json:"warnings,omitempty"`
		}

		type validateResponse struct {
			output.JSONResponse
			Workflow    workflowMetadata `json:"workflow"`
			Profile     *profileInfo     `json:"profile,omitempty"`
			Permissions *permissionInfo  `json:"permissions,omitempty"`
		}

		var profileData *profileInfo
		if workspace != "" || profile != "" {
			profileData = &profileInfo{
				Workspace: workspace,
				Profile:   profile,
			}
		}

		var permissionData *permissionInfo
		if permissionResult != nil {
			permissionData = &permissionInfo{
				Provider:       permissionResult.Provider,
				AllEnforceable: permissionResult.AllEnforceable,
				Warnings:       permissionResult.Warnings,
			}
		}

		resp := validateResponse{
			JSONResponse: output.JSONResponse{
				Version: "1.0",
				Command: "validate",
				Success: true,
			},
			Workflow: workflowMetadata{
				Name:             def.Name,
				Steps:            len(def.Steps),
				ModelTiers:       extractModelTiers(def),
				Inputs:           inputs,
				Outputs:          outputs,
				Connectors:       extractConnectorNames(def),
				SecurityWarnings: securityWarnings,
			},
			Profile:     profileData,
			Permissions: permissionData,
		}

		return output.EmitJSON(resp)
	} else {
		cmd.Println("Validation Results:")
		cmd.Println("  [OK] Syntax valid")
		cmd.Println("  [OK] Schema valid")
		cmd.Println("  [OK] All step references resolve correctly")

		// Show profile information if specified
		if workspace != "" || profile != "" {
			cmd.Println("\nProfile Configuration:")
			if workspace != "" {
				cmd.Printf("  Workspace: %s\n", workspace)
			}
			if profile != "" {
				cmd.Printf("  Profile: %s\n", profile)
			}
			cmd.Println("\n  Note: Profile binding validation requires daemon connection")
			cmd.Println("  Run with --daemon to validate actual bindings")
		}

		// Show security warnings
		if securityResult != nil && len(securityResult.Warnings) > 0 {
			cmd.Println("\nSecurity Warnings:")
			for _, warning := range securityResult.Warnings {
				cmd.Printf("  ⚠ [%s] %s\n", warning.StepID, warning.Message)
				if warning.Suggestion != "" {
					// Indent suggestion for readability
					fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", warning.Suggestion)
				}
			}
			cmd.Println("\nNote: Security warnings are non-blocking but should be reviewed.")
		}

		// Show permission enforcement warnings (SPEC-141)
		if permissionResult != nil {
			cmd.Println("\nPermission Enforcement:")
			cmd.Printf("  Provider: %s\n", permissionResult.Provider)
			if permissionResult.AllEnforceable {
				cmd.Println("  Status: ✓ All permissions can be enforced")
			} else {
				cmd.Println("  Status: ⚠ Some permissions cannot be enforced")
				cmd.Println("\n  Warnings:")
				for _, warning := range permissionResult.Warnings {
					cmd.Printf("    • %s\n", warning)
				}
				cmd.Println("\n  Note: Unenforceable permissions will be logged but not blocked.")
				cmd.Println("  Use --accept-unenforceable-permissions with 'conductor run' to proceed.")
			}
		}

		// Show model tiers used
		modelTiers := extractModelTiers(def)
		if len(modelTiers) > 0 {
			cmd.Printf("\nModel tiers used: %v\n", modelTiers)
			cmd.Println("Note: Run with configured provider to validate model tier mappings")
		}

		// Show connectors used
		connectors := extractConnectorNames(def)
		if len(connectors) > 0 {
			cmd.Printf("\nConnectors defined: %v\n", connectors)
		}
	}

	return nil
}

// extractModelTiers extracts the unique model tiers used in the workflow
func extractModelTiers(def *workflow.Definition) []string {
	if def == nil {
		return nil
	}

	tiers := make(map[string]bool)

	for _, step := range def.Steps {
		if step.Type == workflow.StepTypeLLM && step.Inputs != nil {
			if model, ok := step.Inputs["model"].(string); ok {
				tiers[model] = true
			}
		}
	}

	result := make([]string, 0, len(tiers))
	for tier := range tiers {
		result = append(result, tier)
	}
	return result
}

// extractConnectorNames extracts the names of connectors defined in the workflow
func extractConnectorNames(def *workflow.Definition) []string {
	if def == nil || len(def.Connectors) == 0 {
		return nil
	}

	names := make([]string, 0, len(def.Connectors))
	for name := range def.Connectors {
		names = append(names, name)
	}
	return names
}

// extractYAMLErrorLocation attempts to extract line and column from YAML parse error
func extractYAMLErrorLocation(err error) (line, col int) {
	// yaml.v3 includes line numbers in error messages
	// Try to extract them if possible
	if typeErr, ok := err.(*yaml.TypeError); ok {
		// TypeError contains errors with line information
		if len(typeErr.Errors) > 0 {
			// Parse first error message which may contain line info
			// Format is typically "line X: message"
			var l int
			if _, parseErr := fmt.Sscanf(typeErr.Errors[0], "line %d:", &l); parseErr == nil {
				return l, 0
			}
		}
	}
	return 0, 0
}

// validateAgainstSchema validates data against the workflow JSON Schema
func validateAgainstSchema(data interface{}, schemaPath string) []output.JSONError {
	var errors []output.JSONError

	// Load schema (either from path or embedded)
	var schemaData map[string]interface{}
	if schemaPath != "" {
		// Load from custom path
		schemaBytes, err := os.ReadFile(schemaPath)
		if err != nil {
			errors = append(errors, output.JSONError{
				Code:       shared.ErrorCodeFileNotFound,
				Message:    fmt.Sprintf("failed to read schema file: %v", err),
				Suggestion: "Check that the schema file path is correct",
			})
			return errors
		}
		if err := json.Unmarshal(schemaBytes, &schemaData); err != nil {
			errors = append(errors, output.JSONError{
				Code:       shared.ErrorCodeSchemaViolation,
				Message:    fmt.Sprintf("failed to parse schema file: %v", err),
				Suggestion: "Ensure the schema file is valid JSON",
			})
			return errors
		}
	} else {
		// Use embedded schema
		schemaBytes := workflowschema.GetEmbeddedSchema()
		if err := json.Unmarshal(schemaBytes, &schemaData); err != nil {
			errors = append(errors, output.JSONError{
				Code:       shared.ErrorCodeSchemaViolation,
				Message:    fmt.Sprintf("failed to parse embedded schema: %v", err),
				Suggestion: "This is an internal error; please report it",
			})
			return errors
		}
	}

	// Validate against schema using the built-in validator
	validator := workflowschema.NewValidator()
	if err := validator.Validate(schemaData, data); err != nil {
		// Check if it's a ValidationError with path information
		if valErr, ok := err.(*workflowschema.ValidationError); ok {
			errors = append(errors, output.JSONError{
				Code:       shared.ErrorCodeSchemaViolation,
				Message:    valErr.Message,
				Suggestion: "Review the workflow schema constraints",
			})
		} else {
			errors = append(errors, output.JSONError{
				Code:       shared.ErrorCodeSchemaViolation,
				Message:    err.Error(),
				Suggestion: "Ensure the workflow conforms to the schema",
			})
		}
	}

	return errors
}
