package repository

import (
	"context"
	"fmt"

	"scraper-backend/internal/models"
)

var SeedCatalog = buildSeedCatalog()

func buildSeedCatalog() []models.SearchQuery {
	brands := []string{
		"Nike", "Adidas", "Puma", "Reebok", "New Balance", "Skechers", "Asics", "Converse",
		"Vans", "Under Armour", "Levi's", "Roadster", "H&M", "Zara", "Uniqlo", "The North Face",
	}
	items := []string{"Shoes", "Sneakers", "T-Shirts", "Shirts", "Jeans", "Jackets", "Hoodies", "Shorts"}
	genders := []string{"Men", "Women", "Unisex"}

	catalog := make([]models.SearchQuery, 0, len(brands)*len(items)*len(genders)+len(items))
	for _, brand := range brands {
		for _, item := range items {
			for _, gender := range genders {
				catalog = append(catalog, models.SearchQuery{Brand: brand, Item: item, Gender: gender})
			}
		}
	}
	for _, item := range items {
		catalog = append(catalog, models.SearchQuery{Brand: "All", Item: item, Gender: "All"})
	}
	return catalog
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
