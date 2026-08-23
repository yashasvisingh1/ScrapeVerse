CREATE TABLE IF NOT EXISTS searches (
    id BIGSERIAL PRIMARY KEY,
    brand TEXT NOT NULL DEFAULT 'all',
    item TEXT NOT NULL,
    gender TEXT NOT NULL DEFAULT 'all',
    scraper_query TEXT NOT NULL,
    last_scraped_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    refresh_in_progress BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT searches_item_not_blank CHECK (length(btrim(item)) > 0),
    CONSTRAINT searches_unique_dimensions UNIQUE (brand, item, gender)
);

CREATE TABLE IF NOT EXISTS products (
    id BIGSERIAL PRIMARY KEY,
    search_id BIGINT NOT NULL REFERENCES searches(id) ON DELETE CASCADE,
    retailer TEXT NOT NULL,
    external_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    brand TEXT NOT NULL DEFAULT '',
    product_url TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT products_unique_search_retailer_external UNIQUE (search_id, retailer, external_id)
);

CREATE TABLE IF NOT EXISTS product_prices (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    current_price NUMERIC,
    original_price NUMERIC,
    rating NUMERIC,
    review_count BIGINT,
    scraped_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS product_prices_product_scraped_idx ON product_prices(product_id, scraped_at DESC);
