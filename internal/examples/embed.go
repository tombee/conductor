package examples

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Embed example workflows into the binary for offline availability
//
//go:embed *.yaml
var embeddedFS embed.FS

// Example represents metadata about an embedded example workflow
type Example struct {
	Name        string
	Description string
	FilePath    string
}

// List returns all available embedded examples
func List() ([]Example, error) {
	entries, err := embeddedFS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded examples: %w", err)
	}

	var examples []Example
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml")
		examples = append(examples, Example{
			Name:        name,
			Description: getDescription(name),
			FilePath:    entry.Name(),
		})
	}

	return examples, nil
}

// Get returns the content of a specific example by name
func Get(name string) ([]byte, error) {
	filename := name + ".yaml"
	content, err := embeddedFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("example %q not found: %w", name, err)
	}
	return content, nil
}

// Exists checks if an example with the given name exists
func Exists(name string) bool {
	filename := name + ".yaml"
	_, err := embeddedFS.ReadFile(filename)
	return err == nil
}

// CopyTo writes an example to the filesystem at the specified destination
func CopyTo(name string, destPath string) error {
	content, err := Get(name)
	if err != nil {
		return err
	}

	// Ensure the destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write example file: %w", err)
	}

	return nil
}

// getDescription returns a human-readable description for each example
func getDescription(name string) string {
	descriptions := map[string]string{
		"quickstart": "Simple hello world workflow for testing Conductor",
	}

	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return "Example workflow"
}
