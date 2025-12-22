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

package shared

// Error codes for structured JSON output
const (
	// Validation errors (E001-E099)
	ErrorCodeMissingField       = "E001" // Missing required field
	ErrorCodeInvalidYAML        = "E002" // Invalid YAML syntax
	ErrorCodeSchemaViolation    = "E003" // Schema constraint violation
	ErrorCodeInvalidReference   = "E004" // Invalid reference (unknown step ID)

	// Execution errors (E100-E199)
	ErrorCodeProviderNotFound   = "E101" // Provider not found
	ErrorCodeProviderTimeout    = "E102" // Provider timeout
	ErrorCodeStepFailed         = "E103" // Step execution failed
	ErrorCodeWorkflowTimeout    = "E104" // Workflow timeout

	// Configuration errors (E200-E299)
	ErrorCodeConfigNotFound     = "E201" // Config file not found
	ErrorCodeInvalidConfig      = "E202" // Invalid provider configuration
	ErrorCodeMissingAPIKey      = "E203" // Missing API key

	// Input errors (E300-E399)
	ErrorCodeMissingInput       = "E301" // Required input missing
	ErrorCodeInvalidInput       = "E302" // Invalid input format
	ErrorCodeFileNotFound       = "E303" // File not found

	// Resource errors (E400-E499)
	ErrorCodeNotFound           = "E401" // Resource not found
	ErrorCodeInternal           = "E402" // Internal error
	ErrorCodeExecutionFailed    = "E403" // Execution failed
)

// mapExitErrorToCode maps ExitError codes to JSON error codes
func mapExitErrorToCode(exitErr *ExitError) string {
	if exitErr == nil {
		return ""
	}

	switch exitErr.Code {
	case ExitInvalidWorkflow:
		return ErrorCodeSchemaViolation
	case ExitMissingInput:
		return ErrorCodeMissingInput
	case ExitProviderError:
		return ErrorCodeProviderNotFound
	case ExitExecutionFailed:
		return ErrorCodeStepFailed
	default:
		return ErrorCodeStepFailed
	}
}
