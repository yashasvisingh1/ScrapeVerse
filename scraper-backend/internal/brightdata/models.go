package brightdata

import "net/http"

// BrightDataClient holds the configured HTTP client and necessary settings for calling the Bright Data API.
type BrightDataClient struct {
	HTTPClient  *http.Client
	BaseURL     string
	APIToken    string
	CollectorID string
}

// These structures are intentionally flexible. Bright Data responses are not always a single fixed schema,
// so parsing is done dynamically when working with collection status and results.
type TriggerPayload struct {
	ID string `json:"id"`
}
