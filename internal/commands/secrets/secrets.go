// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secrets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/secrets"
)

var (
	secretBackend string
	secretUnmask  bool
	secretForce   bool
	secretDryRun  bool
	secretYes     bool
)

// NewCommand creates the secrets command for secret management.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "secrets",
		Annotations: map[string]string{
			"group": "configuration",
		},
		Short: "Manage secure secrets (API keys, credentials)",
		Long: `Manage secrets securely using multiple backends.

Secrets are stored in a tiered backend system with automatic fallback:
  1. Environment variables (highest priority, read-only)
  2. System keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager)

Commands:
  set       Store a secret securely
  get       Retrieve a secret value
  list      List all secret keys
  delete    Remove a secret

Examples:
  conductor secrets set providers/anthropic/api_key
  conductor secrets get providers/anthropic/api_key
  conductor secrets list
  conductor secrets delete providers/anthropic/api_key`,
	}

	cmd.AddCommand(newSecretsSetCommand())
	cmd.AddCommand(newSecretsGetCommand())
	cmd.AddCommand(newSecretsListCommand())
	cmd.AddCommand(newSecretsDeleteCommand())

	return cmd
}

func newSecretsSetCommand() *cobra.Command {
	var setDryRun bool

	cmd := &cobra.Command{
		Use:   "set <key>",
		Short: "Store a secret securely",
		Long: `Store a secret in the specified backend.

The secret value can be provided via:
  - Interactive prompt (hidden input, default)
  - Standard input: echo "value" | conductor secrets set <key>

Key Format:
  Hierarchical format: namespace/subkey
  Examples:
    providers/anthropic/api_key
    providers/openai/api_key
    webhooks/github/secret

Backend Selection:
  --backend <name>  Target specific backend (env, keychain, file)
  Default: First available writable backend (usually keychain)

Examples:
  conductor secrets set providers/anthropic/api_key
  conductor secrets set providers/openai/api_key --backend file
  echo "sk-..." | conductor secrets set providers/anthropic/api_key`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretsSet(cmd, args, setDryRun)
		},
	}

	cmd.Flags().StringVar(&secretBackend, "backend", "", "Target backend (env, keychain, file)")
	cmd.Flags().BoolVar(&setDryRun, "dry-run", false, "Show what would be stored without executing")

	return cmd
}

func newSecretsGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Retrieve a secret value",
		Long: `Retrieve a secret value from any available backend.

By default, the value is masked for security. Use --unmask to show the full value.

Examples:
  conductor secrets get providers/anthropic/api_key
  conductor secrets get providers/anthropic/api_key --unmask`,
		Args: cobra.ExactArgs(1),
		RunE: runSecretsGet,
	}

	cmd.Flags().BoolVar(&secretUnmask, "unmask", false, "Show full value (not masked)")

	return cmd
}

func newSecretsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all secret keys",
		Long: `List all secret keys across all backends.

Shows:
  - Secret key
  - Backend providing the secret
  - Read-only status

Does not show secret values for security.

Examples:
  conductor secrets list
  conductor secrets list --json`,
		Args: cobra.NoArgs,
		RunE: runSecretsList,
	}

	return cmd
}

func newSecretsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <key>",
		Short: "Remove a secret",
		Long: `Remove a secret from the specified backend.

Requires confirmation unless --force is used.

Examples:
  conductor secrets delete providers/anthropic/api_key
  conductor secrets delete providers/anthropic/api_key --backend keychain
  conductor secrets delete providers/anthropic/api_key --force`,
		Args: cobra.ExactArgs(1),
		RunE: runSecretsDelete,
	}

	cmd.Flags().StringVar(&secretBackend, "backend", "", "Target backend (env, keychain, file)")
	cmd.Flags().BoolVar(&secretForce, "force", false, "Skip confirmation prompt")

	return cmd
}

// runSecretsSet handles the 'secrets set' command.
func runSecretsSet(cmd *cobra.Command, args []string, dryRun bool) error {
	key := args[0]

	// Validate key format
	if err := validateSecretKey(key); err != nil {
		return err
	}

	// In dry-run mode, don't read the actual value - just simulate
	if dryRun {
		return secretsSetDryRun(key)
	}

	// Read secret value from stdin or prompt
	value, err := readSecretValue()
	if err != nil {
		return fmt.Errorf("failed to read secret value: %w", err)
	}

	if value == "" {
		return errors.New("secret value cannot be empty")
	}

	// Create resolver with available backends
	resolver := createResolver()

	// Store the secret
	ctx := context.Background()
	if err := resolver.Set(ctx, key, value, secretBackend); err != nil {
		if errors.Is(err, secrets.ErrBackendUnavailable) {
			return fmt.Errorf("backend unavailable: %w\n\nTry:\n  1. Use --backend to specify a different backend\n  2. Set environment variable: export CONDUCTOR_SECRET_%s=<value>\n  3. Check keychain accessibility", err, normalizeEnvKey(key))
		}
		return fmt.Errorf("failed to set secret: %w", err)
	}

	// Determine which backend was used
	backendUsed := secretBackend
	if backendUsed == "" {
		// Find first writable backend
		for _, b := range resolver.Backends() {
			if ro, ok := b.(secrets.ReadOnlyBackend); !ok || !ro.ReadOnly() {
				backendUsed = b.Name()
				break
			}
		}
	}

	fmt.Println(shared.RenderOK(fmt.Sprintf("Secret stored successfully in %s backend", backendUsed)))
	return nil
}

