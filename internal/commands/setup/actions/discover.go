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

package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ModelInfo represents a model from the /v1/models endpoint
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse is the response from the OpenAI-compatible /v1/models endpoint
type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// DiscoverModels queries the OpenAI-compatible /v1/models endpoint to discover
// available models. This supports most OpenAI-compatible gateways and APIs.
//
// Returns:
//   - List of model IDs
//   - Error if discovery fails
func DiscoverModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	// Create context with 10 second timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Build URL
	modelsURL := strings.TrimSuffix(baseURL, "/") + "/models"

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header if API key provided
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout after 10 seconds")
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("unauthorized (401): invalid API key")
		case http.StatusForbidden:
			return nil, fmt.Errorf("forbidden (403): API key lacks required permissions")
		case http.StatusNotFound:
			return nil, fmt.Errorf("not found (404): endpoint does not support model discovery")
		default:
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
	}

	// Parse response
	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract model IDs
	if len(modelsResp.Data) == 0 {
		return nil, fmt.Errorf("no models returned from endpoint")
	}

	modelIDs := make([]string, 0, len(modelsResp.Data))
	for _, model := range modelsResp.Data {
		if model.ID != "" {
			modelIDs = append(modelIDs, model.ID)
		}
	}

	return modelIDs, nil
}

// ModelTier represents the tier a model belongs to
type ModelTier string

const (
	TierFast      ModelTier = "fast"      // Quick tasks, low cost
	TierBalanced  ModelTier = "balanced"  // General purpose
	TierStrategic ModelTier = "strategic" // Complex reasoning
)

// ModelMapping maps tiers to specific model names
type ModelMapping struct {
	Fast      string
	Balanced  string
	Strategic string
}

// SuggestTierMappings attempts to intelligently map discovered models to tiers
// based on common model naming patterns.
func SuggestTierMappings(models []string) ModelMapping {
	mapping := ModelMapping{}

	// Common patterns for tier detection
	fastPatterns := []string{"haiku", "3.5-haiku", "mini", "small", "8b", "7b"}
	balancedPatterns := []string{"sonnet", "3.5-sonnet", "gpt-4o", "gpt-4", "70b"}
	strategicPatterns := []string{"opus", "claude-3-opus", "gpt-4-turbo", "405b"}

	// Find best match for each tier
	mapping.Fast = findBestMatch(models, fastPatterns, "haiku")
	mapping.Balanced = findBestMatch(models, balancedPatterns, "sonnet")
	mapping.Strategic = findBestMatch(models, strategicPatterns, "opus")

	// If no matches found, use first available model for each tier
	if mapping.Fast == "" && len(models) > 0 {
		mapping.Fast = models[0]
	}
	if mapping.Balanced == "" && len(models) > 0 {
		mapping.Balanced = models[0]
	}
	if mapping.Strategic == "" && len(models) > 0 {
		mapping.Strategic = models[0]
	}

	return mapping
}

// findBestMatch finds the best matching model from a list based on patterns
func findBestMatch(models []string, patterns []string, preferredSubstring string) string {
	// First pass: exact match on preferred substring
	for _, model := range models {
		if strings.Contains(strings.ToLower(model), preferredSubstring) {
			return model
		}
	}

	// Second pass: match any pattern
	for _, pattern := range patterns {
		for _, model := range models {
			if strings.Contains(strings.ToLower(model), pattern) {
				return model
			}
		}
	}

	return ""
}

// ValidateModelMapping validates that all tier mappings are non-empty
func ValidateModelMapping(mapping ModelMapping) error {
	if mapping.Fast == "" {
		return fmt.Errorf("fast tier model is required")
	}
	if mapping.Balanced == "" {
		return fmt.Errorf("balanced tier model is required")
	}
	if mapping.Strategic == "" {
		return fmt.Errorf("strategic tier model is required")
	}
	return nil
}

// FormatModelList formats a list of models for display
func FormatModelList(models []string) string {
	if len(models) == 0 {
		return "No models available"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("%d models available:", len(models)))
	for i, model := range models {
		if i < 10 {
			lines = append(lines, fmt.Sprintf("  • %s", model))
		}
	}
	if len(models) > 10 {
		lines = append(lines, fmt.Sprintf("  ... and %d more", len(models)-10))
	}

	return strings.Join(lines, "\n")
}
