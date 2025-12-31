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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverModels(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantModels []string
		wantErr    bool
	}{
		{
			name: "successful discovery",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"object":"list","data":[{"id":"model1","object":"model","created":1234567890,"owned_by":"org"},{"id":"model2","object":"model","created":1234567891,"owned_by":"org"}]}`))
			},
			wantModels: []string{"model1", "model2"},
			wantErr:    false,
		},
		{
			name: "unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantModels: nil,
			wantErr:    true,
		},
		{
			name: "empty model list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"object":"list","data":[]}`))
			},
			wantModels: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			ctx := context.Background()
			models, err := DiscoverModels(ctx, server.URL, "test-api-key")

			if (err != nil) != tt.wantErr {
				t.Errorf("DiscoverModels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(models) != len(tt.wantModels) {
					t.Errorf("DiscoverModels() got %d models, want %d", len(models), len(tt.wantModels))
					return
				}
				for i, model := range models {
					if model != tt.wantModels[i] {
						t.Errorf("DiscoverModels() model[%d] = %v, want %v", i, model, tt.wantModels[i])
					}
				}
			}
		})
	}
}

func TestSuggestTierMappings(t *testing.T) {
	tests := []struct {
		name       string
		models     []string
		wantFast   string
		wantTier2  string
		wantTier3  string
	}{
		{
			name:       "claude models",
			models:     []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307"},
			wantFast:   "claude-3-haiku-20240307",
			wantTier2:  "claude-3-sonnet-20240229",
			wantTier3:  "claude-3-opus-20240229",
		},
		{
			name:       "empty models",
			models:     []string{},
			wantFast:   "",
			wantTier2:  "",
			wantTier3:  "",
		},
		{
			name:       "single model",
			models:     []string{"model-1"},
			wantFast:   "model-1",
			wantTier2:  "model-1",
			wantTier3:  "model-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping := SuggestTierMappings(tt.models)

			if mapping.Fast != tt.wantFast {
				t.Errorf("SuggestTierMappings().Fast = %v, want %v", mapping.Fast, tt.wantFast)
			}
			if mapping.Balanced != tt.wantTier2 {
				t.Errorf("SuggestTierMappings().Balanced = %v, want %v", mapping.Balanced, tt.wantTier2)
			}
			if mapping.Strategic != tt.wantTier3 {
				t.Errorf("SuggestTierMappings().Strategic = %v, want %v", mapping.Strategic, tt.wantTier3)
			}
		})
	}
}

func TestValidateModelMapping(t *testing.T) {
	tests := []struct {
		name    string
		mapping ModelMapping
		wantErr bool
	}{
		{
			name: "valid mapping",
			mapping: ModelMapping{
				Fast:       "model-fast",
				Balanced:   "model-balanced",
				Strategic:  "model-strategic",
			},
			wantErr: false,
		},
		{
			name: "missing fast",
			mapping: ModelMapping{
				Fast:       "",
				Balanced:   "model-balanced",
				Strategic:  "model-strategic",
			},
			wantErr: true,
		},
		{
			name: "missing balanced",
			mapping: ModelMapping{
				Fast:       "model-fast",
				Balanced:   "",
				Strategic:  "model-strategic",
			},
			wantErr: true,
		},
		{
			name: "missing strategic",
			mapping: ModelMapping{
				Fast:       "model-fast",
				Balanced:   "model-balanced",
				Strategic:  "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateModelMapping(tt.mapping)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateModelMapping() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
