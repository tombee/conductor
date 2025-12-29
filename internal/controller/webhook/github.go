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

// GitHubHandler handles GitHub webhooks.
type GitHubHandler struct{}

// Verify verifies the GitHub webhook signature.
func (h *GitHubHandler) Verify(r *http.Request, body []byte, secret string) error {
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		// Try legacy SHA-1 signature
		signature = r.Header.Get("X-Hub-Signature")
		if signature == "" {
			return fmt.Errorf("missing signature header")
		}
		// For now, reject SHA-1 signatures
		return fmt.Errorf("SHA-1 signatures not supported, please use SHA-256")
	}

	// Remove "sha256=" prefix
	if !strings.HasPrefix(signature, "sha256=") {
		return fmt.Errorf("invalid signature format")
	}
	signature = strings.TrimPrefix(signature, "sha256=")

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// ParseEvent parses the GitHub event type from the request.
func (h *GitHubHandler) ParseEvent(r *http.Request) string {
	return r.Header.Get("X-GitHub-Event")
}

// ExtractPayload extracts the payload from a GitHub webhook.
func (h *GitHubHandler) ExtractPayload(body []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return payload, nil
}

// GitHubEvent contains common GitHub webhook event fields.
type GitHubEvent struct {
	Action     string            `json:"action"`
	Sender     GitHubUser        `json:"sender"`
	Repository GitHubRepository  `json:"repository"`
}

// GitHubUser represents a GitHub user.
type GitHubUser struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubRepository represents a GitHub repository.
type GitHubRepository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	HTMLURL  string `json:"html_url"`
	CloneURL string `json:"clone_url"`
}

// Common GitHub event types
const (
	GitHubEventPush              = "push"
	GitHubEventPullRequest       = "pull_request"
	GitHubEventPullRequestReview = "pull_request_review"
	GitHubEventIssues            = "issues"
	GitHubEventIssueComment      = "issue_comment"
	GitHubEventCreate            = "create"
	GitHubEventDelete            = "delete"
	GitHubEventRelease           = "release"
	GitHubEventWorkflowRun       = "workflow_run"
)

// Common GitHub actions
const (
	GitHubActionOpened      = "opened"
	GitHubActionClosed      = "closed"
	GitHubActionReopened    = "reopened"
	GitHubActionSynchronize = "synchronize"
	GitHubActionCreated     = "created"
	GitHubActionDeleted     = "deleted"
	GitHubActionSubmitted   = "submitted"
)
