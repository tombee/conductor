package pricing

import (
	"testing"
	"time"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name            string
		pricing         *ModelPricing
		usage           TokenUsage
		wantAmount      float64
		wantAccuracy    CostAccuracy
		wantSource      string
	}{
		{
			name: "basic cost calculation",
			pricing: &ModelPricing{
				Provider:              "anthropic",
				Model:                 "claude-3-opus-20240229",
				InputPricePerMillion:  15.00,
				OutputPricePerMillion: 75.00,
				EffectiveDate:         time.Now(),
			},
			usage: TokenUsage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			wantAmount:   0.0525, // (1000/1M * 15) + (500/1M * 75) = 0.015 + 0.0375
			wantAccuracy: CostMeasured,
			wantSource:   SourcePricingTable,
		},
		{
			name: "cost with cache creation tokens",
			pricing: &ModelPricing{
				Provider:                     "anthropic",
				Model:                        "claude-3-5-sonnet-20241022",
				InputPricePerMillion:         3.00,
				OutputPricePerMillion:        15.00,
				CacheCreationPricePerMillion: 3.75,
				CacheReadPricePerMillion:     0.30,
				EffectiveDate:                time.Now(),
			},
			usage: TokenUsage{
				PromptTokens:        1000,
				CompletionTokens:    500,
				CacheCreationTokens: 2000,
			},
			wantAmount:   0.0180, // (1000/1M * 3) + (500/1M * 15) + (2000/1M * 3.75) = 0.003 + 0.0075 + 0.0075
			wantAccuracy: CostMeasured,
			wantSource:   SourcePricingTable,
		},
		{
			name: "cost with cache read tokens",
			pricing: &ModelPricing{
				Provider:                     "anthropic",
				Model:                        "claude-3-5-sonnet-20241022",
				InputPricePerMillion:         3.00,
				OutputPricePerMillion:        15.00,
				CacheCreationPricePerMillion: 3.75,
				CacheReadPricePerMillion:     0.30,
				EffectiveDate:                time.Now(),
			},
			usage: TokenUsage{
				PromptTokens:     1000,
				CompletionTokens: 500,
				CacheReadTokens:  5000,
			},
			wantAmount:   0.0120, // (1000/1M * 3) + (500/1M * 15) + (5000/1M * 0.30) = 0.003 + 0.0075 + 0.0015
			wantAccuracy: CostMeasured,
			wantSource:   SourcePricingTable,
		},
		{
			name: "cost with all token types",
			pricing: &ModelPricing{
				Provider:                     "anthropic",
				Model:                        "claude-3-5-sonnet-20241022",
				InputPricePerMillion:         3.00,
				OutputPricePerMillion:        15.00,
				CacheCreationPricePerMillion: 3.75,
				CacheReadPricePerMillion:     0.30,
				EffectiveDate:                time.Now(),
			},
			usage: TokenUsage{
				PromptTokens:        1000,
				CompletionTokens:    500,
				CacheCreationTokens: 2000,
				CacheReadTokens:     3000,
			},
			wantAmount:   0.0189, // (1000/1M * 3) + (500/1M * 15) + (2000/1M * 3.75) + (3000/1M * 0.30) = 0.003 + 0.0075 + 0.0075 + 0.0009
			wantAccuracy: CostMeasured,
			wantSource:   SourcePricingTable,
		},
		{
			name:         "nil pricing",
			pricing:      nil,
			usage:        TokenUsage{PromptTokens: 1000},
			wantAmount:   0,
			wantAccuracy: CostUnavailable,
			wantSource:   SourcePricingTable,
		},
		{
			name: "subscription model",
			pricing: &ModelPricing{
				Provider:       "ollama",
				Model:          "llama2",
				IsSubscription: true,
				EffectiveDate:  time.Now(),
			},
			usage:        TokenUsage{PromptTokens: 1000, CompletionTokens: 500},
			wantAmount:   0,
			wantAccuracy: CostMeasured,
			wantSource:   SourcePricingTable,
		},
		{
			name: "estimated tokens",
			pricing: &ModelPricing{
				Provider:              "openai",
				Model:                 "gpt-4o",
				InputPricePerMillion:  2.50,
				OutputPricePerMillion: 10.00,
				EffectiveDate:         time.Now(),
			},
			usage: TokenUsage{
				TotalTokens: 1500,
			},
			wantAmount:   0, // No prompt/completion breakdown, can't calculate accurately
			wantAccuracy: CostEstimated,
			wantSource:   SourcePricingTable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateCost(tt.pricing, tt.usage)

			// Use tolerance for float comparison
			const tolerance = 0.0000001
			diff := result.Amount - tt.wantAmount
			if diff < -tolerance || diff > tolerance {
				t.Errorf("Amount = %.6f, want %.6f (diff: %.9f)", result.Amount, tt.wantAmount, diff)
			}
			if result.Accuracy != tt.wantAccuracy {
				t.Errorf("Accuracy = %v, want %v", result.Accuracy, tt.wantAccuracy)
			}
			if result.Source != tt.wantSource {
				t.Errorf("Source = %v, want %v", result.Source, tt.wantSource)
			}
			if result.Currency != "USD" {
				t.Errorf("Currency = %v, want USD", result.Currency)
			}
		})
	}
}

