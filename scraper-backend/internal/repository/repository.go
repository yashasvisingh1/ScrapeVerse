package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"scraper-backend/internal/models"
)

type Repository struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{Pool: pool} }

func (r *Repository) GetSearch(ctx context.Context, query models.SearchQuery) (*models.Search, error) {
	row := r.Pool.QueryRow(ctx, `SELECT id, brand, item, gender, scraper_query, last_scraped_at, expires_at, refresh_in_progress, created_at, updated_at FROM searches WHERE brand=$1 AND item=$2 AND gender=$3`, query.Brand, query.Item, query.Gender)
	return scanSearch(row)
}

func (r *Repository) GetOrCreateSearch(ctx context.Context, query models.SearchQuery) (*models.Search, error) {
	row := r.Pool.QueryRow(ctx, `INSERT INTO searches (brand, item, gender, scraper_query) VALUES ($1, $2, $3, $4) ON CONFLICT (brand, item, gender) DO UPDATE SET updated_at=searches.updated_at RETURNING id, brand, item, gender, scraper_query, last_scraped_at, expires_at, refresh_in_progress, created_at, updated_at`, query.Brand, query.Item, query.Gender, query.ScraperQuery())
	return scanSearch(row)
}

func (r *Repository) GetSearchByID(ctx context.Context, searchID int64) (*models.Search, error) {
	row := r.Pool.QueryRow(ctx, `SELECT id, brand, item, gender, scraper_query, last_scraped_at, expires_at, refresh_in_progress, created_at, updated_at FROM searches WHERE id=$1`, searchID)
	return scanSearch(row)
}