// runSecretsGet handles the 'secrets get' command.
func runSecretsGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	// Create resolver with available backends
	resolver := createResolver()

	// Retrieve the secret
	ctx := context.Background()
	value, err := resolver.Get(ctx, key)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return fmt.Errorf("secret not found: %q\n\nSet it with: conductor secrets set %s", key, key)
		}
		if errors.Is(err, secrets.ErrBackendUnavailable) {
			return fmt.Errorf("backend unavailable: %w", err)
		}
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Display the value (masked or unmasked)
	if secretUnmask {
		fmt.Println(value)
	} else {
		masked := maskSecret(value)
		fmt.Printf("%s (use --unmask to show full value)\n", masked)
	}

	return nil
}

// runSecretsList handles the 'secrets list' command.
func runSecretsList(cmd *cobra.Command, args []string) error {
	// Create resolver with available backends
	resolver := createResolver()

	// List all secrets
	ctx := context.Background()
	metadata, err := resolver.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list secrets: %w", err)
	}

	if len(metadata) == 0 {
		fmt.Println("No secrets found")
		return nil
	}

	// Format output
	fmt.Printf("%-50s %-15s %s\n", "KEY", "BACKEND", "READ-ONLY")
	fmt.Println(strings.Repeat("-", 80))

	for _, meta := range metadata {
		readOnly := ""
		if meta.ReadOnly {
			readOnly = "yes"
		} else {
			readOnly = "no"
		}
		fmt.Printf("%-50s %-15s %s\n", meta.Key, meta.Backend, readOnly)
	}

	fmt.Printf("\nTotal: %d secret(s)\n", len(metadata))
	return nil
}

// runSecretsDelete handles the 'secrets delete' command.
func runSecretsDelete(cmd *cobra.Command, args []string) error {
	key := args[0]

	// Confirm deletion unless --force is used
	if !secretForce {
		fmt.Printf("Are you sure you want to delete secret %q? [y/N]: ", key)
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Deletion canceled")
			return nil
		}
	}

	// Create resolver with available backends
	resolver := createResolver()

	// Delete the secret
	ctx := context.Background()
	if err := resolver.Delete(ctx, key, secretBackend); err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return fmt.Errorf("secret not found: %q", key)
		}
		if errors.Is(err, secrets.ErrReadOnlyBackend) {
			return errors.New("cannot delete from read-only backend (environment variables)")
		}
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	fmt.Println(shared.RenderOK(fmt.Sprintf("Secret %q deleted successfully", key)))
	return nil
}

// Helper functions

// createResolver creates a secrets resolver with available backends.
func createResolver() *secrets.Resolver {
	// Create file backend with default path and empty master key
	// (it will try to resolve from env or file)
	fileBackend, _ := secrets.NewFileBackend("", "")

	backends := []secrets.SecretBackend{
		secrets.NewEnvBackend(),
		secrets.NewKeychainBackend(),
		fileBackend,
	}
	return secrets.NewResolver(backends...)
}

// readSecretValue reads a secret value from stdin or prompts the user.
func readSecretValue() (string, error) {
	// Check if stdin is a pipe
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Reading from pipe
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}

	// Interactive prompt with hidden input
	fmt.Print("Enter secret value (hidden): ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // New line after hidden input
	if err != nil {
		return "", err
	}

	return string(bytePassword), nil
}

// maskSecret masks a secret value for display.
func maskSecret(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	// Show first 4 and last 4 characters
	return value[:4] + "..." + value[len(value)-4:]
}

// validateSecretKey validates a secret key format.
func validateSecretKey(key string) error {
	if key == "" {
		return errors.New("secret key cannot be empty")
	}

	if strings.Contains(key, " ") {
		return errors.New("secret key cannot contain spaces")
	}

	// Keys should use forward slashes, not backslashes
	if strings.Contains(key, "\\") {
		return errors.New("secret key should use forward slashes (/), not backslashes (\\)")
	}

	return nil
}

// normalizeEnvKey converts a secret key to environment variable format.
func normalizeEnvKey(key string) string {
	return strings.ToUpper(strings.ReplaceAll(key, "/", "_"))
}

// getConfigDir returns the conductor config directory.
func getConfigDir() (string, error) {
	return config.ConfigDir()
}

// secretsSetDryRun shows what would be stored when setting a secret
func secretsSetDryRun(key string) error {
	output := shared.NewDryRunOutput()

	// Determine the backend that would be used
	backendName := secretBackend
	if backendName == "" {
		// Find first writable backend
		resolver := createResolver()
		for _, b := range resolver.Backends() {
			if ro, ok := b.(secrets.ReadOnlyBackend); !ok || !ro.ReadOnly() {
				backendName = b.Name()
				break
			}
		}
	}

	// Build description - always mask the value
	description := fmt.Sprintf("store secret '%s' (value: [REDACTED], backend: %s)", key, backendName)

	// The secret storage location depends on the backend, but we use a generic placeholder
	var storagePath string
	switch backendName {
	case "keychain":
		storagePath = "<system-keychain>"
	case "env":
		storagePath = "<environment-variable>"
	default:
		storagePath = "<secret-backend>"
	}

	output.DryRunModify(storagePath, description)

	// Print the output
	fmt.Println(output.String())

	return nil
}
