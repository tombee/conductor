package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/transport"
)

// indexDocument indexes a single document in Elasticsearch.
func (e *ElasticsearchIntegration) indexDocument(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameter: document
	document, ok := inputs["document"].(map[string]interface{})
	if !ok || len(document) == 0 {
		return nil, fmt.Errorf("missing required parameter: document")
	}

	// Get index name (required)
	indexName, ok := inputs["index"].(string)
	if !ok || indexName == "" {
		return nil, fmt.Errorf("missing required parameter: index")
	}

	// Get document ID (optional - Elasticsearch auto-generates if omitted)
	docID, _ := inputs["id"].(string)

	// Build URL path
	var path string
	if docID != "" {
		// PUT or POST with explicit ID
		path = fmt.Sprintf("/%s/_doc/%s", url.PathEscape(indexName), url.PathEscape(docID))
	} else {
		// POST without ID - Elasticsearch auto-generates
		path = fmt.Sprintf("/%s/_doc", url.PathEscape(indexName))
	}

	// Add query parameters
	queryParams := url.Values{}

	// Add refresh policy if specified
	if refresh, ok := inputs["refresh"].(string); ok && refresh != "" {
		// Validate refresh enum
		validRefresh := map[string]bool{
			"true": true, "false": true, "wait_for": true,
		}
		if !validRefresh[refresh] {
			return nil, fmt.Errorf("invalid refresh policy: %s (must be one of: true, false, wait_for)", refresh)
		}
		queryParams.Set("refresh", refresh)
	}

	// Add pipeline if specified
	if pipeline, ok := inputs["pipeline"].(string); ok && pipeline != "" {
		queryParams.Set("pipeline", pipeline)
	}

	// Add routing if specified
	if routing, ok := inputs["routing"].(string); ok && routing != "" {
		queryParams.Set("routing", routing)
	}

	// Build full URL
	fullURL := e.baseURL + path
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	// Marshal document
	bodyBytes, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	// Build request
	req := &transport.Request{
		Method:  "POST",
		URL:     fullURL,
		Headers: e.defaultHeaders(),
		Body:    bodyBytes,
	}

	// Execute request
	resp, err := e.transport.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse response
	var response map[string]interface{}
	if len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
	} else {
		response = map[string]interface{}{
			"status": resp.StatusCode,
		}
	}

	// Extract response fields as per spec: {_index, _id, _version, result}
	transformedResponse := make(map[string]interface{})
	if index, ok := response["_index"]; ok {
		transformedResponse["_index"] = index
	}
	if id, ok := response["_id"]; ok {
		transformedResponse["_id"] = id
	}
	if version, ok := response["_version"]; ok {
		transformedResponse["_version"] = version
	}
	if result, ok := response["result"]; ok {
		transformedResponse["result"] = result
	}

	return &operation.Result{
		Response:    transformedResponse,
		RawResponse: resp.Body,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata:    resp.Metadata,
	}, nil
}
