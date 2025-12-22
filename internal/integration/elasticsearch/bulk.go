package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/transport"
)

// bulkIndex performs bulk indexing of multiple documents.
func (e *ElasticsearchIntegration) bulkIndex(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameter: documents
	documentsInput, ok := inputs["documents"].([]interface{})
	if !ok || len(documentsInput) == 0 {
		return nil, fmt.Errorf("missing required parameter: documents")
	}

	// Get default index name (optional)
	defaultIndex, _ := inputs["index"].(string)

	// Build NDJSON payload
	// Format: {action}\n{document}\n{action}\n{document}\n...
	var buffer bytes.Buffer

	for i, docInput := range documentsInput {
		docMap, ok := docInput.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("documents[%d] must be an object", i)
		}

		// Extract index-specific fields
		indexName := defaultIndex
		if idx, ok := docMap["_index"].(string); ok && idx != "" {
			indexName = idx
		}

		var docID string
		if id, ok := docMap["_id"].(string); ok {
			docID = id
		}

		// Build action line
		action := map[string]interface{}{
			"index": map[string]interface{}{},
		}

		if indexName != "" {
			action["index"].(map[string]interface{})["_index"] = indexName
		}
		if docID != "" {
			action["index"].(map[string]interface{})["_id"] = docID
		}

		actionBytes, err := json.Marshal(action)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal action for documents[%d]: %w", i, err)
		}
		buffer.Write(actionBytes)
		buffer.WriteByte('\n')

		// Build document (exclude metadata fields)
		document := make(map[string]interface{})
		for key, value := range docMap {
			if key != "_index" && key != "_id" {
				document[key] = value
			}
		}

		docBytes, err := json.Marshal(document)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal documents[%d]: %w", i, err)
		}
		buffer.Write(docBytes)
		buffer.WriteByte('\n')
	}

	// Build URL
	path := "/_bulk"
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

	// Build full URL
	fullURL := e.baseURL + path
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	// Build headers (NDJSON content type)
	headers := e.defaultHeaders()
	headers["Content-Type"] = "application/x-ndjson"

	// Build request
	req := &transport.Request{
		Method:  "POST",
		URL:     fullURL,
		Headers: headers,
		Body:    buffer.Bytes(),
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

	// Extract response fields as per spec: {took, errors, items}
	transformedResponse := make(map[string]interface{})
	if took, ok := response["took"]; ok {
		transformedResponse["took"] = took
	}
	if errors, ok := response["errors"]; ok {
		transformedResponse["errors"] = errors
	}
	if items, ok := response["items"]; ok {
		transformedResponse["items"] = items
	}

	return &operation.Result{
		Response:    transformedResponse,
		RawResponse: resp.Body,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata:    resp.Metadata,
	}, nil
}
