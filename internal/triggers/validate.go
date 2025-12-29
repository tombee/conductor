package triggers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/tombee/conductor/internal/controller/scheduler"
)

var (
	// secretRefRegex matches ${VAR_NAME} format.
	secretRefRegex = regexp.MustCompile(`^\$\{[A-Z_][A-Z0-9_]*\}$`)

	// jsonPathRegex is a simple validation for JSONPath syntax.
	// Full validation happens at runtime when the webhook is triggered.
	jsonPathRegex = regexp.MustCompile(`^\$\.?[a-zA-Z0-9_\[\]\.\*]+$`)
)

// ValidateWorkflowExists checks if a workflow file exists.
func ValidateWorkflowExists(workflowsDir, workflow string) error {
	if workflowsDir == "" {
		return fmt.Errorf("workflows directory not configured")
	}

	fullPath := filepath.Join(workflowsDir, workflow)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("workflow file not found: %s", workflow)
	} else if err != nil {
		return fmt.Errorf("failed to check workflow file: %w", err)
	}

	return nil
}

// ValidateCron validates a cron expression using the scheduler's parser.
func ValidateCron(expr string) error {
	if expr == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}

	_, err := scheduler.ParseCron(expr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %s", expr)
	}

	return nil
}

// ValidateJSONPath validates JSONPath expression syntax.
func ValidateJSONPath(expr string) error {
	if expr == "" {
		return nil // Empty is valid (no mapping)
	}

	if !jsonPathRegex.MatchString(expr) {
		return fmt.Errorf("invalid JSONPath: %s", expr)
	}

	return nil
}

// ValidateSecretRef validates that a secret reference matches ${VAR_NAME} format.
func ValidateSecretRef(secret string) error {
	if secret == "" {
		return nil // Empty is valid (no secret)
	}

	if !secretRefRegex.MatchString(secret) {
		return fmt.Errorf("invalid secret format, use ${VAR_NAME}")
	}

	return nil
}

// ValidateTimezone validates that a timezone is a valid IANA timezone.
func ValidateTimezone(tz string) error {
	if tz == "" {
		return nil // Empty is valid (defaults to UTC)
	}

	_, err := time.LoadLocation(tz)
	if err != nil {
		return fmt.Errorf("invalid timezone: %s", tz)
	}

	return nil
}
