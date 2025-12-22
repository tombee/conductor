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

package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers"
)

func TestLLMHandlers_HandleComplete(t *testing.T) {
	// Setup
	registry := llm.NewRegistry()
	costTracker := llm.NewCostTracker()
	mockProvider := providers.NewMockAnthropicProvider()
	registry.Register(mockProvider)
	registry.SetDefault("anthropic")

	handlers := NewLLMHandlers(registry, costTracker)

	// Create request
	reqData := CompleteRequest{
		Provider: "default",
		Model:    "balanced",
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello"},
		},
		Metadata: map[string]string{
			"test": "value",
		},
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	msg := &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-123",
		Method:        "llm.complete",
		Params:        reqBytes,
	}

	// Execute
	ctx := context.Background()
	resp, err := handlers.HandleComplete(ctx, msg)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Type != MessageTypeResponse {
		t.Errorf("expected response type, got %s", resp.Type)
	}

	if resp.CorrelationID != "test-123" {
		t.Errorf("expected correlation ID test-123, got %s", resp.CorrelationID)
	}

	var result CompleteResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Content == "" {
		t.Error("expected content in response")
	}

	if result.RequestID == "" {
		t.Error("expected request ID in response")
	}

	// Verify cost tracking
	records := costTracker.GetRecords()
	if len(records) != 1 {
		t.Errorf("expected 1 cost record, got %d", len(records))
	}

	if len(records) > 0 {
		record := records[0]
		if record.Provider != "anthropic" {
			t.Errorf("expected provider anthropic, got %s", record.Provider)
		}
		if record.Metadata["test"] != "value" {
			t.Errorf("expected metadata test=value, got %v", record.Metadata)
		}
	}
}

func TestLLMHandlers_HandleComplete_InvalidRequest(t *testing.T) {
	registry := llm.NewRegistry()
	costTracker := llm.NewCostTracker()
	handlers := NewLLMHandlers(registry, costTracker)

	msg := &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-123",
		Method:        "llm.complete",
		Params:        []byte("invalid json"),
	}

	ctx := context.Background()
	_, err := handlers.HandleComplete(ctx, msg)

	if err == nil {
		t.Error("expected error for invalid request")
	}
}

func TestLLMHandlers_HandleComplete_NoProvider(t *testing.T) {
	registry := llm.NewRegistry()
	costTracker := llm.NewCostTracker()
	handlers := NewLLMHandlers(registry, costTracker)

	reqData := CompleteRequest{
		Provider: "default",
		Model:    "balanced",
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello"},
		},
	}

	reqBytes, _ := json.Marshal(reqData)

	msg := &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-123",
		Method:        "llm.complete",
		Params:        reqBytes,
	}

	ctx := context.Background()
	_, err := handlers.HandleComplete(ctx, msg)

	if err == nil {
		t.Error("expected error when no default provider is set")
	}
}

func TestLLMHandlers_HandleListProviders(t *testing.T) {
	registry := llm.NewRegistry()
	costTracker := llm.NewCostTracker()

	mockProvider := providers.NewMockAnthropicProvider()
	registry.Register(mockProvider)

	handlers := NewLLMHandlers(registry, costTracker)

	msg := &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-123",
		Method:        "llm.listProviders",
		Params:        []byte("{}"),
	}

	ctx := context.Background()
	resp, err := handlers.HandleListProviders(ctx, msg)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	providers, ok := result["providers"].([]interface{})
	if !ok {
		t.Fatal("expected providers array in response")
	}

	if len(providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(providers))
	}
}

func TestLLMHandlers_HandleGetProvider(t *testing.T) {
	registry := llm.NewRegistry()
	costTracker := llm.NewCostTracker()

	mockProvider := providers.NewMockAnthropicProvider()
	registry.Register(mockProvider)
	registry.SetDefault("anthropic")

	handlers := NewLLMHandlers(registry, costTracker)

	reqData := GetProviderRequest{
		Provider: "anthropic",
	}

	reqBytes, _ := json.Marshal(reqData)

	msg := &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-123",
		Method:        "llm.getProvider",
		Params:        reqBytes,
	}

	ctx := context.Background()
	resp, err := handlers.HandleGetProvider(ctx, msg)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var result GetProviderResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Name != "anthropic" {
		t.Errorf("expected name anthropic, got %s", result.Name)
	}

	if !result.Capabilities.Streaming {
		t.Error("expected streaming capability")
	}

	if !result.Capabilities.Tools {
		t.Error("expected tools capability")
	}
}

func TestLLMHandlers_HandleGetCostRecords(t *testing.T) {
	registry := llm.NewRegistry()
	costTracker := llm.NewCostTracker()

	// Add some test records
	costTracker.Track(llm.CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Usage: llm.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		Cost: &llm.CostInfo{
			Amount:   0.001,
			Currency: "USD",
			Accuracy: llm.CostMeasured,
			Source:   llm.SourceProvider,
		},
	})

	handlers := NewLLMHandlers(registry, costTracker)

	reqData := GetCostRecordsRequest{}
	reqBytes, _ := json.Marshal(reqData)

	msg := &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-123",
		Method:        "llm.getCostRecords",
		Params:        reqBytes,
	}

	ctx := context.Background()
	resp, err := handlers.HandleGetCostRecords(ctx, msg)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var result GetCostRecordsResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(result.Records))
	}

	if len(result.Records) > 0 {
		record := result.Records[0]
		if record.RequestID != "req-1" {
			t.Errorf("expected request ID req-1, got %s", record.RequestID)
		}
		if record.Provider != "anthropic" {
			t.Errorf("expected provider anthropic, got %s", record.Provider)
		}
	}
}

func TestLLMHandlers_HandleGetCostRecords_NoTracker(t *testing.T) {
	registry := llm.NewRegistry()
	handlers := NewLLMHandlers(registry, nil) // No cost tracker

	reqData := GetCostRecordsRequest{}
	reqBytes, _ := json.Marshal(reqData)

	msg := &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-123",
		Method:        "llm.getCostRecords",
		Params:        reqBytes,
	}

	ctx := context.Background()
	_, err := handlers.HandleGetCostRecords(ctx, msg)

	if err == nil {
		t.Error("expected error when cost tracker is not enabled")
	}
}

func TestLLMHandlers_RegisterHandlers(t *testing.T) {
	registry := llm.NewRegistry()
	costTracker := llm.NewCostTracker()
	handlers := NewLLMHandlers(registry, costTracker)

	rpcRegistry := NewRegistry()
	handlers.RegisterHandlers(rpcRegistry)

	// Verify handlers are registered
	methods := []string{
		"llm.complete",
		"llm.stream",
		"llm.listProviders",
		"llm.getProvider",
		"llm.getCostRecords",
	}

	for _, method := range methods {
		if !rpcRegistry.HasMethod(method) {
			t.Errorf("expected method %s to be registered", method)
		}
	}
}
