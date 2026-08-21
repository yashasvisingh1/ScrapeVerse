package brightdata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func NewClient(baseURL, apiToken, collectorID string, httpClient *http.Client) *BrightDataClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &BrightDataClient{
		HTTPClient:  httpClient,
		BaseURL:     strings.TrimRight(baseURL, "/"),
		APIToken:    apiToken,
		CollectorID: collectorID,
	}
}

func (c *BrightDataClient) TriggerCollection(ctx context.Context, query string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("bright data client is nil")
	}
	if c.CollectorID == "" {
		return "", fmt.Errorf("collector ID is required")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	endpoint := fmt.Sprintf("%s/dca/trigger?collector=%s&queue_next=1", c.BaseURL, url.QueryEscape(c.CollectorID))
	payload, err := json.Marshal([]map[string]string{{"query": query}})
	if err != nil {
		return "", fmt.Errorf("encode collector input: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create trigger request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("trigger Bright Data collector: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read trigger response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("trigger request failed with status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	collectionID, err := extractCollectionID(body)
	if err != nil {
		return "", fmt.Errorf("invalid trigger response: %w", err)
	}
	return collectionID, nil
}

func (c *BrightDataClient) GetCollectionStatus(ctx context.Context, collectionID string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("bright data client is nil")
	}
	if collectionID == "" {
		return "", fmt.Errorf("collection ID is required")
	}

	endpoint := fmt.Sprintf("%s/dca/dataset?id=%s", c.BaseURL, url.QueryEscape(collectionID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create status request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch collection status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read status response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("collection status request failed with status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	status := extractStatus(body)
	if status != "" {
		return normalizeStatus(status), nil
	}

	results, err := decodeResults(body)
	if err != nil {
		return "pending", nil
	}
	if len(results) > 0 {
		return "completed", nil
	}
	return "pending", nil
}

func (c *BrightDataClient) GetCollectionResults(ctx context.Context, collectionID string) ([]map[string]any, error) {
	if c == nil {
		return nil, fmt.Errorf("bright data client is nil")
	}
	if collectionID == "" {
		return nil, fmt.Errorf("collection ID is required")
	}

	endpoint := fmt.Sprintf("%s/dca/dataset?id=%s", c.BaseURL, url.QueryEscape(collectionID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create dataset request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("retrieve Bright Data collection results: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read dataset response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("dataset request failed with status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	results, err := decodeResults(body)
	if err != nil {
		return nil, fmt.Errorf("invalid dataset response: %w", err)
	}
	return results, nil
}

func (c *BrightDataClient) PollForCompletion(ctx context.Context, collectionID string, pollInterval, pollTimeout time.Duration) (string, error) {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	if pollTimeout <= 0 {
		pollTimeout = 300 * time.Second
	}

	deadline := time.Now().Add(pollTimeout)
	for {
		if time.Now().After(deadline) || time.Now().Equal(deadline) {
			return "", fmt.Errorf("collection %s timed out after %s", collectionID, pollTimeout)
		}

		status, err := c.GetCollectionStatus(ctx, collectionID)
		if err != nil {
			return "", err
		}

		switch normalizeStatus(status) {
		case "completed", "success", "done", "succeeded", "finished", "ready":
			return status, nil
		case "failed", "error", "cancelled", "canceled", "timedout", "timeout":
			return "", fmt.Errorf("Bright Data collection failed with status %q", status)
		case "running", "pending", "queued", "waiting", "processing", "inprogress":
			// continue polling until timeout or cancellation
		default:
			if status == "" {
				return "", fmt.Errorf("Bright Data collection status was empty")
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func extractCollectionID(payload []byte) (string, error) {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return "", fmt.Errorf("decode JSON: %w", err)
	}
	id, found := findCollectionID(value)
	if !found {
		return "", fmt.Errorf("response did not contain a collection ID")
	}
	if id == "" {
		return "", fmt.Errorf("collection ID was empty")
	}
	return id, nil
}

func findCollectionID(value any) (string, bool) {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{"id", "collection_id", "collectionId", "collectionID", "job_id", "jobId", "jobID", "dataset_id", "datasetId", "datasetID", "result_id", "resultId"} {
			if item, ok := v[key]; ok {
				return stringifyValue(item), true
			}
		}
		for _, nested := range v {
			if id, ok := findCollectionID(nested); ok {
				return id, true
			}
		}
	case []any:
		for _, item := range v {
			if id, ok := findCollectionID(item); ok {
				return id, true
			}
		}
	}
	return "", false
}

func extractStatus(payload []byte) string {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return ""
	}
	status, ok := findStatus(value)
	if !ok {
		return ""
	}
	return status
}

func findStatus(value any) (string, bool) {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{"status", "state", "job_status", "jobStatus", "collection_status", "collectionStatus", "result_status", "resultStatus"} {
			if status, ok := v[key]; ok {
				result := stringifyValue(status)
				if result != "" {
					return result, true
				}
			}
		}
		for _, nested := range v {
			if status, ok := findStatus(nested); ok {
				return status, true
			}
		}
	case []any:
		for _, item := range v {
			if status, ok := findStatus(item); ok {
				return status, true
			}
		}
	}
	return "", false
}

func decodeResults(payload []byte) ([]map[string]any, error) {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, err
	}
	results, ok := findResultObjects(value)
	if !ok {
		return nil, fmt.Errorf("dataset response did not contain a result array")
	}
	return results, nil
}

func findResultObjects(value any) ([]map[string]any, bool) {
	switch v := value.(type) {
	case []any:
		results := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				results = append(results, m)
			}
		}
		if len(results) > 0 {
			return results, true
		}
	case map[string]any:
		for _, key := range []string{"data", "results", "items", "records", "output"} {
			if nested, ok := v[key]; ok {
				if arr, ok := nested.([]any); ok {
					results := make([]map[string]any, 0, len(arr))
					for _, item := range arr {
						if m, ok := item.(map[string]any); ok {
							results = append(results, m)
						}
					}
					if len(results) > 0 {
						return results, true
					}
				}
			}
		}
		for _, nested := range v {
			if results, ok := findResultObjects(nested); ok {
				return results, true
			}
		}
	}
	return nil, false
}

func normalizeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	status = strings.ReplaceAll(status, "_", "")
	status = strings.ReplaceAll(status, "-", "")
	status = strings.ReplaceAll(status, " ", "")
	return status
}

func stringifyValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64, float32, int, int64, int32:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return ""
	default:
		if out, err := json.Marshal(v); err == nil {
			return string(out)
		}
		return fmt.Sprintf("%v", v)
	}
}
