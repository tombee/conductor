package permissions

import (
	"github.com/bmatcuk/doublestar/v4"
)

// CheckSecret checks if access to a secret is allowed.
// Returns nil if allowed, error if denied.
func CheckSecret(ctx *PermissionContext, secretName string) error {
	if ctx == nil || ctx.Secrets == nil {
		// No restrictions
		return nil
	}

	// If allowed secrets list is empty, deny all
	if len(ctx.Secrets.Allowed) == 0 {
		return &PermissionError{
			Type:     "secrets.access",
			Resource: secretName,
			Allowed:  ctx.Secrets.Allowed,
			Message:  "no secret permissions configured",
		}
	}

	// Check if secret name matches any allowed pattern
	for _, pattern := range ctx.Secrets.Allowed {
		matched, err := doublestar.Match(pattern, secretName)
		if err != nil {
			// Invalid pattern - skip it
			continue
		}
		if matched {
			return nil // Access allowed
		}
	}

	// Secret doesn't match any allowed pattern
	return &PermissionError{
		Type:     "secrets.access",
		Resource: secretName,
		Allowed:  ctx.Secrets.Allowed,
		Message:  "secret not in allowed patterns",
	}
}

// FilterAllowedSecrets filters a list of secret names to only include those allowed by permissions.
// This is used to provide a filtered view of available secrets to workflow steps.
func FilterAllowedSecrets(ctx *PermissionContext, secretNames []string) []string {
	if ctx == nil || ctx.Secrets == nil {
		// No restrictions - return all secrets
		return secretNames
	}

	// If allowed list is empty, deny all secrets
	if len(ctx.Secrets.Allowed) == 0 {
		return []string{}
	}

	allowed := make([]string, 0, len(secretNames))
	for _, secretName := range secretNames {
		if CheckSecret(ctx, secretName) == nil {
			allowed = append(allowed, secretName)
		}
	}

	return allowed
}
