package models

import "testing"

func TestNormalizeSearchQuery(t *testing.T) {
	query, err := NormalizeSearchQuery(SearchQuery{Brand: "  Nike  ", Item: "  Running   Shoes ", Gender: " MEN "})
	if err != nil {
		t.Fatalf("NormalizeSearchQuery() error = %v", err)
	}
	if query.Brand != "nike" || query.Item != "running shoes" || query.Gender != "men" {
		t.Fatalf("normalized query = %#v", query)
	}
	if query.SearchKey() != "nike:running shoes:men" {
		t.Fatalf("SearchKey() = %q", query.SearchKey())
	}
}

func TestNormalizeSearchQueryDefaultsAll(t *testing.T) {
	query, err := NormalizeSearchQuery(SearchQuery{Item: "Shoes"})
	if err != nil {
		t.Fatalf("NormalizeSearchQuery() error = %v", err)
	}
	if query.Brand != "all" || query.Gender != "all" {
		t.Fatalf("defaults = %#v", query)
	}
	if query.SearchKey() != "all:shoes:all" {
		t.Fatalf("SearchKey() = %q", query.SearchKey())
	}
	if query.ScraperQuery() != "shoes" {
		t.Fatalf("ScraperQuery() = %q", query.ScraperQuery())
	}
}

func TestNormalizeSearchQueryRequiresItem(t *testing.T) {
	if _, err := NormalizeSearchQuery(SearchQuery{Brand: "Nike"}); err == nil {
		t.Fatal("expected missing item error")
	}
}
