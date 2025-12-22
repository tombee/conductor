package schema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// BuildPromptWithSchema appends JSON schema instructions to a prompt.
// This implements T3.1: prompt builder that adds schema requirements.
func BuildPromptWithSchema(originalPrompt string, schema map[string]interface{}, retryAttempt int) string {
	// Build schema description
	schemaDesc := formatSchemaForPrompt(schema)

	var instruction string
	switch retryAttempt {
	case 0:
		// First attempt: subtle hint
		instruction = fmt.Sprintf("\n\nPlease respond with valid JSON matching this structure:\n%s", schemaDesc)
	case 1:
		// Second attempt: more explicit
		instruction = fmt.Sprintf("\n\nIMPORTANT: Your previous response didn't match the required format. Please respond with valid JSON matching this schema:\n%s\n\nRespond ONLY with the JSON object, no additional text.", schemaDesc)
	default:
		// Third attempt: very explicit with example
		exampleJSON := buildExampleJSON(schema)
		instruction = fmt.Sprintf("\n\nCRITICAL: You must respond with ONLY valid JSON. No explanations, no markdown, just the JSON object.\n\nRequired format:\n%s\n\nExample:\n%s", schemaDesc, exampleJSON)
	}

	return originalPrompt + instruction
}

// formatSchemaForPrompt creates a human-readable description of the schema.
func formatSchemaForPrompt(schema map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("{\n")

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		required := make(map[string]bool)
		if reqList, ok := schema["required"].([]interface{}); ok {
			for _, r := range reqList {
				if rStr, ok := r.(string); ok {
					required[rStr] = true
				}
			}
		}

		first := true
		for name, propSchema := range props {
			if !first {
				sb.WriteString(",\n")
			}
			first = false

			propMap, _ := propSchema.(map[string]interface{})
			propType, _ := propMap["type"].(string)

			requiredMarker := ""
			if required[name] {
				requiredMarker = " (required)"
			}

			// Add enum info if present
			enumInfo := ""
			if enum, ok := propMap["enum"].([]interface{}); ok {
				enumValues := make([]string, len(enum))
				for i, v := range enum {
					enumValues[i] = fmt.Sprintf("%q", v)
				}
				enumInfo = fmt.Sprintf(" // one of: %s", strings.Join(enumValues, ", "))
			}

			sb.WriteString(fmt.Sprintf("  %q: %s%s%s", name, propType, requiredMarker, enumInfo))
		}
	}

	sb.WriteString("\n}")
	return sb.String()
}

// buildExampleJSON creates an example JSON object from the schema.
func buildExampleJSON(schema map[string]interface{}) string {
	example := buildExampleValue(schema)
	jsonBytes, _ := json.MarshalIndent(example, "", "  ")
	return string(jsonBytes)
}

// buildExampleValue recursively builds example values from schema.
func buildExampleValue(schema map[string]interface{}) interface{} {
	schemaType, _ := schema["type"].(string)

	switch schemaType {
	case "object":
		obj := make(map[string]interface{})
		if props, ok := schema["properties"].(map[string]interface{}); ok {
			for name, propSchema := range props {
				propMap, _ := propSchema.(map[string]interface{})
				obj[name] = buildExampleValue(propMap)
			}
		}
		return obj

	case "array":
		if items, ok := schema["items"].(map[string]interface{}); ok {
			// Return array with one example item
			return []interface{}{buildExampleValue(items)}
		}
		return []interface{}{}

	case "string":
		// Check for enum
		if enum, ok := schema["enum"].([]interface{}); ok && len(enum) > 0 {
			if str, ok := enum[0].(string); ok {
				return str
			}
		}
		return "example"

	case "number":
		return 42

	case "integer":
		return 1

	case "boolean":
		return true

	default:
		return nil
	}
}

// ExtractJSON attempts to extract JSON from an LLM response that may contain extra text.
// This implements T3.2: JSON extraction from various response formats.
func ExtractJSON(response string) (interface{}, error) {
	// Trim whitespace
	response = strings.TrimSpace(response)

	// Try parsing directly first
	var data interface{}
	if err := json.Unmarshal([]byte(response), &data); err == nil {
		return data, nil
	}

	// Try to extract JSON from markdown code blocks
	if extracted := extractFromCodeBlock(response); extracted != "" {
		if err := json.Unmarshal([]byte(extracted), &data); err == nil {
			return data, nil
		}
	}

	// Try to find JSON object or array in the text
	if extracted := extractJSONFromText(response); extracted != "" {
		if err := json.Unmarshal([]byte(extracted), &data); err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("could not extract valid JSON from response")
}

// extractFromCodeBlock extracts content from markdown code blocks.
func extractFromCodeBlock(text string) string {
	// Match ```json ... ``` or ``` ... ```
	patterns := []string{
		"(?s)```json\\s*\\n(.+?)```",
		"(?s)```\\s*\\n(.+?)```",
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// extractJSONFromText tries to find a JSON object or array in arbitrary text.
func extractJSONFromText(text string) string {
	// Look for { ... } or [ ... ]
	var depth int
	var start int
	var inString bool
	var escape bool
	var foundStart bool

	for i, ch := range text {
		if escape {
			escape = false
			continue
		}

		switch ch {
		case '\\':
			if inString {
				escape = true
			}
		case '"':
			inString = !inString
		case '{', '[':
			if !inString {
				if depth == 0 {
					start = i
					foundStart = true
				}
				depth++
			}
		case '}', ']':
			if !inString {
				depth--
				if depth == 0 && foundStart {
					return text[start : i+1]
				}
			}
		}
	}

	return ""
}
