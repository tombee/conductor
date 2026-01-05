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

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

// ProviderStatus represents the status of a provider for display
type ProviderStatus struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured providers",
		Long:  "Display all configured providers with their types and health status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			// Load configuration
			cfgPath, err := getConfigPathOrDefault()
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			useJSON := shared.GetJSON()

			// Check if no providers configured
			if len(cfg.Providers) == 0 {
				if useJSON {
					fmt.Fprintln(out, `{"providers":[]}`)
					return nil
				}
				fmt.Fprintln(out, shared.RenderWarn("No providers configured."))
				fmt.Fprintln(out)
				fmt.Fprintf(out, "Run %s to set up an LLM provider.\n",
					shared.Bold.Render("conductor provider add"))
				return nil
			}

			// Gather provider status
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			statuses := make([]ProviderStatus, 0, len(cfg.Providers))
			for name, providerCfg := range cfg.Providers {
				status := ProviderStatus{
					Name: name,
					Type: providerCfg.Type,
				}

				// Run health check for each provider
				healthResult := checkProviderHealth(ctx, providerCfg)
				if healthResult.Healthy() {
					status.Status = "OK"
				} else {
					status.Status = "ERROR"
					status.Message = formatHealthError(healthResult)
				}

				statuses = append(statuses, status)
			}

			// Output results
			if useJSON {
				result := map[string][]ProviderStatus{"providers": statuses}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Table output with colors
			fmt.Println(shared.Header.Render("Configured Providers"))
			fmt.Println()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				shared.Bold.Render("NAME"),
				shared.Bold.Render("TYPE"),
				shared.Bold.Render("STATUS"))
			for _, s := range statuses {
				var statusDisplay string
				if s.Status == "OK" {
					statusDisplay = shared.StatusOK.Render(shared.SymbolOK + " OK")
				} else {
					msg := s.Message
					if msg == "" {
						msg = "ERROR"
					}
					statusDisplay = shared.StatusError.Render(shared.SymbolError + " " + msg)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, shared.Muted.Render(s.Type), statusDisplay)
			}
			w.Flush()

			return nil
		},
	}

	return cmd
}

// getConfigPathOrDefault returns the config path from the flag or falls back to XDG default
func getConfigPathOrDefault() (string, error) {
	cfgPath := shared.GetConfigPath()
	if cfgPath == "" {
		return config.SettingsPath()
	}
	return cfgPath, nil
}

// checkProviderHealth runs a health check for the given provider configuration
func checkProviderHealth(ctx context.Context, providerCfg config.ProviderConfig) llm.HealthCheckResult {
	// For now, only implement claude-code health checks
	// Other providers will be added as they're implemented
	switch providerCfg.Type {
	case "claude-code":
		p := &claudecode.Provider{}
		if hc, ok := interface{}(p).(llm.HealthCheckable); ok {
			return hc.HealthCheck(ctx)
		}
	case "ollama":
		baseURL := providerCfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		resp, err := http.Get(baseURL + "/api/tags")
		if err != nil {
			return llm.HealthCheckResult{
				Installed:     true,
				Authenticated: true,
				Working:       false,
				Message:       fmt.Sprintf("Cannot connect to Ollama at %s: %v", baseURL, err),
			}
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return llm.HealthCheckResult{
				Installed:     true,
				Authenticated: true,
				Working:       false,
				Message:       fmt.Sprintf("Ollama returned status %d", resp.StatusCode),
			}
		}
		return llm.HealthCheckResult{
			Installed:     true,
			Authenticated: true,
			Working:       true,
			Message:       fmt.Sprintf("Connected to Ollama at %s", baseURL),
		}
	case "anthropic", "openai":
		// Not yet implemented - return basic result
		return llm.HealthCheckResult{
			Installed:     true,
			Authenticated: true,
			Working:       false,
			Message:       fmt.Sprintf("Health checks not yet implemented for %s provider", providerCfg.Type),
		}
	}

	return llm.HealthCheckResult{
		Error:   fmt.Errorf("unknown provider type: %s", providerCfg.Type),
		Message: "Unknown provider type",
	}
}

// formatHealthError formats a health check result error message
func formatHealthError(result llm.HealthCheckResult) string {
	if result.Error != nil {
		return result.Error.Error()
	}
	if !result.Installed {
		return "not installed"
	}
	if !result.Authenticated {
		return "not authenticated"
	}
	if !result.Working {
		return "connectivity failed"
	}
	return "unknown error"
}
