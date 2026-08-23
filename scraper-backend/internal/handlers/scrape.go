package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"scraper-backend/internal/brightdata"
	"scraper-backend/internal/config"
	"scraper-backend/internal/csvwriter"
	"scraper-backend/internal/models"
	"scraper-backend/internal/service"
)

// ScrapeHandler handles scrape requests from clients and delegates to the Bright Data integration.
type ScrapeHandler struct {
	Cfg          *config.Config
	Client       *brightdata.BrightDataClient
	Logger       *log.Logger
	CSVWriter    *csvwriter.CSVWriter
	Persistence  *service.PersistenceService
	Refresh      *service.RefreshService
	RequestLimit int64
}

func NewScrapeHandler(cfg *config.Config, client *brightdata.BrightDataClient, logger *log.Logger, persistence ...*service.PersistenceService) *ScrapeHandler {
	var persistenceService *service.PersistenceService
	if len(persistence) > 0 {
		persistenceService = persistence[0]
	}
	return &ScrapeHandler{
		Cfg:          cfg,
		Client:       client,
		Logger:       logger,
		CSVWriter:    csvwriter.New(cfg.OutputDir),
		Persistence:  persistenceService,
		Refresh:      nil,
		RequestLimit: 1 << 20,
	}
}

func (h *ScrapeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.ContentLength > h.RequestLimit {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	var req models.ScrapeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, h.RequestLimit)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	query := req.Query
	if req.Item != "" {
		dimensions, err := models.NormalizeSearchQuery(models.SearchQuery{Brand: req.Brand, Item: req.Item, Gender: req.Gender})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		query = dimensions.ScraperQuery()
	}
	if query == "" {
		query = req.URL
	}
	if query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}
	if len(query) > 500 {
		http.Error(w, "query is too long", http.StatusBadRequest)
		return
	}

	if h.Refresh != nil && req.Item != "" {
		search, results, stale, err := h.Refresh.Search(r.Context(), models.SearchQuery{Brand: req.Brand, Item: req.Item, Gender: req.Gender})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load search: "+err.Error())
			return
		}
		csvPath, err := h.CSVWriter.Write(results)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate CSV: "+err.Error())
			return
		}
		if h.Logger != nil {
			h.Logger.Printf("products returned from PostgreSQL search_id=%d stale=%t count=%d", search.ID, stale, len(results))
		}
		response := models.ScrapeResponse{Success: true, Records: len(results), CSVFile: filepath.ToSlash(csvPath), Data: results}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	if h.Logger != nil {
		h.Logger.Printf("scrape request received for query=%s", query)
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.Cfg.PollTimeout+10*time.Second)
	defer cancel()

	collectionID, err := h.Client.TriggerCollection(ctx, query)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to trigger Bright Data collection: "+err.Error())
		return
	}
	if h.Logger != nil {
		h.Logger.Printf("collector triggered; collection ID received=%s", collectionID)
	}

	status, err := h.Client.PollForCompletion(ctx, collectionID, h.Cfg.PollInterval, h.Cfg.PollTimeout)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed while polling Bright Data collection: "+err.Error())
		return
	}
	if h.Logger != nil {
		h.Logger.Printf("collection completed with status=%s", status)
	}

	results, err := h.Client.GetCollectionResults(ctx, collectionID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to retrieve Bright Data collection: "+err.Error())
		return
	}
	if h.Logger != nil {
		h.Logger.Printf("results retrieved: count=%d", len(results))
	}

	csvPath, err := h.CSVWriter.Write(results)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate CSV: "+err.Error())
		return
	}
	if h.Logger != nil {
		h.Logger.Printf("CSV generated: %s", csvPath)
	}

	completedAt := time.Now()
	if h.Persistence != nil && req.Item != "" {
		err := h.Persistence.Persist(ctx, models.SearchQuery{Brand: req.Brand, Item: req.Item, Gender: req.Gender}, results, completedAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to persist scrape results: "+err.Error())
			return
		}
	}

	response := models.ScrapeResponse{
		Success:      true,
		CollectionID: collectionID,
		Timestamp:    completedAt.Format(time.RFC3339),
		Records:      len(results),
		CSVFile:      filepath.ToSlash(csvPath),
		Data:         results,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		if h.Logger != nil {
			h.Logger.Printf("write response: %v", err)
		}
	}
	if h.Logger != nil {
		h.Logger.Printf("scrape completed for collection %s", collectionID)
	}
}

func validateURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme == "" || u.Host == "" {
		return errors.New("missing scheme or host")
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(models.ErrorResponse{Error: message}); err != nil {
		return
	}
}
