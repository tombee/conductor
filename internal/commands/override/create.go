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

package override

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/output"
	"github.com/tombee/conductor/pkg/security"
)

type createFlags struct {
	reason string
	ttl    string
	json   bool
}

func newCreateCommand() *cobra.Command {
	flags := &createFlags{}

	cmd := &cobra.Command{
		Use:   "create <type>",
		Short: "Create a security override",
		Long: `Create a security override to temporarily bypass security controls.

Available override types:
  - disable-enforcement: Bypass security policy enforcement
  - disable-sandbox: Bypass sandboxing restrictions

All overrides are time-limited and fully audited.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			overrideType := args[0]

			// Validate override type
			if !isValidOverrideType(overrideType) {
				return fmt.Errorf("invalid override type: %s (valid types: disable-enforcement, disable-sandbox)", overrideType)
			}

			// Block disable-audit type at CLI level (defense in depth)
			if overrideType == string(security.OverrideDisableAudit) {
				return fmt.Errorf("disable-audit override type is not allowed")
			}

			// Validate reason
			if flags.reason == "" {
				return fmt.Errorf("reason is required (use --reason)")
			}

			// Create override via controller API
			reqBody := map[string]string{
				"type":   overrideType,
				"reason": flags.reason,
			}
			if flags.ttl != "" {
				reqBody["ttl"] = flags.ttl
			}

			reqJSON, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("failed to marshal request: %w", err)
			}

			url := shared.BuildAPIURL("/v1/override", nil)
			body, err := shared.MakeAPIRequest("POST", url, reqJSON)
			if err != nil {
				return fmt.Errorf("failed to create override: %w", err)
			}

			// Parse response
			var override struct {
				Type      string    `json:"type"`
				Reason    string    `json:"reason"`
				ExpiresAt time.Time `json:"expires_at"`
				CreatedAt time.Time `json:"created_at"`
			}
			if err := json.Unmarshal(body, &override); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			// Format output
			if flags.json {
				return output.EmitJSON(override)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Override created successfully\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Type:       %s\n", override.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "  Reason:     %s\n", override.Reason)
			fmt.Fprintf(cmd.OutOrStdout(), "  Expires at: %s\n", override.ExpiresAt.Format(time.RFC3339))
			fmt.Fprintf(cmd.OutOrStdout(), "  Created at: %s\n", override.CreatedAt.Format(time.RFC3339))

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.reason, "reason", "r", "", "Reason for creating the override (required)")
	cmd.Flags().StringVarP(&flags.ttl, "ttl", "t", "", "Time-to-live for the override (e.g., '1h', '30m'; default: 1h)")
	cmd.Flags().BoolVar(&flags.json, "json", false, "Output in JSON format")

	cmd.MarkFlagRequired("reason")

	return cmd
}

// isValidOverrideType checks if the override type is valid and allowed via CLI.
func isValidOverrideType(t string) bool {
	switch security.OverrideType(t) {
	case security.OverrideDisableEnforcement,
		security.OverrideDisableSandbox:
		return true
	case security.OverrideDisableAudit:
		// disable-audit is valid but not allowed via CLI
		return false
	default:
		return false
	}
}
