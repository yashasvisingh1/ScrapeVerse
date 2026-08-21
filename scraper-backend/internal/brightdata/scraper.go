package brightdata

import (
	"context"
	"fmt"
	"time"
)

// Scraper is a convenience wrapper around the BrightDataClient for a full scrape workflow.
type Scraper struct {
	Client *BrightDataClient
}

func NewScraper(client *BrightDataClient) *Scraper {
	return &Scraper{Client: client}
}

func (s *Scraper) Run(ctx context.Context, collectionID string, pollInterval, pollTimeout time.Duration) ([]map[string]any, string, error) {
	if s == nil || s.Client == nil {
		return nil, "", fmt.Errorf("bright data scraper is not configured")
	}

	status, err := s.Client.PollForCompletion(ctx, collectionID, pollInterval, pollTimeout)
	if err != nil {
		return nil, status, err
	}

	results, err := s.Client.GetCollectionResults(ctx, collectionID)
	if err != nil {
		return nil, collectionID, err
	}

	return results, collectionID, nil
}
