package models

import "time"

type Search struct {
	ID                int64
	Brand             string
	Item              string
	Gender            string
	ScraperQuery      string
	LastScrapedAt     *time.Time
	ExpiresAt         *time.Time
	RefreshInProgress bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Product struct {
	ID         int64
	SearchID   int64
	Retailer   string
	ExternalID string
	Title      string
	Brand      string
	ProductURL string
	ImageURL   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type PriceObservation struct {
	ID            int64
	ProductID     int64
	CurrentPrice  *float64
	OriginalPrice *float64
	Rating        *float64
	ReviewCount   *int64
	ScrapedAt     time.Time
}
