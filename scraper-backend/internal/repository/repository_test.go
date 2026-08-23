package repository

import (
	"context"
	"testing"

	"scraper-backend/internal/models"
)

func TestSeedCatalogNormalizesToExpectedQueries(t *testing.T) {
	for _, input := range SeedCatalog {
		query, err := models.NormalizeSearchQuery(input)
		if err != nil {
			t.Fatalf("normalize %q: %v", input.Item, err)
		}
		if query.ScraperQuery() == "" {
			t.Fatalf("empty scraper query for %#v", input)
		}
	}
}

func TestSeedSearchCatalogRequiresRepository(t *testing.T) {
	if err := SeedSearchCatalog(context.Background(), nil); err == nil {
		t.Fatal("expected nil repository to fail")
	}
}
