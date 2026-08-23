package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"scraper-backend/internal/models"
	"scraper-backend/internal/repository"
	"scraper-backend/internal/service"
)

type apiFakeRepository struct {
	search   models.Search
	products repository.ProductPage
	searches []models.Search
	claim    bool
}

func (r *apiFakeRepository) GetSearch(context.Context, models.SearchQuery) (*models.Search, error) {
	if r.search.ID == 0 {
		return nil, pgx.ErrNoRows
	}
	return &r.search, nil
}
func (r *apiFakeRepository) GetSearchByID(context.Context, int64) (*models.Search, error) {
	return &r.search, nil
}
func (r *apiFakeRepository) GetOrCreateSearch(context.Context, models.SearchQuery) (*models.Search, error) {
	return &r.search, nil
}
func (r *apiFakeRepository) GetProductsWithLatestPrices(context.Context, int64) ([]map[string]any, error) {
	return nil, nil
}
func (r *apiFakeRepository) ClaimRefresh(context.Context, int64) (bool, error) {
	if r.claim {
		return false, nil
	}
	r.claim = true
	return true, nil
}
func (r *apiFakeRepository) MarkRefreshFailed(context.Context, int64) error {
	r.claim = false
	return nil
}
func (r *apiFakeRepository) ReplaceSearchProducts(context.Context, int64, []models.Product, []models.PriceObservation, time.Time, time.Duration) error {
	return nil
}
func (r *apiFakeRepository) FindSearchesNeedingRefresh(context.Context) ([]models.Search, error) {
	return nil, nil
}
func (r *apiFakeRepository) ListSearches(context.Context) ([]models.Search, error) {
	return r.searches, nil
}
func (r *apiFakeRepository) GetProductsPage(context.Context, int64, int, int, string, string) (repository.ProductPage, error) {
	return r.products, nil
}
func (r *apiFakeRepository) Ping(context.Context) error { return nil }

type apiFakeScraper struct{}

func (apiFakeScraper) Search(context.Context, string, time.Duration, time.Duration) ([]map[string]any, error) {
	return nil, errors.New("not expected")
}

func newAPIForTest(repo *apiFakeRepository) *APIHandler {
	refresh := service.NewRefreshService(repo, apiFakeScraper{}, time.Minute, nil)
	return NewAPIHandler(repo, refresh, "http://localhost:3000")
}

func TestListSearches(t *testing.T) {
	repo := &apiFakeRepository{searches: []models.Search{{Brand: "nike", Item: "shoes", Gender: "men"}}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/searches", nil)
	recorder := httptest.NewRecorder()
	newAPIForTest(repo).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d", recorder.Code)
	}
	var result []models.SearchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].Brand != "nike" {
		t.Fatalf("result=%v", result)
	}
}

func TestProductsReturnsCachedPage(t *testing.T) {
	scraped := time.Now().Add(-time.Minute)
	expires := time.Now().Add(time.Hour)
	repo := &apiFakeRepository{search: models.Search{ID: 1, Brand: "nike", Item: "shoes", Gender: "men", LastScrapedAt: &scraped, ExpiresAt: &expires}, products: repository.ProductPage{Total: 1, Products: []models.ProductResponse{{ID: 2, Retailer: "Amazon", CurrentPrice: floatPointer(2999)}}}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/products?brand=nike&item=shoes&gender=men&page=1&page_size=20", nil)
	recorder := httptest.NewRecorder()
	newAPIForTest(repo).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Products []models.ProductResponse `json:"products"`
		Metadata struct {
			Total      int  `json:"total"`
			Page       int  `json:"page"`
			TotalPages int  `json:"totalPages"`
			Stale      bool `json:"stale"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Products) != 1 || response.Metadata.Total != 1 || response.Metadata.TotalPages != 1 || response.Metadata.Stale {
		t.Fatalf("response=%+v", response)
	}
}

func TestProductsRejectsMissingItem(t *testing.T) {
	recorder := httptest.NewRecorder()
	newAPIForTest(&apiFakeRepository{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/products?brand=nike", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestProductsRejectsUnsupportedSearch(t *testing.T) {
	repo := &apiFakeRepository{}
	recorder := httptest.NewRecorder()
	newAPIForTest(repo).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/products?item=shoes", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestManualRefreshReturnsAccepted(t *testing.T) {
	repo := &apiFakeRepository{search: models.Search{ID: 1, Brand: "nike", Item: "shoes", Gender: "men"}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/products/refresh", bytes.NewBufferString(`{"brand":"nike","item":"shoes","gender":"men"}`))
	newAPIForTest(repo).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || recorder.Body.String() == "" {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func floatPointer(value float64) *float64 { return &value }
