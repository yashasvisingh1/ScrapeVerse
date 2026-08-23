package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"scraper-backend/internal/brightdata"
	"scraper-backend/internal/models"
)

type ProductScraper interface {
	Search(ctx context.Context, query string, pollInterval, pollTimeout time.Duration) ([]map[string]any, error)
}

type BrightDataScraper struct {
	Scraper      *brightdata.Scraper
	PollInterval time.Duration
	PollTimeout  time.Duration
}

type SearchRepository interface {
	GetSearch(context.Context, models.SearchQuery) (*models.Search, error)
	GetSearchByID(context.Context, int64) (*models.Search, error)
	GetOrCreateSearch(context.Context, models.SearchQuery) (*models.Search, error)
	GetProductsWithLatestPrices(context.Context, int64) ([]map[string]any, error)
	ClaimRefresh(context.Context, int64) (bool, error)
	MarkRefreshFailed(context.Context, int64) error
	ReplaceSearchProducts(context.Context, int64, []models.Product, []models.PriceObservation, time.Time, time.Duration) error
	FindSearchesNeedingRefresh(context.Context) ([]models.Search, error)
}

func (s BrightDataScraper) Search(ctx context.Context, query string, _, _ time.Duration) ([]map[string]any, error) {
	return s.Scraper.Search(ctx, query, s.PollInterval, s.PollTimeout)
}

type RefreshService struct {
	Repository SearchRepository
	Scraper    ProductScraper
	TTL        time.Duration
	Logger     *log.Logger
}

func NewRefreshService(repo SearchRepository, scraper ProductScraper, ttl time.Duration, logger *log.Logger) *RefreshService {
	return &RefreshService{Repository: repo, Scraper: scraper, TTL: ttl, Logger: logger}
}

// Search returns cached products and refreshes only when the search is missing or stale.
func (s *RefreshService) Search(ctx context.Context, query models.SearchQuery) (*models.Search, []map[string]any, bool, error) {
	if s == nil || s.Repository == nil {
		return nil, nil, false, fmt.Errorf("refresh service is not configured")
	}
	normalized, err := models.NormalizeSearchQuery(query)
	if err != nil {
		return nil, nil, false, err
	}
	search, err := s.Repository.GetSearch(ctx, normalized)
	if err != nil {
		return nil, nil, false, err
	}
	products, err := s.Repository.GetProductsWithLatestPrices(ctx, search.ID)
	if err != nil {
		return nil, nil, false, err
	}
	if search.LastScrapedAt == nil {
		if !search.RefreshInProgress {
			if err := s.RefreshSearch(ctx, search.ID); err != nil {
				return nil, nil, false, err
			}
		} else if err := s.waitForInitialRefresh(ctx, search.ID); err != nil {
			return nil, nil, false, err
		}
		products, err = s.Repository.GetProductsWithLatestPrices(ctx, search.ID)
		return search, products, true, err
	}
	fresh := search.ExpiresAt != nil && search.ExpiresAt.After(time.Now())
	if !fresh && len(products) == 0 {
		if err := s.RefreshSearch(ctx, search.ID); err != nil {
			return nil, nil, false, err
		}
		search, err = s.Repository.GetSearchByID(ctx, search.ID)
		if err != nil {
			return nil, nil, false, err
		}
		products, err = s.Repository.GetProductsWithLatestPrices(ctx, search.ID)
		return search, products, true, err
	}
	if !fresh {
		s.RefreshSearchAsync(context.Background(), search.ID)
	}
	return search, products, !fresh, nil
}

func (s *RefreshService) SearchExisting(ctx context.Context, query models.SearchQuery) (*models.Search, []map[string]any, bool, error) {
	if s == nil || s.Repository == nil {
		return nil, nil, false, fmt.Errorf("refresh service is not configured")
	}
	normalized, err := models.NormalizeSearchQuery(query)
	if err != nil {
		return nil, nil, false, err
	}
	search, err := s.Repository.GetSearch(ctx, normalized)
	if err != nil {
		return nil, nil, false, err
	}
	products, err := s.Repository.GetProductsWithLatestPrices(ctx, search.ID)
	if err != nil {
		return nil, nil, false, err
	}
	if search.LastScrapedAt == nil {
		if !search.RefreshInProgress {
			if err := s.RefreshSearch(ctx, search.ID); err != nil {
				return nil, nil, false, err
			}
		} else if err := s.waitForInitialRefresh(ctx, search.ID); err != nil {
			return nil, nil, false, err
		}
		search, err = s.Repository.GetSearchByID(ctx, search.ID)
		if err != nil {
			return nil, nil, false, err
		}
		products, err = s.Repository.GetProductsWithLatestPrices(ctx, search.ID)
		return search, products, true, err
	}
	fresh := search.ExpiresAt != nil && search.ExpiresAt.After(time.Now())
	if !fresh {
		s.RefreshSearchAsync(context.Background(), search.ID)
	}
	return search, products, !fresh, nil
}

