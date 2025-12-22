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

package workflow

import (
	"fmt"
	"regexp"
	"strings"
)

// SecurityWarning represents a security concern found during validation
type SecurityWarning struct {
	// StepID is the step identifier where the issue was found
	StepID string

	// Type categorizes the warning (e.g., "shell_injection", "plaintext_credential")
	Type string

	// Message describes the security concern
	Message string

	// Suggestion provides guidance on how to fix the issue
	Suggestion string

	// Severity indicates the importance (warning, error)
	Severity string
}

// SecurityValidationResult contains all security findings from validation
type SecurityValidationResult struct {
	// Warnings contains non-blocking security concerns
	Warnings []SecurityWarning

	// Errors contains blocking security issues
	Errors []SecurityWarning
}

// ValidateSecurity performs comprehensive security validation on a workflow definition
func ValidateSecurity(def *Definition) *SecurityValidationResult {
	result := &SecurityValidationResult{
		Warnings: []SecurityWarning{},
		Errors:   []SecurityWarning{},
	}

	// Check for shell injection risks in steps
	checkShellInjectionRisk(def, result)

	// Check for plaintext credentials in connector auth
	detectPlaintextCredentials(def, result)

	// Check for overly permissive file paths
	checkOverlyPermissivePaths(def, result)

	// Check for missing auth on external connectors
	checkMissingAuth(def, result)

	return result
}

// checkShellInjectionRisk detects shell.run steps using string form with template variables
// Per DECISION-67-1, this emits warnings (not blocking errors) and recommends array form
func checkShellInjectionRisk(def *Definition, result *SecurityValidationResult) {
	templateVarPattern := regexp.MustCompile(`\{\{[^}]+\}\}`)

	for i := range def.Steps {
		step := &def.Steps[i]
		checkStepShellInjection(step, result, templateVarPattern)
	}
}

func checkStepShellInjection(step *StepDefinition, result *SecurityValidationResult, pattern *regexp.Regexp) {
	// Check if this is a shell.run step (builtin connector)
	if step.Type == StepTypeBuiltin && step.BuiltinConnector == "shell" && step.BuiltinOperation == "run" {
		// Check if inputs contain a command as a string with template variables
		if step.Inputs != nil {
			if cmdVal, exists := step.Inputs["command"]; exists {
				// If command is a string (not array) and contains template variables
				if cmdStr, isString := cmdVal.(string); isString {
					if pattern.MatchString(cmdStr) {
						result.Warnings = append(result.Warnings, SecurityWarning{
							StepID:   step.ID,
							Type:     "shell_injection",
							Message:  "shell.run with string command contains template variables",
							Severity: "warning",
							Suggestion: fmt.Sprintf(
								"This may be vulnerable to command injection. Consider using array form:\n"+
									"  Before: shell.run: %q\n"+
									"  After:  shell.run:\n"+
									"            command: [\"cmd\", \"arg1\", \"{{.var}}\"]",
								cmdStr,
							),
						})
					}
				}
			}
		}
	}

	// Recursively check nested steps (for parallel, condition)
	for i := range step.Steps {
		nestedStep := &step.Steps[i]
		checkStepShellInjection(nestedStep, result, pattern)
	}
}

// detectPlaintextCredentials checks auth fields for plaintext credential patterns
// Produces validation ERROR for plaintext credentials per SR2
func detectPlaintextCredentials(def *Definition, result *SecurityValidationResult) {
	credentialPatterns := []struct {
		pattern     *regexp.Regexp
		description string
	}{
		{regexp.MustCompile(`^ghp_[a-zA-Z0-9]{36,}$`), "GitHub personal access token"},
		{regexp.MustCompile(`^gho_[a-zA-Z0-9]{36,}$`), "GitHub OAuth token"},
		{regexp.MustCompile(`^ghs_[a-zA-Z0-9]{36,}$`), "GitHub server-to-server token"},
		{regexp.MustCompile(`^github_pat_[a-zA-Z0-9_]{82}$`), "GitHub fine-grained PAT"},
		{regexp.MustCompile(`^sk-[a-zA-Z0-9]{20,}$`), "OpenAI API key"},
		{regexp.MustCompile(`^sk-ant-[a-zA-Z0-9-]{95,}$`), "Anthropic API key"},
		{regexp.MustCompile(`^xoxb-[0-9]{10,}-[0-9]{10,}-[a-zA-Z0-9]{24,}$`), "Slack bot token"},
		{regexp.MustCompile(`^xoxp-[0-9]{10,}-[0-9]{10,}-[a-zA-Z0-9]{24,}$`), "Slack user token"},
		{regexp.MustCompile(`^gsk_[a-zA-Z0-9]{32,}$`), "Groq API key"},
		{regexp.MustCompile(`^xai-[a-zA-Z0-9]{40,}$`), "xAI API key"},
	}

	// Check connector auth fields
	for connectorName, connector := range def.Connectors {
		if connector.Auth != nil {
			auth := connector.Auth

			// Check all auth fields that might contain credentials
			fieldsToCheck := map[string]string{
				"token":         auth.Token,
				"password":      auth.Password,
				"value":         auth.Value,
				"client_secret": auth.ClientSecret,
			}

			for fieldName, fieldValue := range fieldsToCheck {
				if fieldValue == "" {
					continue
				}

				// Skip environment variable references
				if isSecretReference(fieldValue) {
					continue
				}

				// Check against known credential patterns
				for _, cp := range credentialPatterns {
					if cp.pattern.MatchString(fieldValue) {
						result.Errors = append(result.Errors, SecurityWarning{
							StepID:   fmt.Sprintf("connectors.%s", connectorName),
							Type:     "plaintext_credential",
							Severity: "error",
							Message:  fmt.Sprintf("Plaintext %s detected in auth.%s", cp.description, fieldName),
							Suggestion: fmt.Sprintf(
								"Use environment variables or secrets instead:\n"+
									"  connectors:\n"+
									"    %s:\n"+
									"      auth:\n"+
									"        %s: ${%s}  # Environment variable\n"+
									"        # OR\n"+
									"        %s: $secret:%s  # Secrets backend",
								connectorName, fieldName, strings.ToUpper(connectorName)+"_"+strings.ToUpper(fieldName),
								fieldName, strings.ToLower(connectorName)+"_"+fieldName,
							),
						})
						break // Only report first matching pattern per field
					}
				}

				// Even if no specific pattern matched, warn about non-secret values in sensitive fields
				if !isSecretReference(fieldValue) && (fieldName == "token" || fieldName == "password" || fieldName == "client_secret") {
					// Only add generic warning if we haven't already added a specific pattern match
					alreadyReported := false
					for _, warning := range result.Errors {
						if warning.StepID == fmt.Sprintf("connectors.%s", connectorName) && warning.Type == "plaintext_credential" {
							alreadyReported = true
							break
						}
					}

					if !alreadyReported {
						result.Errors = append(result.Errors, SecurityWarning{
							StepID:   fmt.Sprintf("connectors.%s", connectorName),
							Type:     "plaintext_credential",
							Severity: "error",
							Message:  fmt.Sprintf("Potential plaintext credential in auth.%s", fieldName),
							Suggestion: fmt.Sprintf(
								"Use environment variables or secrets:\n"+
									"  auth:\n"+
									"    %s: ${%s_TOKEN}",
								fieldName, strings.ToUpper(connectorName),
							),
						})
					}
				}
			}
		}
	}
}

