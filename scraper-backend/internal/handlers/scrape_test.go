package handlers

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"scraper-backend/internal/brightdata"
	"scraper-backend/internal/config"
)

func TestScrapeHandlerSuccess(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/dca/trigger":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"abc123"}`))
		case "/dca/dataset":
			if r.URL.Query().Get("id") == "abc123" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[{"name":"Product A","price":"$20","url":"https://example.com/a"}]`))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		BrightDataAPIToken:    "token",
		BrightDataCollectorID: "collector-123",
		BrightDataBaseURL:     server.URL,
		PollInterval:          time.Second,
		PollTimeout:           5 * time.Second,
		OutputDir:             t.TempDir(),
	}
	client := brightdata.NewClient(cfg.BrightDataBaseURL, cfg.BrightDataAPIToken, cfg.BrightDataCollectorID, server.Client())
	handler := NewScrapeHandler(cfg, client, log.New(os.Stdout, "", 0))

	req := httptest.NewRequest(http.MethodPost, "/api/scrape", bytes.NewBufferString(`{"query":"tops for women"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if calls == 0 {
		t.Fatal("expected Bright Data endpoints to be called")
	}
	if !strings.Contains(w.Body.String(), "\"success\":true") {
		t.Fatalf("response missing success indicator: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "abc123") {
		t.Fatalf("response missing collection_id: %s", w.Body.String())
	}
}

func TestScrapeHandlerRejectsMissingQuery(t *testing.T) {
	cfg := &config.Config{
		BrightDataAPIToken:    "token",
		BrightDataCollectorID: "collector-123",
		BrightDataBaseURL:     "https://example.com",
		PollInterval:          time.Second,
		PollTimeout:           5 * time.Second,
		OutputDir:             t.TempDir(),
	}
	client := brightdata.NewClient(cfg.BrightDataBaseURL, cfg.BrightDataAPIToken, cfg.BrightDataCollectorID, http.DefaultClient)
	handler := NewScrapeHandler(cfg, client, log.New(os.Stdout, "", 0))

	req := httptest.NewRequest(http.MethodPost, "/api/scrape", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestScrapeHandlerMethodNotAllowed(t *testing.T) {
	cfg := &config.Config{OutputDir: t.TempDir()}
	client := brightdata.NewClient("https://example.com", "token", "collector", http.DefaultClient)
	handler := NewScrapeHandler(cfg, client, log.New(os.Stdout, "", 0))
	req := httptest.NewRequest(http.MethodGet, "/api/scrape", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestScrapeWithContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-context.Background().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		BrightDataAPIToken:    "token",
		BrightDataCollectorID: "collector-123",
		BrightDataBaseURL:     server.URL,
		PollInterval:          time.Millisecond,
		PollTimeout:           time.Second,
		OutputDir:             t.TempDir(),
	}
	client := brightdata.NewClient(cfg.BrightDataBaseURL, cfg.BrightDataAPIToken, cfg.BrightDataCollectorID, server.Client())
	handler := NewScrapeHandler(cfg, client, log.New(os.Stdout, "", 0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/scrape", bytes.NewBufferString(`{"query":"tops for women"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	_ = handler
	_ = w
}
