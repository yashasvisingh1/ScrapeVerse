package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"scraper-backend/internal/models"
)

type fakeRepository struct {
	mu         sync.Mutex
	search     models.Search
	products   []map[string]any
	claim      bool
	claimCalls int
	failed     int
	stored     bool
	expiresAt  time.Time
	needed     []models.Search
	claimSeen  chan struct{}
}

func (r *fakeRepository) GetSearchByID(context.Context, int64) (*models.Search, error) {
	return &r.search, nil
}
func (r *fakeRepository) GetOrCreateSearch(context.Context, models.SearchQuery) (*models.Search, error) {
	return &r.search, nil
}
func (r *fakeRepository) GetProductsWithLatestPrices(context.Context, int64) ([]map[string]any, error) {
	return r.products, nil
}
func (r *fakeRepository) ClaimRefresh(context.Context, int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimCalls++
	if r.claimSeen != nil {
		r.claimSeen <- struct{}{}
	}
	if r.claim {
		return false, nil
	}
	r.claim = true
	return true, nil
}
func (r *fakeRepository) MarkRefreshFailed(context.Context, int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed++
	r.claim = false
	return nil
}
func (r *fakeRepository) ReplaceSearchProducts(_ context.Context, _ int64, _ []models.Product, _ []models.PriceObservation, scrapedAt time.Time, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stored = true
	r.expiresAt = scrapedAt.Add(ttl)
	r.claim = false
	r.search.LastScrapedAt = &scrapedAt
	r.search.ExpiresAt = &r.expiresAt
	return nil
}
func (r *fakeRepository) FindSearchesNeedingRefresh(context.Context) ([]models.Search, error) {
	return r.needed, nil
}

type fakeScraper struct {
	mu      sync.Mutex
	calls   int
	err     error
	records []map[string]any
	started chan struct{}
	wait    chan struct{}
}

func (s *fakeScraper) Search(context.Context, string, time.Duration, time.Duration) ([]map[string]any, error) {
	s.mu.Lock()
	s.calls++
	if s.started != nil {
		close(s.started)
		s.started = nil
	}
	s.mu.Unlock()
	if s.wait != nil {
		<-s.wait
	}
	return s.records, s.err
}

func (s *fakeScraper) Calls() int { s.mu.Lock(); defer s.mu.Unlock(); return s.calls }

func TestFreshSearchDoesNotRefresh(t *testing.T) {
	scraped := time.Now().Add(-time.Minute)
	expires := time.Now().Add(time.Hour)
	repo := &fakeRepository{search: models.Search{ID: 1, Item: "shoes", LastScrapedAt: &scraped, ExpiresAt: &expires}, products: []map[string]any{{"title": "cached"}}}
	scraper := &fakeScraper{}
	_, products, stale, err := NewRefreshService(repo, scraper, time.Minute, nil).Search(context.Background(), models.SearchQuery{Item: "Shoes"})
	if err != nil || stale || len(products) != 1 || scraper.Calls() != 0 {
		t.Fatalf("products=%v stale=%v calls=%d err=%v", products, stale, scraper.Calls(), err)
	}
}

func TestNeverScrapedSearchRefreshesSynchronously(t *testing.T) {
	repo := &fakeRepository{search: models.Search{ID: 1, Item: "shoes", ScraperQuery: "shoes"}}
	scraper := &fakeScraper{records: []map[string]any{{"title": "new"}}}
	_, _, _, err := NewRefreshService(repo, scraper, time.Minute, nil).Search(context.Background(), models.SearchQuery{Item: "Shoes"})
	if err != nil || scraper.Calls() != 1 || !repo.stored {
		t.Fatalf("calls=%d stored=%v err=%v", scraper.Calls(), repo.stored, err)
	}
}

func TestNeverScrapedSearchWaitsForExistingRefresh(t *testing.T) {
	scraped := time.Now()
	repo := &fakeRepository{search: models.Search{ID: 1, Item: "shoes", ScraperQuery: "shoes", RefreshInProgress: true}}
	scraper := &fakeScraper{records: []map[string]any{{"title": "new"}}, started: make(chan struct{}), wait: make(chan struct{})}
	service := NewRefreshService(repo, scraper, time.Minute, nil)
	done := make(chan error, 1)
	go func() {
		_, _, _, err := service.Search(context.Background(), models.SearchQuery{Item: "Shoes"})
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	repo.mu.Lock()
	repo.search.RefreshInProgress = false
	repo.search.LastScrapedAt = &scraped
	repo.search.ExpiresAt = &scraped
	repo.mu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if scraper.Calls() != 0 {
		t.Fatalf("scraper calls=%d, want no duplicate refresh", scraper.Calls())
	}
}

func TestFailedRefreshClearsLock(t *testing.T) {
	repo := &fakeRepository{search: models.Search{ID: 1, ScraperQuery: "shoes"}, claimSeen: make(chan struct{}, 2)}
	scraper := &fakeScraper{err: errors.New("scraper failed")}
	if err := NewRefreshService(repo, scraper, time.Minute, nil).RefreshSearch(context.Background(), 1); err == nil {
		t.Fatal("expected refresh error")
	}
	if repo.failed != 1 || repo.claim {
		t.Fatalf("failed=%d claim=%v", repo.failed, repo.claim)
	}
}

func TestConcurrentRefreshProtection(t *testing.T) {
	repo := &fakeRepository{search: models.Search{ID: 1, ScraperQuery: "shoes"}, claimSeen: make(chan struct{}, 2)}
	scraper := &fakeScraper{records: []map[string]any{{"title": "new"}}, started: make(chan struct{}), wait: make(chan struct{})}
	service := NewRefreshService(repo, scraper, time.Minute, nil)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = service.RefreshSearch(context.Background(), 1) }()
	<-scraper.started
	wg.Add(1)
	go func() { defer wg.Done(); _ = service.RefreshSearch(context.Background(), 1) }()
	<-repo.claimSeen
	<-repo.claimSeen
	close(scraper.wait)
	wg.Wait()
	if repo.claimCalls != 2 || scraper.Calls() != 1 {
		t.Fatalf("claim calls=%d scraper calls=%d", repo.claimCalls, scraper.Calls())
	}
}

func TestSuccessfulRefreshUpdatesTTL(t *testing.T) {
	repo := &fakeRepository{search: models.Search{ID: 1, ScraperQuery: "shoes"}}
	scraper := &fakeScraper{records: []map[string]any{{"title": "new"}}}
	ttl := 45 * time.Minute
	if err := NewRefreshService(repo, scraper, ttl, nil).RefreshSearch(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	remaining := time.Until(repo.expiresAt)
	if !repo.stored || remaining > ttl || remaining <= ttl-time.Second {
		t.Fatalf("unexpected expiry %v", repo.expiresAt)
	}
}

func TestWorkerProcessesOnlyNeededSearches(t *testing.T) {
	repo := &fakeRepository{needed: []models.Search{{ID: 1}}, search: models.Search{ID: 1, ScraperQuery: "shoes"}}
	scraper := &fakeScraper{records: []map[string]any{{"title": "new"}}}
	worker := NewRefreshWorker(NewRefreshService(repo, scraper, time.Minute, nil), time.Hour, nil)
	worker.runOnce(context.Background())
	if scraper.Calls() != 1 {
		t.Fatalf("worker scraper calls=%d", scraper.Calls())
	}
}
