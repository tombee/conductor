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
	"fmt"
	"net/http"
	"strings"

	"github.com/tombee/conductor/internal/controller/webhook"
)

// WebhookAuthenticator provides signature verification for webhooks.
type WebhookAuthenticator struct {
	githubHandler *webhook.GitHubHandler
	slackHandler  *webhook.SlackHandler
}

// NewWebhookAuthenticator creates a new webhook authenticator.
func NewWebhookAuthenticator() *WebhookAuthenticator {
	return &WebhookAuthenticator{
		githubHandler: &webhook.GitHubHandler{},
		slackHandler:  &webhook.SlackHandler{},
	}
}

// VerifyGitHub verifies a GitHub webhook signature.
func (a *WebhookAuthenticator) VerifyGitHub(r *http.Request, body []byte, secret string) error {
	return a.githubHandler.Verify(r, body, secret)
}

// VerifySlack verifies a Slack webhook signature.
func (a *WebhookAuthenticator) VerifySlack(r *http.Request, body []byte, secret string) error {
	return a.slackHandler.Verify(r, body, secret)
}

// VerifyGeneric verifies a generic webhook using HMAC-SHA256.
// Expects X-Webhook-Signature header with format: sha256=<hex-signature>
func (a *WebhookAuthenticator) VerifyGeneric(r *http.Request, body []byte, secret string) error {
	signature := r.Header.Get("X-Webhook-Signature")
	if signature == "" {
		return fmt.Errorf("missing X-Webhook-Signature header")
	}

	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// VerifyWebhook verifies a webhook signature based on the source type.
func (a *WebhookAuthenticator) VerifyWebhook(source string, r *http.Request, body []byte, secret string) error {
	switch source {
	case "github":
		return a.VerifyGitHub(r, body, secret)
	case "slack":
		return a.VerifySlack(r, body, secret)
	case "generic", "":
		return a.VerifyGeneric(r, body, secret)
	default:
		return fmt.Errorf("unsupported webhook source: %s", source)
	}
}
