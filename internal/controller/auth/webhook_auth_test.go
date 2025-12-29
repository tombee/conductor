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

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookAuthenticator_VerifyGitHub(t *testing.T) {
	auth := NewWebhookAuthenticator()
	secret := "test-secret"
	body := []byte(`{"action":"opened"}`)

	// Compute valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		signature string
		wantErr   bool
	}{
		{
			name:      "valid signature",
			signature: validSig,
			wantErr:   false,
		},
		{
			name:      "invalid signature",
			signature: "sha256=invalid",
			wantErr:   true,
		},
		{
			name:      "missing signature",
			signature: "",
			wantErr:   true,
		},
		{
			name:      "wrong prefix",
			signature: "sha1=abc123",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(string(body)))
			if tt.signature != "" {
				req.Header.Set("X-Hub-Signature-256", tt.signature)
			}

			err := auth.VerifyGitHub(req, body, secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyGitHub() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWebhookAuthenticator_VerifyGeneric(t *testing.T) {
	auth := NewWebhookAuthenticator()
	secret := "test-secret"
	body := []byte(`{"event":"test"}`)

	// Compute valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	validSig := hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		signature string
		wantErr   bool
	}{
		{
			name:      "valid signature with prefix",
			signature: "sha256=" + validSig,
			wantErr:   false,
		},
		{
			name:      "valid signature without prefix",
			signature: validSig,
			wantErr:   false,
		},
		{
			name:      "invalid signature",
			signature: "invalid",
			wantErr:   true,
		},
		{
			name:      "missing signature",
			signature: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(string(body)))
			if tt.signature != "" {
				req.Header.Set("X-Webhook-Signature", tt.signature)
			}

			err := auth.VerifyGeneric(req, body, secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyGeneric() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWebhookAuthenticator_VerifyWebhook(t *testing.T) {
	auth := NewWebhookAuthenticator()
	secret := "test-secret"
	body := []byte(`{"test":"data"}`)

	// Generic signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	genericSig := hex.EncodeToString(mac.Sum(nil))

	// GitHub signature
	mac = hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	githubSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name       string
		source     string
		headerName string
		signature  string
		wantErr    bool
	}{
		{
			name:       "github source",
			source:     "github",
			headerName: "X-Hub-Signature-256",
			signature:  githubSig,
			wantErr:    false,
		},
		{
			name:       "generic source",
			source:     "generic",
			headerName: "X-Webhook-Signature",
			signature:  genericSig,
			wantErr:    false,
		},
		{
			name:       "empty source defaults to generic",
			source:     "",
			headerName: "X-Webhook-Signature",
			signature:  genericSig,
			wantErr:    false,
		},
		{
			name:    "unsupported source",
			source:  "unsupported",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(string(body)))
			if tt.headerName != "" && tt.signature != "" {
				req.Header.Set(tt.headerName, tt.signature)
			}

			err := auth.VerifyWebhook(tt.source, req, body, secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyWebhook() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
