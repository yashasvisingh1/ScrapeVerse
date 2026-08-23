package repository

import (
	"context"
	"fmt"

	"scraper-backend/internal/models"
)

var SeedCatalog = []models.SearchQuery{
	{Brand: "Nike", Item: "Shoes", Gender: "Men"},
	{Brand: "Nike", Item: "Shoes", Gender: "Women"},
	{Brand: "Adidas", Item: "Shoes", Gender: "Men"},
	{Brand: "Adidas", Item: "Shoes", Gender: "Women"},
	{Brand: "Puma", Item: "T-Shirts", Gender: "Men"},
	{Brand: "Puma", Item: "T-Shirts", Gender: "Women"},
	{Brand: "All", Item: "Shoes", Gender: "Men"},
	{Brand: "All", Item: "Shoes", Gender: "Women"},
}

func SeedSearchCatalog(ctx context.Context, repo *Repository) error {
	if repo == nil || repo.Pool == nil {
		return fmt.Errorf("repository is not configured")
	}
	for _, query := range SeedCatalog {
		normalized, err := models.NormalizeSearchQuery(query)
		if err != nil {
			return err
		}
		if _, err := repo.GetOrCreateSearch(ctx, normalized); err != nil {
			return err
		}
	}
	return nil
}
