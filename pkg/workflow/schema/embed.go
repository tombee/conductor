package schema

import (
	"github.com/tombee/conductor/schemas"
)

// GetEmbeddedSchema returns the embedded workflow JSON Schema as raw bytes.
// This schema can be used for validation, IDE integration, or schema export.
//
// The schema is embedded via the schemas package at the module root level,
// since go:embed directives cannot reference parent directories.
func GetEmbeddedSchema() []byte {
	return schemas.GetWorkflowSchema()
}

// GetEmbeddedSchemaString returns the embedded workflow JSON Schema as a string.
// This is a convenience method for use cases that need the schema as a string.
func GetEmbeddedSchemaString() string {
	return schemas.GetWorkflowSchemaString()
}