// isSecretReference checks if a value references an environment variable or secret
func isSecretReference(value string) bool {
	// Environment variable: ${VAR_NAME}
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return true
	}

	// Secrets backend: $secret:name
	if strings.HasPrefix(value, "$secret:") {
		return true
	}

	return false
}

// checkOverlyPermissivePaths warns about broad file paths
func checkOverlyPermissivePaths(def *Definition, result *SecurityValidationResult) {
	for i := range def.Steps {
		step := &def.Steps[i]
		checkStepPermissivePaths(step, result)
	}
}

func checkStepPermissivePaths(step *StepDefinition, result *SecurityValidationResult) {
	// Check if this is a file connector step
	if step.Type == StepTypeBuiltin && step.BuiltinConnector == "file" {
		if step.Inputs != nil {
			// Check path field
			if pathVal, exists := step.Inputs["path"]; exists {
				if pathStr, ok := pathVal.(string); ok {
					// Warn on overly broad paths
					if isOverlyPermissivePath(pathStr) {
						result.Warnings = append(result.Warnings, SecurityWarning{
							StepID:     step.ID,
							Type:       "overly_permissive_path",
							Severity:   "warning",
							Message:    fmt.Sprintf("File path is too broad: %q", pathStr),
							Suggestion: "Specify explicit paths instead of root or home directory.\n  Example: ~/projects/myapp instead of ~",
						})
					}
				}
			}
		}
	}

	// Recursively check nested steps
	for i := range step.Steps {
		nestedStep := &step.Steps[i]
		checkStepPermissivePaths(nestedStep, result)
	}
}

// isOverlyPermissivePath checks if a path is too broad
func isOverlyPermissivePath(path string) bool {
	// Trim whitespace
	path = strings.TrimSpace(path)

	// Check for exact matches of overly broad paths
	broadPaths := []string{
		"/",        // Root directory
		"~",        // Home directory without subdirectory
		"~/",       // Home directory without subdirectory
		"$out",     // Output directory without subdirectory
		"$out/",    // Output directory without subdirectory
		"$temp",    // Temp directory without subdirectory
		"$temp/",   // Temp directory without subdirectory
	}

	for _, broad := range broadPaths {
		if path == broad {
			return true
		}
	}

	return false
}

// checkMissingAuth detects external connectors without auth configuration
func checkMissingAuth(def *Definition, result *SecurityValidationResult) {
	builtinConnectors := map[string]bool{
		"file":      true,
		"shell":     true,
		"http":      true,
		"transform": true,
	}

	for connectorName, connector := range def.Connectors {
		// Skip builtin connectors
		if builtinConnectors[connectorName] {
			continue
		}

		// External connectors should have auth configured
		if connector.Auth == nil {
			result.Warnings = append(result.Warnings, SecurityWarning{
				StepID:   fmt.Sprintf("connectors.%s", connectorName),
				Type:     "missing_auth",
				Severity: "warning",
				Message:  fmt.Sprintf("External connector %q has no auth configuration", connectorName),
				Suggestion: fmt.Sprintf(
					"Consider adding authentication:\n"+
						"  connectors:\n"+
						"    %s:\n"+
						"      auth:\n"+
						"        token: ${%s_TOKEN}",
					connectorName, strings.ToUpper(connectorName),
				),
			})
		}
	}
}
