package file

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// stripBOM removes UTF-8/UTF-16 BOM from the beginning of content.
func stripBOM(content []byte) []byte {
	// UTF-8 BOM: EF BB BF
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		return content[3:]
	}

	// UTF-16 BE BOM: FE FF
	if len(content) >= 2 && content[0] == 0xFE && content[1] == 0xFF {
		return content[2:]
	}

	// UTF-16 LE BOM: FF FE
	if len(content) >= 2 && content[0] == 0xFF && content[1] == 0xFE {
		return content[2:]
	}

	return content
}

// parseJSON parses JSON content and returns the result.
func parseJSON(content []byte) (interface{}, error) {
	var result interface{}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber() // Preserve number precision

	if err := decoder.Decode(&result); err != nil {
		return nil, enhanceJSONError(err, content)
	}

	return result, nil
}

// enhanceJSONError adds line/column information to JSON parse errors.
func enhanceJSONError(err error, content []byte) error {
	if syntaxErr, ok := err.(*json.SyntaxError); ok {
		line, col := findLineCol(content, syntaxErr.Offset)
		return fmt.Errorf("syntax error at line %d, column %d: %v", line, col, syntaxErr)
	}
	return err
}

// findLineCol calculates line and column number from byte offset.
func findLineCol(content []byte, offset int64) (line, col int) {
	line = 1
	col = 1

	for i := int64(0); i < offset && i < int64(len(content)); i++ {
		if content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	return line, col
}

// parseYAML parses YAML content and returns the result.
func parseYAML(content []byte) (interface{}, error) {
	var result interface{}

	decoder := yaml.NewDecoder(bytes.NewReader(content))

	// Try to decode the first document
	if err := decoder.Decode(&result); err != nil {
		if err == io.EOF {
			// Empty YAML file
			return nil, nil
		}
		return nil, err
	}

	// Check if there are more documents
	var secondDoc interface{}
	if err := decoder.Decode(&secondDoc); err == nil {
		// Multiple documents - return array of documents
		docs := []interface{}{result, secondDoc}

		// Decode remaining documents
		for {
			var doc interface{}
			if err := decoder.Decode(&doc); err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			docs = append(docs, doc)
		}

		return docs, nil
	}

	// Single document
	return result, nil
}

// parseCSV parses CSV content and returns an array of objects.
func parseCSV(content []byte, delimiter string) ([]map[string]string, error) {
	if delimiter == "" {
		delimiter = ","
	}

	if len(delimiter) != 1 {
		return nil, fmt.Errorf("delimiter must be a single character")
	}

	reader := csv.NewReader(bytes.NewReader(content))
	reader.Comma = rune(delimiter[0])
	reader.TrimLeadingSpace = true

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV parsing error: %w", err)
	}

	if len(records) == 0 {
		// Empty CSV
		return []map[string]string{}, nil
	}

	// First row is headers
	headers := records[0]
	if len(headers) == 0 {
		return nil, fmt.Errorf("CSV has no columns")
	}

	// Handle duplicate headers by appending _2, _3, etc.
	uniqueHeaders := make([]string, len(headers))
	headerCounts := make(map[string]int)

	for i, header := range headers {
		count := headerCounts[header]
		headerCounts[header]++

		if count == 0 {
			uniqueHeaders[i] = header
		} else {
			uniqueHeaders[i] = fmt.Sprintf("%s_%d", header, count+1)
		}
	}

	// Convert remaining rows to objects
	result := make([]map[string]string, 0, len(records)-1)

	for rowIdx := 1; rowIdx < len(records); rowIdx++ {
		row := records[rowIdx]
		obj := make(map[string]string)

		for colIdx, header := range uniqueHeaders {
			if colIdx < len(row) {
				obj[header] = row[colIdx]
			} else {
				// Missing column - set to empty string
				obj[header] = ""
			}
		}

		result = append(result, obj)
	}

	return result, nil
}

// extractJSONPath extracts a value from parsed JSON using a simple JSONPath expression.
// This is a basic implementation supporting simple paths like $.field.nested
func extractJSONPath(data interface{}, path string) (interface{}, error) {
	// Remove leading $. if present
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")

	if path == "" || path == "." {
		return data, nil
	}

	// Split path by dots
	parts := strings.Split(path, ".")

	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}

		// Handle object access
		if obj, ok := current.(map[string]interface{}); ok {
			val, exists := obj[part]
			if !exists {
				return nil, fmt.Errorf("path not found: %s", part)
			}
			current = val
			continue
		}

		// Handle array index (simple numeric index like [0])
		// This is a simplified version - full JSONPath would be more complex
		return nil, fmt.Errorf("unsupported JSONPath operation at: %s", part)
	}

	return current, nil
}

// formatJSON formats content as pretty-printed JSON.
func formatJSON(content interface{}) ([]byte, error) {
	// Pretty-print with 2-space indentation
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("JSON marshal error: %w", err)
	}
	// Add trailing newline
	return append(data, '\n'), nil
}

// formatYAML formats content as YAML.
func formatYAML(content interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(content); err != nil {
		return nil, fmt.Errorf("YAML marshal error: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("YAML encoder close error: %w", err)
	}

	return buf.Bytes(), nil
}

// renderTemplate renders a Go template with restricted functions.
func renderTemplate(tmplStr string, data interface{}) (string, error) {
	// Create template with restricted function map
	tmpl, err := template.New("template").Funcs(restrictedFuncMap()).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

// restrictedFuncMap returns a whitelist of safe template functions.
func restrictedFuncMap() template.FuncMap {
	return template.FuncMap{
		// String functions
		"lower":   strings.ToLower,
		"upper":   strings.ToUpper,
		"trim":    strings.TrimSpace,
		"replace": strings.ReplaceAll,
		"split":   strings.Split,
		"join":    strings.Join,

		// Default function (returns first non-empty value)
		"default": func(def interface{}, val interface{}) interface{} {
			if val == nil || val == "" {
				return def
			}
			return val
		},
	}
}