func (r *Repository) ListSearches(ctx context.Context) ([]models.Search, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id, brand, item, gender, scraper_query, last_scraped_at, expires_at, refresh_in_progress, created_at, updated_at FROM searches ORDER BY brand, item, gender`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	searches := []models.Search{}
	for rows.Next() {
		search, err := scanSearch(rows)
		if err != nil {
			return nil, err
		}
		searches = append(searches, *search)
	}
	return searches, rows.Err()
}

func (r *Repository) GetProductsBySearchID(ctx context.Context, searchID int64) ([]models.Product, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id, search_id, retailer, external_id, title, brand, product_url, image_url, created_at, updated_at FROM products WHERE search_id=$1 ORDER BY id`, searchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := []models.Product{}
	for rows.Next() {
		var product models.Product
		if err := rows.Scan(&product.ID, &product.SearchID, &product.Retailer, &product.ExternalID, &product.Title, &product.Brand, &product.ProductURL, &product.ImageURL, &product.CreatedAt, &product.UpdatedAt); err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (r *Repository) GetProductsWithLatestPrices(ctx context.Context, searchID int64) ([]map[string]any, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT p.retailer, p.external_id, p.title, p.brand, p.product_url, p.image_url,
		       pp.current_price, pp.original_price, pp.rating, pp.review_count, pp.scraped_at
		FROM products p
		LEFT JOIN LATERAL (
			SELECT current_price, original_price, rating, review_count, scraped_at
			FROM product_prices
			WHERE product_id = p.id
			ORDER BY scraped_at DESC, id DESC
			LIMIT 1
		) pp ON TRUE
		WHERE p.search_id=$1
		ORDER BY p.id`, searchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []map[string]any{}
	for rows.Next() {
		var retailer, externalID, title, brand, productURL, imageURL string
		var currentPrice, originalPrice, rating *float64
		var reviewCount *int64
		var scrapedAt *time.Time
		if err := rows.Scan(&retailer, &externalID, &title, &brand, &productURL, &imageURL, &currentPrice, &originalPrice, &rating, &reviewCount, &scrapedAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]any{
			"retailerName": retailer, "productId": externalID, "title": title, "brand": brand,
			"externalUrl": productURL, "imageUrl": imageURL, "currentPrice": floatValue(currentPrice),
			"originalPrice": floatValue(originalPrice), "rating": floatValue(rating), "reviewCount": intValue(reviewCount),
			"scrapedAt": timeValue(scrapedAt),
		})
	}
	return results, rows.Err()
}

func floatValue(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func intValue(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func timeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339)
}

func (r *Repository) StoreProducts(ctx context.Context, products []models.Product) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, product := range products {
		if _, err := tx.Exec(ctx, `INSERT INTO products (search_id, retailer, external_id, title, brand, product_url, image_url) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (search_id, retailer, external_id) DO UPDATE SET title=EXCLUDED.title, brand=EXCLUDED.brand, product_url=EXCLUDED.product_url, image_url=EXCLUDED.image_url, updated_at=NOW()`, product.SearchID, product.Retailer, product.ExternalID, product.Title, product.Brand, product.ProductURL, product.ImageURL); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) StorePriceObservation(ctx context.Context, observation models.PriceObservation) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO product_prices (product_id, current_price, original_price, rating, review_count, scraped_at) VALUES ($1,$2,$3,$4,$5,$6)`, observation.ProductID, observation.CurrentPrice, observation.OriginalPrice, observation.Rating, observation.ReviewCount, observation.ScrapedAt)
	return err
}

func (r *Repository) ReplaceSearchProducts(ctx context.Context, searchID int64, products []models.Product, observations []models.PriceObservation, scrapedAt time.Time, ttl time.Duration) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	productIDs := make([]int64, 0, len(products))
	for _, product := range products {
		var productID int64
		err := tx.QueryRow(ctx, `INSERT INTO products (search_id, retailer, external_id, title, brand, product_url, image_url) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (search_id, retailer, external_id) DO UPDATE SET title=EXCLUDED.title, brand=EXCLUDED.brand, product_url=EXCLUDED.product_url, image_url=EXCLUDED.image_url, updated_at=NOW() RETURNING id`, searchID, product.Retailer, product.ExternalID, product.Title, product.Brand, product.ProductURL, product.ImageURL).Scan(&productID)
		if err != nil {
			return fmt.Errorf("store product %s/%s: %w", product.Retailer, product.ExternalID, err)
		}
		productIDs = append(productIDs, productID)
	}
	for index, observation := range observations {
		if index >= len(productIDs) {
			return fmt.Errorf("price observation count exceeds product count")
		}
		_, err := tx.Exec(ctx, `INSERT INTO product_prices (product_id, current_price, original_price, rating, review_count, scraped_at) VALUES ($1,$2,$3,$4,$5,$6)`, productIDs[index], observation.CurrentPrice, observation.OriginalPrice, observation.Rating, observation.ReviewCount, observation.ScrapedAt)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE searches SET last_scraped_at=$1, expires_at=$2, refresh_in_progress=false, updated_at=NOW() WHERE id=$3`, scrapedAt, scrapedAt.Add(ttl), searchID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) GetLatestPrice(ctx context.Context, productID int64) (*models.PriceObservation, error) {
	var observation models.PriceObservation
	err := r.Pool.QueryRow(ctx, `SELECT id, product_id, current_price, original_price, rating, review_count, scraped_at FROM product_prices WHERE product_id=$1 ORDER BY scraped_at DESC, id DESC LIMIT 1`, productID).Scan(&observation.ID, &observation.ProductID, &observation.CurrentPrice, &observation.OriginalPrice, &observation.Rating, &observation.ReviewCount, &observation.ScrapedAt)
	if err != nil {
		return nil, err
	}
	return &observation, nil
}

func (r *Repository) GetSearchFreshness(ctx context.Context, searchID int64) (*time.Time, *time.Time, error) {
	var scrapedAt, expiresAt *time.Time
	err := r.Pool.QueryRow(ctx, `SELECT last_scraped_at, expires_at FROM searches WHERE id=$1`, searchID).Scan(&scrapedAt, &expiresAt)
	return scrapedAt, expiresAt, err
}

func (r *Repository) MarkRefreshStarted(ctx context.Context, searchID int64) error {
	_, err := r.ClaimRefresh(ctx, searchID)
	return err
}

func (r *Repository) ClaimRefresh(ctx context.Context, searchID int64) (bool, error) {
	result, err := r.Pool.Exec(ctx, `UPDATE searches SET refresh_in_progress=true, updated_at=NOW() WHERE id=$1 AND refresh_in_progress=false`, searchID)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}

func (r *Repository) ClearRefreshLocks(ctx context.Context) error {
	_, err := r.Pool.Exec(ctx, `UPDATE searches SET refresh_in_progress=false, updated_at=NOW() WHERE refresh_in_progress=true`)
	return err
}

func (r *Repository) FindSearchesNeedingRefresh(ctx context.Context) ([]models.Search, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id, brand, item, gender, scraper_query, last_scraped_at, expires_at, refresh_in_progress, created_at, updated_at FROM searches WHERE last_scraped_at IS NULL OR expires_at IS NULL OR expires_at <= NOW() ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	searches := []models.Search{}
	for rows.Next() {
		search, err := scanSearch(rows)
		if err != nil {
			return nil, err
		}
		searches = append(searches, *search)
	}
	return searches, rows.Err()
}

func (r *Repository) MarkRefreshCompleted(ctx context.Context, searchID int64, scrapedAt time.Time, ttl time.Duration) error {
	_, err := r.Pool.Exec(ctx, `UPDATE searches SET last_scraped_at=$1, expires_at=$2, refresh_in_progress=false, updated_at=NOW() WHERE id=$3`, scrapedAt, scrapedAt.Add(ttl), searchID)
	return err
}

func (r *Repository) MarkRefreshFailed(ctx context.Context, searchID int64) error {
	_, err := r.Pool.Exec(ctx, `UPDATE searches SET refresh_in_progress=false, updated_at=NOW() WHERE id=$1`, searchID)
	return err
}

func scanSearch(row interface{ Scan(...any) error }) (*models.Search, error) {
	var search models.Search
	if err := row.Scan(&search.ID, &search.Brand, &search.Item, &search.Gender, &search.ScraperQuery, &search.LastScrapedAt, &search.ExpiresAt, &search.RefreshInProgress, &search.CreatedAt, &search.UpdatedAt); err != nil {
		return nil, err
	}
	return &search, nil
}
