package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"scraper-backend/internal/models"
	"scraper-backend/internal/repository"
)

type PersistenceService struct {
	Repository *repository.Repository
	TTL        time.Duration
}

func NewPersistenceService(repo *repository.Repository, ttl time.Duration) *PersistenceService {
	return &PersistenceService{Repository: repo, TTL: ttl}
}

func (s *PersistenceService) Persist(ctx context.Context, dimensions models.SearchQuery, records []map[string]any, scrapedAt time.Time) error {
	if s == nil || s.Repository == nil {
		return nil
	}
	normalized, err := models.NormalizeSearchQuery(dimensions)
	if err != nil {
		return err
	}
	search, err := s.Repository.GetOrCreateSearch(ctx, normalized)
	if err != nil {
		return err
	}
	if err := s.Repository.MarkRefreshStarted(ctx, search.ID); err != nil {
		return err
	}

	products := make([]models.Product, 0, len(records))
	observations := make([]models.PriceObservation, 0, len(records))
	for _, record := range records {
		product := models.Product{
			SearchID:   search.ID,
			Retailer:   stringValue(record, "retailerName", "retailer"),
			ExternalID: stringValue(record, "productId", "externalId", "id"),
			Title:      stringValue(record, "title", "name"),
			Brand:      stringValue(record, "brand"),
			ProductURL: stringValue(record, "externalUrl", "productUrl", "url"),
			ImageURL:   stringValue(record, "imageUrl", "image_url"),
		}
		if product.Retailer == "" {
			product.Retailer = "unknown"
		}
		if product.ExternalID == "" {
			product.ExternalID = fallbackExternalID(record)
		}
		products = append(products, product)
		observations = append(observations, models.PriceObservation{
			CurrentPrice:  numberValue(record, "currentPrice", "price"),
			OriginalPrice: numberValue(record, "originalPrice", "mrp"),
			Rating:        numberValue(record, "rating"),
			ReviewCount:   integerValue(record, "reviewCount"),
			ScrapedAt:     scrapedAt,
		})
	}
	if err := s.Repository.ReplaceSearchProducts(ctx, search.ID, products, observations, scrapedAt, s.TTL); err != nil {
		_ = s.Repository.MarkRefreshFailed(ctx, search.ID)
		return err
	}
	return nil
}

func stringValue(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := record[key]; ok && value != nil {
			if result, ok := value.(string); ok {
				return strings.TrimSpace(result)
			}
			return fmt.Sprint(value)
		}
	}
	return ""
}

func numberValue(record map[string]any, keys ...string) *float64 {
	value := stringValue(record, keys...)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func integerValue(record map[string]any, keys ...string) *int64 {
	value := stringValue(record, keys...)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(strings.ReplaceAll(value, ",", ""), 10, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func fallbackExternalID(record map[string]any) string {
	encoded, err := json.Marshal(record)
	if err != nil {
		return "record"
	}
	return fmt.Sprintf("record-%x", encoded)
}
