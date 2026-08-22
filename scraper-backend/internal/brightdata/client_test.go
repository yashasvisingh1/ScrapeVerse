package brightdata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTriggerRequestCreation(t *testing.T) {
	var receivedAuth, receivedMethod, receivedPath string
	var receivedPayload []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.String()
		receivedAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Fatalf("decode trigger payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"abc123"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", "collector-123", server.Client())
	id, err := client.TriggerCollection(context.Background(), "tops for women")
	if err != nil {
		t.Fatalf("TriggerCollection() returned error: %v", err)
	}
	if id != "abc123" {
		t.Fatalf("TriggerCollection() returned %q, want %q", id, "abc123")
	}
	if receivedMethod != http.MethodPost {
		t.Fatalf("request method = %q, want %q", receivedMethod, http.MethodPost)
	}
	if receivedAuth != "Bearer token" {
		t.Fatalf("Authorization header = %q, want %q", receivedAuth, "Bearer token")
	}
	if receivedPath != "/dca/trigger?collector=collector-123&queue_next=1" {
		t.Fatalf("path = %q", receivedPath)
	}
	if len(receivedPayload) != 1 || receivedPayload[0]["query"] != "tops for women" {
		t.Fatalf("payload = %#v, want query input", receivedPayload)
	}
}

func TestExtractCollectionIDFromNestedResponse(t *testing.T) {
	payload := []byte(`{"meta":{"job":{"id":"nested-id"}}}`)
	id, err := extractCollectionID(payload)
	if err != nil {
		t.Fatalf("extractCollectionID() returned error: %v", err)
	}
	if id != "nested-id" {
		t.Fatalf("extractCollectionID() returned %q, want %q", id, "nested-id")
	}
}

func TestGetCollectionStatusUsesDatasetEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "job-1" {
			t.Fatalf("id query = %q, want %q", r.URL.Query().Get("id"), "job-1")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"completed"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", "collector", server.Client())
	status, err := client.GetCollectionStatus(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetCollectionStatus() returned error: %v", err)
	}
	if status != "completed" {
		t.Fatalf("status = %q, want %q", status, "completed")
	}
}

func TestGetCollectionStatusTreatsAcceptedAsPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"starting"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", "collector", server.Client())
	status, err := client.GetCollectionStatus(context.Background(), "job-pending")
	if err != nil {
		t.Fatalf("GetCollectionStatus() returned error: %v", err)
	}
	if status != "pending" {
		t.Fatalf("status = %q, want pending", status)
	}
}

func TestPollingBehaviorStopsOnCompletion(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"status":"pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"completed"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", "collector", server.Client())
	status, err := client.PollForCompletion(context.Background(), "job-2", 10*time.Millisecond, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("PollForCompletion() unexpected error: %v", err)
	}
	if status != "completed" {
		t.Fatalf("status = %q, want %q", status, "completed")
	}
	if calls < 2 {
		t.Fatalf("expected at least two polling calls, got %d", calls)
	}
}

func TestGetCollectionResultsParsesDynamicArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"Product A","price":"$20","url":"https://example.com/a"},{"name":"Product B","price":"$30","url":"https://example.com/b"}]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", "collector", server.Client())
	results, err := client.GetCollectionResults(context.Background(), "job-3")
	if err != nil {
		t.Fatalf("GetCollectionResults() unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0]["name"] != "Product A" {
		t.Fatalf("first record name = %q, want %q", results[0]["name"], "Product A")
	}
}

func TestGetCollectionResultsParsesNDJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"name\":\"Product A\"}\n{\"name\":\"Product B\"}\n"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", "collector", server.Client())
	results, err := client.GetCollectionResults(context.Background(), "job-ndjson")
	if err != nil {
		t.Fatalf("GetCollectionResults() returned error: %v", err)
	}
	if len(results) != 2 || results[1]["name"] != "Product B" {
		t.Fatalf("results = %#v, want two NDJSON records", results)
	}
}

func TestGetCollectionStatusTreatsEmptyArrayAsCompleted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", "collector", server.Client())
	status, err := client.GetCollectionStatus(context.Background(), "job-empty")
	if err != nil {
		t.Fatalf("GetCollectionStatus() returned error: %v", err)
	}
	if status != "completed" {
		t.Fatalf("status = %q, want completed", status)
	}
}

func TestGetCollectionResultsRejectsMalformedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", "collector", server.Client())
	if _, err := client.GetCollectionResults(context.Background(), "bad-job"); err == nil {
		t.Fatal("expected malformed dataset payload error")
	}
}
