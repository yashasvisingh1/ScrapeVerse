package models

import (
	"encoding/json"
	"time"
)

// ScrapeRequest contains the request payload accepted by the backend HTTP API.
type ScrapeRequest struct {
	Query  string `json:"query,omitempty"`
	URL    string `json:"url,omitempty"`
	Brand  string `json:"brand,omitempty"`
	Item   string `json:"item,omitempty"`
	Gender string `json:"gender,omitempty"`
}

// ScrapeResponse is returned to the client after a successful scrape.
type ScrapeResponse struct {
	Success      bool             `json:"success"`
	CollectionID string           `json:"collection_id,omitempty"`
	Timestamp    string           `json:"timestamp,omitempty"`
	Records      int              `json:"records"`
	CSVFile      string           `json:"csv_file,omitempty"`
	Data         []map[string]any `json:"data,omitempty"`
}

// ErrorResponse is returned to the client when an error occurs.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type ProductResponse struct {
	ID            int64      `json:"id"`
	Retailer      string     `json:"retailer"`
	ExternalID    string     `json:"externalId"`
	Title         string     `json:"title"`
	Brand         string     `json:"brand"`
	CurrentPrice  *float64   `json:"currentPrice"`
	OriginalPrice *float64   `json:"originalPrice"`
	Currency      string     `json:"currency,omitempty"`
	Rating        *float64   `json:"rating"`
	ReviewCount   *int64     `json:"reviewCount"`
	ImageURL      string     `json:"imageUrl"`
	ProductURL    string     `json:"productUrl"`
	ScrapedAt     *time.Time `json:"scrapedAt"`
}

type SearchResponse struct {
	Brand  string `json:"brand"`
	Item   string `json:"item"`
	Gender string `json:"gender"`
}

// MarshalJSON ensures error payloads are always sent as valid JSON objects.
func (e ErrorResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}{
		Success: false,
		Error:   e.Error,
	})
}