func TestDetermineAccuracy(t *testing.T) {
	tests := []struct {
		name  string
		usage TokenUsage
		want  CostAccuracy
	}{
		{
			name:  "provider tokens - measured",
			usage: TokenUsage{PromptTokens: 100, CompletionTokens: 50},
			want:  CostMeasured,
		},
		{
			name:  "only prompt tokens - measured",
			usage: TokenUsage{PromptTokens: 100},
			want:  CostMeasured,
		},
		{
			name:  "only completion tokens - measured",
			usage: TokenUsage{CompletionTokens: 50},
			want:  CostMeasured,
		},
		{
			name:  "only total tokens - estimated",
			usage: TokenUsage{TotalTokens: 150},
			want:  CostEstimated,
		},
		{
			name:  "no tokens - unavailable",
			usage: TokenUsage{},
			want:  CostUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineAccuracy(tt.usage)
			if got != tt.want {
				t.Errorf("determineAccuracy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEstimateTokensFromText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantMin  int
		wantMax  int
	}{
		{
			name:    "empty text",
			text:    "",
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "single word",
			text:    "hello",
			wantMin: 1,
			wantMax: 2,
		},
		{
			name:    "short sentence",
			text:    "Hello, how are you?",
			wantMin: 4,
			wantMax: 6,
		},
		{
			name:    "longer text",
			text:    "This is a longer piece of text that should result in a reasonable token estimate.",
			wantMin: 15,
			wantMax: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokensFromText(tt.text)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("EstimateTokensFromText() = %d, want between %d and %d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestEstimateTokensFromMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		wantMin  int
		wantMax  int
	}{
		{
			name:     "empty messages",
			messages: []Message{},
			wantMin:  3, // Base overhead
			wantMax:  3,
		},
		{
			name: "single message",
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			wantMin: 5,
			wantMax: 10,
		},
		{
			name: "multiple messages",
			messages: []Message{
				{Role: "user", Content: "What is the weather like?"},
				{Role: "assistant", Content: "I don't have access to real-time weather data."},
			},
			wantMin: 15,
			wantMax: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokensFromMessages(tt.messages)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("EstimateTokensFromMessages() = %d, want between %d and %d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		name string
		cost *CostInfo
		want string
	}{
		{
			name: "measured cost",
			cost: &CostInfo{
				Amount:   0.0525,
				Currency: "USD",
				Accuracy: CostMeasured,
			},
			want: "$0.0525",
		},
		{
			name: "estimated cost",
			cost: &CostInfo{
				Amount:   0.0525,
				Currency: "USD",
				Accuracy: CostEstimated,
			},
			want: "~$0.0525",
		},
		{
			name: "unavailable cost",
			cost: &CostInfo{
				Amount:   0,
				Currency: "USD",
				Accuracy: CostUnavailable,
			},
			want: "--",
		},
		{
			name: "nil cost",
			cost: nil,
			want: "--",
		},
		{
			name: "very small cost",
			cost: &CostInfo{
				Amount:   0.0001,
				Currency: "USD",
				Accuracy: CostMeasured,
			},
			want: "$0.0001",
		},
		{
			name: "large cost",
			cost: &CostInfo{
				Amount:   1.2345,
				Currency: "USD",
				Accuracy: CostMeasured,
			},
			want: "$1.2345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCost(tt.cost)
			if got != tt.want {
				t.Errorf("FormatCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		name   string
		tokens int
		want   string
	}{
		{
			name:   "small number",
			tokens: 42,
			want:   "42",
		},
		{
			name:   "thousands",
			tokens: 1500,
			want:   "1.5K",
		},
		{
			name:   "exact thousand",
			tokens: 5000,
			want:   "5.0K",
		},
		{
			name:   "millions",
			tokens: 1500000,
			want:   "1.5M",
		},
		{
			name:   "exact million",
			tokens: 2000000,
			want:   "2.0M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTokens(tt.tokens)
			if got != tt.want {
				t.Errorf("FormatTokens() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseModel(t *testing.T) {
	tests := []struct {
		name         string
		modelStr     string
		wantProvider string
		wantModel    string
	}{
		{
			name:         "explicit provider:model",
			modelStr:     "anthropic:claude-3-opus-20240229",
			wantProvider: "anthropic",
			wantModel:    "claude-3-opus-20240229",
		},
		{
			name:         "claude model inference",
			modelStr:     "claude-3-5-sonnet-20241022",
			wantProvider: "anthropic",
			wantModel:    "claude-3-5-sonnet-20241022",
		},
		{
			name:         "gpt model inference",
			modelStr:     "gpt-4o",
			wantProvider: "openai",
			wantModel:    "gpt-4o",
		},
		{
			name:         "o1 model inference",
			modelStr:     "o1-preview",
			wantProvider: "openai",
			wantModel:    "o1-preview",
		},
		{
			name:         "unknown model",
			modelStr:     "unknown-model-123",
			wantProvider: "unknown",
			wantModel:    "unknown-model-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProvider, gotModel := ParseModel(tt.modelStr)
			if gotProvider != tt.wantProvider {
				t.Errorf("ParseModel() provider = %v, want %v", gotProvider, tt.wantProvider)
			}
			if gotModel != tt.wantModel {
				t.Errorf("ParseModel() model = %v, want %v", gotModel, tt.wantModel)
			}
		})
	}
}
