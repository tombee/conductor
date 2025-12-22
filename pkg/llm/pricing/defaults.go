package pricing

import "time"

// getBuiltInPricing returns the default pricing configuration.
// This includes current pricing for major providers as of the effective date.
func getBuiltInPricing() *PricingConfig {
	effectiveDate := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)

	return &PricingConfig{
		Version:   "1.0",
		UpdatedAt: effectiveDate,
		Models: []ModelPricing{
			// Anthropic Claude 3.5 models
			{
				Provider:                     "anthropic",
				Model:                        "claude-3-5-sonnet-20241022",
				InputPricePerMillion:         3.00,
				OutputPricePerMillion:        15.00,
				CacheCreationPricePerMillion: 3.75,
				CacheReadPricePerMillion:     0.30,
				EffectiveDate:                effectiveDate,
			},
			{
				Provider:                     "anthropic",
				Model:                        "claude-3-5-haiku-20241022",
				InputPricePerMillion:         1.00,
				OutputPricePerMillion:        5.00,
				CacheCreationPricePerMillion: 1.25,
				CacheReadPricePerMillion:     0.10,
				EffectiveDate:                effectiveDate,
			},

			// Anthropic Claude 3 Opus
			{
				Provider:                     "anthropic",
				Model:                        "claude-3-opus-20240229",
				InputPricePerMillion:         15.00,
				OutputPricePerMillion:        75.00,
				CacheCreationPricePerMillion: 18.75,
				CacheReadPricePerMillion:     1.50,
				EffectiveDate:                effectiveDate,
			},

			// Anthropic Claude 3 Sonnet
			{
				Provider:                     "anthropic",
				Model:                        "claude-3-sonnet-20240229",
				InputPricePerMillion:         3.00,
				OutputPricePerMillion:        15.00,
				CacheCreationPricePerMillion: 3.75,
				CacheReadPricePerMillion:     0.30,
				EffectiveDate:                effectiveDate,
			},

			// Anthropic Claude 3 Haiku
			{
				Provider:                     "anthropic",
				Model:                        "claude-3-haiku-20240307",
				InputPricePerMillion:         0.25,
				OutputPricePerMillion:        1.25,
				CacheCreationPricePerMillion: 0.30,
				CacheReadPricePerMillion:     0.03,
				EffectiveDate:                effectiveDate,
			},

			// OpenAI GPT-4o models
			{
				Provider:              "openai",
				Model:                 "gpt-4o",
				InputPricePerMillion:  2.50,
				OutputPricePerMillion: 10.00,
				EffectiveDate:         effectiveDate,
			},
			{
				Provider:              "openai",
				Model:                 "gpt-4o-mini",
				InputPricePerMillion:  0.15,
				OutputPricePerMillion: 0.60,
				EffectiveDate:         effectiveDate,
			},

			// OpenAI GPT-4 Turbo
			{
				Provider:              "openai",
				Model:                 "gpt-4-turbo",
				InputPricePerMillion:  10.00,
				OutputPricePerMillion: 30.00,
				EffectiveDate:         effectiveDate,
			},
			{
				Provider:              "openai",
				Model:                 "gpt-4-turbo-preview",
				InputPricePerMillion:  10.00,
				OutputPricePerMillion: 30.00,
				EffectiveDate:         effectiveDate,
			},

			// OpenAI GPT-4
			{
				Provider:              "openai",
				Model:                 "gpt-4",
				InputPricePerMillion:  30.00,
				OutputPricePerMillion: 60.00,
				EffectiveDate:         effectiveDate,
			},
			{
				Provider:              "openai",
				Model:                 "gpt-4-32k",
				InputPricePerMillion:  60.00,
				OutputPricePerMillion: 120.00,
				EffectiveDate:         effectiveDate,
			},

			// OpenAI GPT-3.5 Turbo
			{
				Provider:              "openai",
				Model:                 "gpt-3.5-turbo",
				InputPricePerMillion:  0.50,
				OutputPricePerMillion: 1.50,
				EffectiveDate:         effectiveDate,
			},
			{
				Provider:              "openai",
				Model:                 "gpt-3.5-turbo-16k",
				InputPricePerMillion:  3.00,
				OutputPricePerMillion: 4.00,
				EffectiveDate:         effectiveDate,
			},

			// OpenAI o1 models (reasoning models)
			{
				Provider:              "openai",
				Model:                 "o1-preview",
				InputPricePerMillion:  15.00,
				OutputPricePerMillion: 60.00,
				EffectiveDate:         effectiveDate,
			},
			{
				Provider:              "openai",
				Model:                 "o1-mini",
				InputPricePerMillion:  3.00,
				OutputPricePerMillion: 12.00,
				EffectiveDate:         effectiveDate,
			},

			// Ollama (local models - no cost)
			{
				Provider:       "ollama",
				Model:          "llama2",
				IsSubscription: true,
				EffectiveDate:  effectiveDate,
			},
			{
				Provider:       "ollama",
				Model:          "llama3",
				IsSubscription: true,
				EffectiveDate:  effectiveDate,
			},
			{
				Provider:       "ollama",
				Model:          "mistral",
				IsSubscription: true,
				EffectiveDate:  effectiveDate,
			},
			{
				Provider:       "ollama",
				Model:          "mixtral",
				IsSubscription: true,
				EffectiveDate:  effectiveDate,
			},
		},
	}
}
