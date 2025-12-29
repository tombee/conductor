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

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GenericHandler handles generic webhooks.
type GenericHandler struct{}

// Verify verifies a generic webhook signature.
// Supports multiple signature header formats:
// - X-Webhook-Signature: sha256=<hex>
// - X-Signature: <hex>
// - Authorization: Bearer <token>
func (h *GenericHandler) Verify(r *http.Request, body []byte, secret string) error {
	// Try X-Webhook-Signature header
	sig := r.Header.Get("X-Webhook-Signature")
	if sig != "" {
		return h.verifyHMAC(sig, body, secret)
	}

	// Try X-Signature header
	sig = r.Header.Get("X-Signature")
	if sig != "" {
		return h.verifyHMAC("sha256="+sig, body, secret)
	}

	// Try Authorization header as bearer token
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == secret {
			return nil
		}
		return fmt.Errorf("invalid token")
	}

	// No signature found
	return fmt.Errorf("no signature header found")
}

// verifyHMAC verifies an HMAC signature.
func (h *GenericHandler) verifyHMAC(signature string, body []byte, secret string) error {
	// Remove algorithm prefix if present
	parts := strings.SplitN(signature, "=", 2)
	var algo, sig string
	if len(parts) == 2 {
		algo = parts[0]
		sig = parts[1]
	} else {
		algo = "sha256"
		sig = signature
	}

	// Only support SHA-256
	if algo != "sha256" {
		return fmt.Errorf("unsupported algorithm: %s", algo)
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// ParseEvent parses the event type from a generic webhook.
// Tries various headers:
// - X-Event-Type
// - X-Webhook-Event
// - X-Event
func (h *GenericHandler) ParseEvent(r *http.Request) string {
	headers := []string{
		"X-Event-Type",
		"X-Webhook-Event",
		"X-Event",
	}

	for _, header := range headers {
		if value := r.Header.Get(header); value != "" {
			return value
		}
	}

	// Default event
	return "webhook"
}

// ExtractPayload extracts the payload from a generic webhook.
func (h *GenericHandler) ExtractPayload(body []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return payload, nil
}
