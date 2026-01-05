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

package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tombee/conductor/pkg/httpclient"
	"github.com/tombee/conductor/pkg/llm"
)

const (
	// defaultOllamaURL is the default Ollama API endpoint
	defaultOllamaURL = "http://localhost:11434"
)

// OllamaProvider implements basic Ollama provider functionality.
// Currently supports model discovery; full completion support to be added.
type OllamaProvider struct {
	baseURL    string
	httpClient *http.Client
}

// NewOllamaProvider creates a new Ollama provider instance.
func NewOllamaProvider(baseURL string) (*OllamaProvider, error) {
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}

	// Create HTTP client
	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 10 * time.Second
	cfg.UserAgent = "conductor-ollama/1.0"
	cfg.RetryAttempts = 0

	httpClient, err := httpclient.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &OllamaProvider{
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

// NewOllamaWithCredentials creates an Ollama provider from credentials.
// Ollama doesn't require authentication by default, but supports base_url override.
func NewOllamaWithCredentials(creds llm.Credentials) (llm.Provider, error) {
	ollamaCreds, ok := creds.(llm.OllamaCredentials)
	if !ok {
		// Try empty credentials for discovery-only usage
		return NewOllamaProvider("")
	}
	return NewOllamaProvider(ollamaCreds.BaseURL)
}

// Name returns the provider identifier.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Capabilities returns the features supported by this provider.
func (p *OllamaProvider) Capabilities() llm.Capabilities {
	// Return empty models list - discovery will populate this dynamically
	return llm.Capabilities{
		Streaming: false, // Not yet implemented
		Tools:     false, // Not yet implemented
		Models:    []llm.ModelInfo{},
	}
}

// DiscoverModels queries Ollama's /api/tags endpoint to discover installed models.
func (p *OllamaProvider) DiscoverModels(ctx context.Context) ([]llm.ModelInfo, error) {
	url := fmt.Sprintf("%s/api/tags", p.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert Ollama models to ModelInfo format
	models := make([]llm.ModelInfo, 0, len(tagsResp.Models))
	for _, model := range tagsResp.Models {
		models = append(models, llm.ModelInfo{
			ID:                    model.Name,
			MaxTokens:             0, // Ollama doesn't expose this in /api/tags
			InputPricePerMillion:  0, // Ollama is free/local
			OutputPricePerMillion: 0, // Ollama is free/local
		})
	}

	return models, nil
}

// Complete sends a completion request to the Ollama API.
func (p *OllamaProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	url := fmt.Sprintf("%s/api/chat", p.baseURL)

	// Convert messages to Ollama format
	messages := make([]ollamaChatMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, ollamaChatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	// Build request body
	chatReq := ollamaChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   false,
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Use a longer timeout for completions
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &llm.CompletionResponse{
		Content: chatResp.Message.Content,
		Model:   chatResp.Model,
		Usage: llm.TokenUsage{
			InputTokens:  chatResp.PromptEvalCount,
			OutputTokens: chatResp.EvalCount,
		},
	}, nil
}

// Stream is not yet implemented for Ollama provider.
func (p *OllamaProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("ollama provider does not yet support streaming")
}

// ollamaTagsResponse represents the response from GET /api/tags
type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

// ollamaModel represents a single model in the /api/tags response
type ollamaModel struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
	Details    struct {
		Format            string   `json:"format"`
		Family            string   `json:"family"`
		Families          []string `json:"families"`
		ParameterSize     string   `json:"parameter_size"`
		QuantizationLevel string   `json:"quantization_level"`
	} `json:"details"`
}

// ollamaChatRequest represents a request to POST /api/chat
type ollamaChatRequest struct {
	Model    string               `json:"model"`
	Messages []ollamaChatMessage  `json:"messages"`
	Stream   bool                 `json:"stream"`
}

// ollamaChatMessage represents a single message in the chat
type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaChatResponse represents the response from POST /api/chat
type ollamaChatResponse struct {
	Model           string            `json:"model"`
	Message         ollamaChatMessage `json:"message"`
	Done            bool              `json:"done"`
	TotalDuration   int64             `json:"total_duration"`
	LoadDuration    int64             `json:"load_duration"`
	PromptEvalCount int               `json:"prompt_eval_count"`
	EvalCount       int               `json:"eval_count"`
}
