package shared

import (
	"fmt"
	"strings"

	"github.com/tombee/conductor/internal/operation"
)

// DryRunAction represents an action type in dry-run output.
type DryRunAction string

const (
	// DryRunActionCreate indicates a resource would be created.
	DryRunActionCreate DryRunAction = "CREATE"
	// DryRunActionModify indicates a resource would be modified.
	DryRunActionModify DryRunAction = "MODIFY"
	// DryRunActionDelete indicates a resource would be deleted.
	DryRunActionDelete DryRunAction = "DELETE"
)

// DryRunOutput formats dry-run output in a consistent way across commands.
// It shows what actions would be performed without executing them.
type DryRunOutput struct {
	actions []string
}

// NewDryRunOutput creates a new dry-run output formatter.
func NewDryRunOutput() *DryRunOutput {
	return &DryRunOutput{
		actions: make([]string, 0),
	}
}

// DryRunCreate adds a CREATE action to the dry-run output.
// The path should use placeholders like <config-dir> instead of full system paths.
func (d *DryRunOutput) DryRunCreate(path string) {
	d.actions = append(d.actions, fmt.Sprintf("%s: %s", DryRunActionCreate, path))
}

// DryRunCreateWithDescription adds a CREATE action with additional description.
// Example: DryRunCreateWithDescription("<config-dir>/config.yaml", "with default providers")
func (d *DryRunOutput) DryRunCreateWithDescription(path, description string) {
	d.actions = append(d.actions, fmt.Sprintf("%s: %s (%s)", DryRunActionCreate, path, description))
}

// DryRunModify adds a MODIFY action to the dry-run output.
// The path should use placeholders like <config-dir> instead of full system paths.
// The description should briefly explain what would change.
func (d *DryRunOutput) DryRunModify(path, description string) {
	d.actions = append(d.actions, fmt.Sprintf("%s: %s (%s)", DryRunActionModify, path, description))
}

// DryRunDelete adds a DELETE action to the dry-run output.
// The path should use placeholders like <config-dir> instead of full system paths.
func (d *DryRunOutput) DryRunDelete(path string) {
	d.actions = append(d.actions, fmt.Sprintf("%s: %s", DryRunActionDelete, path))
}

// DryRunDeleteWithCount adds a DELETE action with count information.
// Example: DryRunDeleteWithCount("<cache-dir>", "5 entries")
func (d *DryRunOutput) DryRunDeleteWithCount(path, count string) {
	d.actions = append(d.actions, fmt.Sprintf("%s: %s (%s)", DryRunActionDelete, path, count))
}

// String returns the formatted dry-run output.
// Format:
//   Dry run: The following actions would be performed:
//
//   CREATE: <config-dir>/config.yaml
//   MODIFY: <config-dir>/config.yaml (add provider)
//   DELETE: <cache-dir>/session-123
//
//   Run without --dry-run to execute.
func (d *DryRunOutput) String() string {
	if len(d.actions) == 0 {
		return "Dry run: No actions would be performed."
	}

	var sb strings.Builder
	sb.WriteString("Dry run: The following actions would be performed:\n\n")

	for _, action := range d.actions {
		sb.WriteString(action)
		sb.WriteString("\n")
	}

	sb.WriteString("\nRun without --dry-run to execute.")

	return sb.String()
}

// MaskSensitiveData masks sensitive values in dry-run output.
// This is a convenience wrapper around operation.MaskSensitiveValue.
func MaskSensitiveData(key, value string) string {
	return operation.MaskSensitiveValue(key, value)
}

// PlaceholderPath converts a full system path to a placeholder path for dry-run output.
// This prevents leaking full system paths to agents.
// Examples:
//   - /Users/john/.config/conductor/config.yaml -> <config-dir>/config.yaml
//   - /home/user/.conductor/workflows/ -> <config-dir>/workflows/
//   - /tmp/conductor/cache/session-123 -> <cache-dir>/session-123
func PlaceholderPath(fullPath, baseDir, placeholder string) string {
	// Simple replacement for now - can be enhanced with path cleaning if needed
	return strings.Replace(fullPath, baseDir, placeholder, 1)
}
