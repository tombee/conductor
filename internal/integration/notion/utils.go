package notion

import "strings"

// utils.go contains shared helper functions for the Notion integration.
// These utilities handle ID validation, normalization, and property extraction
// used across pages, blocks, and database operations.

// normalizeNotionID removes hyphens from a Notion ID.
// Notion IDs can be formatted with or without hyphens (UUID format).
func normalizeNotionID(id string) string {
	return strings.ReplaceAll(id, "-", "")
}

// isValidNotionID validates a Notion ID format.
// Valid IDs are 32 alphanumeric characters (after removing hyphens).
func isValidNotionID(id string) bool {
	// Remove hyphens if present (Notion IDs can be formatted with or without hyphens)
	id = normalizeNotionID(id)

	if len(id) != 32 {
		return false
	}

	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}

	return true
}

// extractPageTitle extracts the title from page properties.
// Returns empty string if the title property is not found or malformed.
func extractPageTitle(properties map[string]interface{}) string {
	titleProp, ok := properties["title"]
	if !ok {
		return ""
	}

	titleMap, ok := titleProp.(map[string]interface{})
	if !ok {
		return ""
	}

	titleArray, ok := titleMap["title"].([]interface{})
	if !ok || len(titleArray) == 0 {
		return ""
	}

	firstTitle, ok := titleArray[0].(map[string]interface{})
	if !ok {
		return ""
	}

	textMap, ok := firstTitle["text"].(map[string]interface{})
	if !ok {
		return ""
	}

	content, _ := textMap["content"].(string)
	return content
}