func (s *RefreshService) StartRefresh(ctx context.Context, query models.SearchQuery) (bool, int64, error) {
	if s == nil || s.Repository == nil || s.Scraper == nil {
		return false, 0, fmt.Errorf("refresh service is not configured")
	}
	normalized, err := models.NormalizeSearchQuery(query)
	if err != nil {
		return false, 0, err
	}
	search, err := s.Repository.GetSearch(ctx, normalized)
	if err != nil {
		return false, 0, err
	}
	claimed, err := s.Repository.ClaimRefresh(ctx, search.ID)
	if err != nil {
		return false, 0, err
	}
	if !claimed {
		s.logf("refresh skipped search_id=%d: another refresh owns the lock", search.ID)
		return false, search.ID, nil
	}
	go s.refreshClaimed(context.Background(), search)
	return true, search.ID, nil
}

func (s *RefreshService) waitForInitialRefresh(ctx context.Context, searchID int64) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		search, err := s.Repository.GetSearchByID(ctx, searchID)
		if err != nil {
			return err
		}
		if !search.RefreshInProgress {
			if search.LastScrapedAt == nil {
				return fmt.Errorf("initial refresh finished without updating search %d", searchID)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *RefreshService) RefreshSearch(ctx context.Context, searchID int64) error {
	if s == nil || s.Repository == nil || s.Scraper == nil {
		return fmt.Errorf("refresh service is not configured")
	}
	search, err := s.Repository.GetSearchByID(ctx, searchID)
	if err != nil {
		return err
	}
	claimed, err := s.Repository.ClaimRefresh(ctx, searchID)
	if err != nil {
		return err
	}
	if !claimed {
		s.logf("refresh skipped search_id=%d: another refresh owns the lock", searchID)
		return nil
	}
	return s.refreshClaimed(ctx, search)
}

func (s *RefreshService) refreshClaimed(ctx context.Context, search *models.Search) error {
	searchID := search.ID
	start := time.Now()
	s.logf("refresh lock acquired search_id=%d", searchID)
	completed := false
	defer func() {
		if completed {
			return
		}
		if err := s.Repository.MarkRefreshFailed(context.Background(), searchID); err != nil {
			s.logf("clear refresh lock failed search_id=%d error=%v", searchID, err)
		}
	}()

	s.logf("refresh started search_id=%d query=%s", searchID, search.ScraperQuery)
	records, err := s.Scraper.Search(ctx, search.ScraperQuery, 0, 0)
	if err != nil {
		s.logf("refresh failed search_id=%d error=%v duration=%s", searchID, err, time.Since(start))
		return err
	}
	s.logf("products received search_id=%d count=%d", searchID, len(records))

	products, observations := productData(searchID, records, time.Now())
	if err := s.Repository.ReplaceSearchProducts(ctx, searchID, products, observations, time.Now(), s.TTL); err != nil {
		s.logf("refresh failed search_id=%d persistence error=%v duration=%s", searchID, err, time.Since(start))
		return err
	}
	completed = true
	s.logf("products persisted search_id=%d count=%d", searchID, len(products))
	s.logf("refresh completed search_id=%d duration=%s", searchID, time.Since(start))
	return nil
}

func (s *RefreshService) RefreshSearchAsync(ctx context.Context, searchID int64) {
	go func() { _ = s.RefreshSearch(ctx, searchID) }()
}

func (s *RefreshService) GetOrCreateAndRefresh(ctx context.Context, query models.SearchQuery) (*models.Search, error) {
	normalized, err := models.NormalizeSearchQuery(query)
	if err != nil {
		return nil, err
	}
	search, err := s.Repository.GetOrCreateSearch(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return search, nil
}

func (s *RefreshService) logf(format string, args ...any) {
	if s.Logger != nil {
		s.Logger.Printf(format, args...)
	}
}

func productData(searchID int64, records []map[string]any, scrapedAt time.Time) ([]models.Product, []models.PriceObservation) {
	products := make([]models.Product, 0, len(records))
	observations := make([]models.PriceObservation, 0, len(records))
	for _, record := range records {
		product := models.Product{SearchID: searchID, Retailer: stringValue(record, "retailerName", "retailer"), ExternalID: stringValue(record, "productId", "externalId", "id"), Title: stringValue(record, "title", "name"), Brand: stringValue(record, "brand"), ProductURL: stringValue(record, "externalUrl", "productUrl", "url"), ImageURL: stringValue(record, "imageUrl", "image_url")}
		if product.Retailer == "" {
			product.Retailer = "unknown"
		}
		if product.ExternalID == "" {
			product.ExternalID = fallbackExternalID(record)
		}
		products = append(products, product)
		observations = append(observations, PriceObservation(record, scrapedAt))
	}
	return products, observations
}

func PriceObservation(record map[string]any, scrapedAt time.Time) models.PriceObservation {
	return models.PriceObservation{CurrentPrice: numberValue(record, "currentPrice", "price"), OriginalPrice: numberValue(record, "originalPrice", "mrp"), Rating: numberValue(record, "rating"), ReviewCount: integerValue(record, "reviewCount"), ScrapedAt: scrapedAt}
}
