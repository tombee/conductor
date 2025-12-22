// Package schemas provides access to embedded JSON schemas.
package schemas

import (
	_ "embed"
)

// Embed the workflow JSON Schema into the binary for validation and tooling.
// The schema defines the structure of workflow definitions and enables
// IDE autocompletion, early validation, and schema-based tools.
//
//go:embed workflow.schema.json
var workflowSchema []byte

// GetWorkflowSchema returns the embedded workflow JSON Schema as raw bytes.
// This schema can be used for validation, IDE integration, or schema export.
func GetWorkflowSchema() []byte {
	return workflowSchema
}

// GetWorkflowSchemaString returns the embedded workflow JSON Schema as a string.
// This is a convenience method for use cases that need the schema as a string.
func GetWorkflowSchemaString() string {
	return string(workflowSchema)
}
