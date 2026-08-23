package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"scraper-backend/internal/brightdata"
	"scraper-backend/internal/config"
	"scraper-backend/internal/handlers"
	"scraper-backend/internal/repository"
	"scraper-backend/internal/service"
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
	var persistence *service.PersistenceService
	var refreshService *service.RefreshService
	var dbRepo *repository.Repository
	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			cancel()
			logger.Fatalf("create database pool: %v", err)
		}
		if err := pool.Ping(ctx); err != nil {
			cancel()
			pool.Close()
			logger.Fatalf("connect to database: %v", err)
		}
		if err := repository.New(pool).ClearRefreshLocks(ctx); err != nil {
			cancel()
			pool.Close()
			logger.Fatalf("clear stale refresh locks: %v", err)
		}
		cancel()
		defer pool.Close()
		dbRepo = repository.New(pool)
		persistence = service.NewPersistenceService(dbRepo, cfg.ScrapeTTL)
		refreshService = service.NewRefreshService(dbRepo, service.BrightDataScraper{Scraper: brightdata.NewScraper(client), PollInterval: cfg.PollInterval, PollTimeout: cfg.PollTimeout}, cfg.ScrapeTTL, logger)
		worker := service.NewRefreshWorker(refreshService, cfg.ScraperWorkerInterval, logger)
		go worker.Run(context.Background())
		logger.Printf("database persistence enabled")
	} else {
		logger.Printf("database persistence disabled: DATABASE_URL is not set")
	}
	handler := handlers.NewScrapeHandler(cfg, client, logger, persistence)
	handler.Refresh = refreshService
	mux := http.NewServeMux()
	mux.Handle("/api/scrape", handler)
	apiHandler := handlers.NewAPIHandler(dbRepo, refreshService, cfg.AllowedOrigins)
	mux.Handle("/api/v1/", apiHandler)
	mux.Handle("/health", apiHandler)
	mux.Handle("/ready", apiHandler)

	logger.Printf("server started on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Fatalf("http server failed: %v", err)
	}
}
