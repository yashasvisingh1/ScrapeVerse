package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"scraper-backend/internal/brightdata"
	"scraper-backend/internal/config"
	"scraper-backend/internal/handlers"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	client := brightdata.NewClient(cfg.BrightDataBaseURL, cfg.BrightDataAPIToken, cfg.BrightDataCollectorID, &http.Client{Timeout: 30 * time.Second})
	logger := log.New(os.Stdout, "scraper-backend ", log.LstdFlags|log.LUTC)
	handler := handlers.NewScrapeHandler(cfg, client, logger)

	logger.Printf("server started on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		logger.Fatalf("http server failed: %v", err)
	}
}
