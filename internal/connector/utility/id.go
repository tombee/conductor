package utility

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
)

// Default alphabet for nanoid (URL-safe)
const nanoidAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"

// idUUID generates a UUID v4.
func (c *UtilityConnector) idUUID(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Generate 16 random bytes
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	if err != nil {
		return nil, &OperationError{
			Operation:  "id_uuid",
			Message:    "failed to generate random bytes",
			ErrorType:  ErrorTypeInternal,
			Cause:      err,
			Suggestion: "Check system entropy source",
		}
	}

	// Set version (4) and variant (RFC 4122)
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10

	// Format as string: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	uuidStr := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	)

	return &Result{
		Response: uuidStr,
		Metadata: map[string]interface{}{
			"operation": "id_uuid",
			"version":   4,
		},
	}, nil
}

// idNanoid generates a short URL-safe ID.
func (c *UtilityConnector) idNanoid(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get optional length parameter
	length := c.config.DefaultNanoidLength
	if _, ok := inputs["length"]; ok {
		ln, err := getInt(inputs, "length")
		if err != nil {
			return nil, &OperationError{
				Operation:  "id_nanoid",
				Message:    "invalid 'length' parameter",
				ErrorType:  ErrorTypeValidation,
				Cause:      err,
				Suggestion: "Provide 'length' as a positive integer",
			}
		}
		length = ln
	}

	if length <= 0 {
		return nil, &OperationError{
			Operation:  "id_nanoid",
			Message:    "length must be positive",
			ErrorType:  ErrorTypeRange,
			Suggestion: "Provide a length >= 1",
		}
	}

	if length > c.config.MaxIDLength {
		return nil, &OperationError{
			Operation:  "id_nanoid",
			Message:    fmt.Sprintf("length (%d) exceeds maximum (%d)", length, c.config.MaxIDLength),
			ErrorType:  ErrorTypeRange,
			Suggestion: fmt.Sprintf("Reduce length to at most %d", c.config.MaxIDLength),
		}
	}

	id, err := generateRandomString(length, nanoidAlphabet)
	if err != nil {
		return nil, &OperationError{
			Operation:  "id_nanoid",
			Message:    "failed to generate ID",
			ErrorType:  ErrorTypeInternal,
			Cause:      err,
			Suggestion: "Check system entropy source",
		}
	}

	return &Result{
		Response: id,
		Metadata: map[string]interface{}{
			"operation": "id_nanoid",
			"length":    length,
		},
	}, nil
}

// idCustom generates an ID with custom length and alphabet.
func (c *UtilityConnector) idCustom(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get required length parameter
	length, err := getInt(inputs, "length")
	if err != nil {
		return nil, &OperationError{
			Operation:  "id_custom",
			Message:    "invalid 'length' parameter",
			ErrorType:  ErrorTypeValidation,
			Cause:      err,
			Suggestion: "Provide 'length' as a positive integer",
		}
	}

	if length <= 0 {
		return nil, &OperationError{
			Operation:  "id_custom",
			Message:    "length must be positive",
			ErrorType:  ErrorTypeRange,
			Suggestion: "Provide a length >= 1",
		}
	}

	if length > c.config.MaxIDLength {
		return nil, &OperationError{
			Operation:  "id_custom",
			Message:    fmt.Sprintf("length (%d) exceeds maximum (%d)", length, c.config.MaxIDLength),
			ErrorType:  ErrorTypeRange,
			Suggestion: fmt.Sprintf("Reduce length to at most %d", c.config.MaxIDLength),
		}
	}

	// Get required alphabet parameter
	alphabetRaw, ok := inputs["alphabet"]
	if !ok {
		return nil, &OperationError{
			Operation:  "id_custom",
			Message:    "missing required parameter: alphabet",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide 'alphabet' as a string of characters to use",
		}
	}

	alphabet, ok := alphabetRaw.(string)
	if !ok {
		return nil, &OperationError{
			Operation:  "id_custom",
			Message:    "alphabet must be a string",
			ErrorType:  ErrorTypeType,
			Suggestion: "Provide 'alphabet' as a string",
		}
	}

	if len(alphabet) == 0 {
		return nil, &OperationError{
			Operation:  "id_custom",
			Message:    "alphabet cannot be empty",
			ErrorType:  ErrorTypeEmpty,
			Suggestion: "Provide at least one character in the alphabet",
		}
	}

	if len(alphabet) > 256 {
		return nil, &OperationError{
			Operation:  "id_custom",
			Message:    "alphabet exceeds maximum length of 256 characters",
			ErrorType:  ErrorTypeRange,
			Suggestion: "Reduce alphabet to at most 256 characters",
		}
	}

	// Validate alphabet characters (printable ASCII only for safety)
	for i, ch := range alphabet {
		if ch < 32 || ch > 126 {
			return nil, &OperationError{
				Operation:  "id_custom",
				Message:    fmt.Sprintf("alphabet contains invalid character at position %d", i),
				ErrorType:  ErrorTypeValidation,
				Suggestion: "Use only printable ASCII characters (space through tilde)",
			}
		}
	}

	// Check for problematic characters that might cause issues in various contexts
	problematicChars := `<>"'&\` + "`"
	for _, ch := range problematicChars {
		if strings.ContainsRune(alphabet, ch) {
			return nil, &OperationError{
				Operation:  "id_custom",
				Message:    fmt.Sprintf("alphabet contains potentially unsafe character: %c", ch),
				ErrorType:  ErrorTypeValidation,
				Suggestion: "Avoid characters that may cause issues in URLs, HTML, or shell: < > \" ' & \\ `",
			}
		}
	}

	id, err := generateRandomString(length, alphabet)
	if err != nil {
		return nil, &OperationError{
			Operation:  "id_custom",
			Message:    "failed to generate ID",
			ErrorType:  ErrorTypeInternal,
			Cause:      err,
			Suggestion: "Check system entropy source",
		}
	}

	return &Result{
		Response: id,
		Metadata: map[string]interface{}{
			"operation":       "id_custom",
			"length":          length,
			"alphabet_length": len(alphabet),
		},
	}, nil
}

// generateRandomString creates a random string of the specified length using the given alphabet.
func generateRandomString(length int, alphabet string) (string, error) {
	if len(alphabet) == 0 {
		return "", fmt.Errorf("alphabet cannot be empty")
	}

	result := make([]byte, length)

	// Generate random bytes
	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	alphabetLen := len(alphabet)
	for i := 0; i < length; i++ {
		// Use modulo to select from alphabet
		// This has slight bias for non-power-of-2 alphabet sizes, but acceptable for IDs
		result[i] = alphabet[int(randomBytes[i])%alphabetLen]
	}

	return string(result), nil
}
