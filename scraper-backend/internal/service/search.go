package service

import "scraper-backend/internal/models"

func NormalizeSearchQuery(query models.SearchQuery) (models.SearchQuery, error) {
	return models.NormalizeSearchQuery(query)
}
