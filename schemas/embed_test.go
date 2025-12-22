package schemas

import (
	"encoding/json"
	"testing"
)

func TestGetWorkflowSchema(t *testing.T) {
	schema := GetWorkflowSchema()

	// Schema should not be empty
	if len(schema) == 0 {
		t.Fatal("embedded schema is empty")
	}

	// Schema should be valid JSON
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		t.Fatalf("embedded schema is not valid JSON: %v", err)
	}

	// Should contain required JSON Schema fields
	if _, ok := schemaMap["$schema"]; !ok {
		t.Error("schema missing $schema field")
	}

	if _, ok := schemaMap["$id"]; !ok {
		t.Error("schema missing $id field")
	}

	if title, ok := schemaMap["title"].(string); !ok || title == "" {
		t.Error("schema missing or empty title field")
	}
}

func TestGetWorkflowSchemaString(t *testing.T) {
	schemaStr := GetWorkflowSchemaString()

	// Should not be empty
	if schemaStr == "" {
		t.Fatal("embedded schema string is empty")
	}

	// Should be same content as bytes version
	schemaBytes := GetWorkflowSchema()
	if schemaStr != string(schemaBytes) {
		t.Error("string and bytes versions of schema do not match")
	}

	// Should be valid JSON
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schemaMap); err != nil {
		t.Fatalf("embedded schema string is not valid JSON: %v", err)
	}
}
