package models

import (
	"fmt"
	"strings"
)

// SearchQuery is the fixed search dimension set used by the product catalog.
type SearchQuery struct {
	Brand  string `json:"brand"`
	Item   string `json:"item"`
	Gender string `json:"gender"`
}

func NormalizeSearchQuery(query SearchQuery) (SearchQuery, error) {
	query.Brand = normalizeDimension(query.Brand)
	query.Item = normalizeDimension(query.Item)
	query.Gender = normalizeDimension(query.Gender)

	if query.Brand == "" {
		query.Brand = "all"
	}
	if query.Gender == "" {
		query.Gender = "all"
	}
	if query.Item == "" {
		return SearchQuery{}, fmt.Errorf("item is required")
	}
	return query, nil
}

func (query SearchQuery) SearchKey() string {
	return strings.Join([]string{query.Brand, query.Item, query.Gender}, ":")
}

func (query SearchQuery) ScraperQuery() string {
	parts := make([]string, 0, 3)
	for _, value := range []string{query.Brand, query.Item, query.Gender} {
		if value != "" && value != "all" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " ")
}

func normalizeDimension(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}
