package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"scraper-backend/internal/models"
	"scraper-backend/internal/repository"
	"scraper-backend/internal/service"
)

type APIHandler struct {
	Repository     APIRepository
	Refresh        *service.RefreshService
	AllowedOrigins string
}

type APIRepository interface {
	ListSearches(context.Context) ([]models.Search, error)
	GetSearchByID(context.Context, int64) (*models.Search, error)
	GetProductsPage(context.Context, int64, int, int, string, string) (repository.ProductPage, error)
	Ping(context.Context) error
}

func NewAPIHandler(repo APIRepository, refresh *service.RefreshService, allowedOrigins string) *APIHandler {
	return &APIHandler{Repository: repo, Refresh: refresh, AllowedOrigins: allowedOrigins}
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.setCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch r.URL.Path {
	case "/api/v1/searches":
		h.listSearches(w, r)
	case "/api/v1/products":
		h.listProducts(w, r)
	case "/api/v1/products/refresh":
		h.refreshProducts(w, r)
	case "/health":
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	case "/ready":
		h.ready(w, r)
	default:
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
	}
}

func (h *APIHandler) listSearches(w http.ResponseWriter, r *http.Request) {
	if h.Repository == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database is not configured")
		return
	}
	searches, err := h.Repository.ListSearches(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load searches")
		return
	}
	result := make([]models.SearchResponse, 0, len(searches))
	for _, search := range searches {
		result = append(result, models.SearchResponse{Brand: search.Brand, Item: search.Item, Gender: search.Gender})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) listProducts(w http.ResponseWriter, r *http.Request) {
	if h.Repository == nil || h.Refresh == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database is not configured")
		return
	}
	query, err := queryFromValues(r.URL.Query())
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_SEARCH", err.Error())
		return
	}
	search, _, stale, err := h.Refresh.SearchExisting(r.Context(), query)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "SEARCH_NOT_FOUND", "Unsupported search combination")
			return
		}
		writeAPIError(w, http.StatusServiceUnavailable, "REFRESH_UNAVAILABLE", "Search data is not available")
		return
	}
	search, err = h.Repository.GetSearchByID(r.Context(), search.ID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load search metadata")
		return
	}
	page, pageSize, err := pagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PAGINATION", err.Error())
		return
	}
	retailer := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("retailer")))
	if retailer != "" && retailer != "amazon" && retailer != "myntra" && retailer != "ajio" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_RETAILER", "Unsupported retailer")
		return
	}
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "price_asc"
	}
	if sortBy != "price_asc" && sortBy != "price_desc" && sortBy != "rating_desc" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_SORT", "Unsupported sort order")
		return
	}
	productPage, err := h.Repository.GetProductsPage(r.Context(), search.ID, page, pageSize, sortBy, retailer)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load products")
		return
	}
	totalPages := (productPage.Total + pageSize - 1) / pageSize
	writeJSON(w, http.StatusOK, map[string]any{
		"search":   models.SearchResponse{Brand: search.Brand, Item: search.Item, Gender: search.Gender},
		"products": productPage.Products,
		"metadata": map[string]any{"total": productPage.Total, "lastScrapedAt": search.LastScrapedAt, "expiresAt": search.ExpiresAt, "stale": stale, "refreshInProgress": search.RefreshInProgress, "page": page, "pageSize": pageSize, "totalPages": totalPages},
	})
}

func (h *APIHandler) refreshProducts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST is required")
		return
	}
	if h.Refresh == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database is not configured")
		return
	}
	var request models.SearchQuery
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON body")
		return
	}
	started, _, err := h.Refresh.StartRefresh(r.Context(), request)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_SEARCH", err.Error())
		return
	}
	if started {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "refresh_started"})
	} else {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "refresh_already_in_progress"})
	}
}

func (h *APIHandler) ready(w http.ResponseWriter, r *http.Request) {
	if h.Repository == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database is not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.Repository.Ping(ctx); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func queryFromValues(values url.Values) (models.SearchQuery, error) {
	return models.NormalizeSearchQuery(models.SearchQuery{Brand: values.Get("brand"), Item: values.Get("item"), Gender: values.Get("gender")})
}

func pagination(r *http.Request) (int, int, error) {
	page, pageSize := 1, 20
	var err error
	if value := r.URL.Query().Get("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			return 0, 0, fmt.Errorf("page must be at least 1")
		}
	}
	if value := r.URL.Query().Get("page_size"); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > 100 {
			return 0, 0, fmt.Errorf("page_size must be between 1 and 100")
		}
	}
	return page, pageSize, nil
}

func (h *APIHandler) setCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	for _, allowed := range strings.Split(h.AllowedOrigins, ",") {
		if strings.TrimSpace(allowed) == origin && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
