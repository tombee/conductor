package transform

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// XMLParseOptions configures XML parsing behavior.
type XMLParseOptions struct {
	// AttributePrefix is the prefix for attribute keys (default: "@")
	AttributePrefix string

	// StripNamespaces removes namespace prefixes from element names
	StripNamespaces bool
}

// DefaultXMLParseOptions returns sensible defaults for XML parsing.
func DefaultXMLParseOptions() *XMLParseOptions {
	return &XMLParseOptions{
		AttributePrefix: "@",
		StripNamespaces: true,
	}
}

// XXE prevention patterns - case insensitive
var (
	doctypePattern = regexp.MustCompile(`(?i)<!DOCTYPE`)
	entityPattern  = regexp.MustCompile(`(?i)<!ENTITY`)
	systemPattern  = regexp.MustCompile(`(?i)\bSYSTEM\b`)
	publicPattern  = regexp.MustCompile(`(?i)\bPUBLIC\b`)
)

// scanForXXE performs pre-parse security scan for dangerous XML constructs.
// Returns error if dangerous patterns are found.
func scanForXXE(data []byte) error {
	if doctypePattern.Match(data) {
		return fmt.Errorf("XXE prevention: DOCTYPE declarations are not allowed in XML input")
	}
	if entityPattern.Match(data) {
		return fmt.Errorf("XXE prevention: ENTITY declarations are not allowed in XML input")
	}
	// Check for SYSTEM/PUBLIC only in entity context (rough heuristic)
	if bytes.Contains(data, []byte("!ENTITY")) || bytes.Contains(data, []byte("!entity")) {
		if systemPattern.Match(data) || publicPattern.Match(data) {
			return fmt.Errorf("XXE prevention: external entity references (SYSTEM/PUBLIC) are not allowed")
		}
	}
	return nil
}

// parseXMLToMap converts XML bytes to a map structure.
// Attributes are prefixed with @ by default.
// Namespaces are stripped from element names if StripNamespaces is true.
func parseXMLToMap(data []byte, options *XMLParseOptions) (interface{}, error) {
	if options == nil {
		options = DefaultXMLParseOptions()
	}

	// Pre-scan for XXE vulnerabilities
	if err := scanForXXE(data); err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true

	// Build element tree
	root, err := parseElement(decoder, options)
	if err != nil {
		return nil, err
	}

	return root, nil
}

// parseElement recursively parses an XML element and its children.
func parseElement(decoder *xml.Decoder, options *XMLParseOptions) (interface{}, error) {
	var stack []map[string]interface{}
	var current map[string]interface{}
	var text strings.Builder

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("XML parse error: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			// Create new element map
			elem := make(map[string]interface{})

			// Add attributes with prefix
			for _, attr := range t.Attr {
				attrName := attr.Name.Local
				if attr.Name.Space != "" && !options.StripNamespaces {
					attrName = attr.Name.Space + ":" + attrName
				}
				elem[options.AttributePrefix+attrName] = attr.Value
			}

			// Push current element onto stack
			if current != nil {
				stack = append(stack, current)
			}

			// Get element name (strip namespace if requested)
			elemName := t.Name.Local
			if !options.StripNamespaces && t.Name.Space != "" {
				elemName = t.Name.Space + ":" + elemName
			}

			current = map[string]interface{}{
				"__name__": elemName,
				"__data__": elem,
			}
			text.Reset()

		case xml.CharData:
			text.Write(t)

		case xml.EndElement:
			// Add text content if any
			textContent := strings.TrimSpace(text.String())
			if textContent != "" {
				current["__data__"].(map[string]interface{})["#text"] = textContent
			} else if len(current["__data__"].(map[string]interface{})) == 0 {
				// Empty element with no attributes
				current["__data__"].(map[string]interface{})["#text"] = ""
			}

			elemName := current["__name__"].(string)
			elemData := current["__data__"].(map[string]interface{})

			// Simplify if only text content
			var value interface{}
			if len(elemData) == 1 {
				if txt, ok := elemData["#text"]; ok {
					value = txt
				} else {
					value = elemData
				}
			} else {
				value = elemData
			}

			// Pop from stack and add to parent
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				stack = stack[:len(stack)-1]

				// Handle multiple elements with same name
				parentData := parent["__data__"].(map[string]interface{})
				if existing, ok := parentData[elemName]; ok {
					// Convert to array
					if arr, isArr := existing.([]interface{}); isArr {
						parentData[elemName] = append(arr, value)
					} else {
						parentData[elemName] = []interface{}{existing, value}
					}
				} else {
					parentData[elemName] = value
				}

				current = parent
				text.Reset()
			} else {
				// Root element - return just the data
				return map[string]interface{}{elemName: value}, nil
			}

		case xml.Comment:
			// Ignore comments

		case xml.ProcInst:
			// Ignore processing instructions

		case xml.Directive:
			// Directives should have been caught by XXE scan, but reject anyway
			return nil, fmt.Errorf("XXE prevention: XML directives are not allowed")
		}
	}

	if current != nil {
		elemName := current["__name__"].(string)
		elemData := current["__data__"].(map[string]interface{})
		return map[string]interface{}{elemName: elemData}, nil
	}

	return nil, fmt.Errorf("XML parse error: no root element found")
}
