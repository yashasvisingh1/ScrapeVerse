import type { Product } from '../types/product'

type Props = { product: Product; cheapest: boolean }

function formatPrice(value: number | null) {
  return value == null ? 'Price unavailable' : `₹${Math.round(value).toLocaleString('en-IN')}`
}

export function ProductCard({ product, cheapest }: Props) {
  const discount = product.originalPrice && product.currentPrice && product.originalPrice > product.currentPrice
    ? Math.round((1 - product.currentPrice / product.originalPrice) * 100)
    : null

  return (
    <article className={`product-card${cheapest ? ' is-cheapest' : ''}`}>
      <div className="product-image-wrap">
        {product.imageUrl ? <img src={product.imageUrl} alt={product.title || 'Product image'} loading="lazy" onError={(event) => { event.currentTarget.style.display = 'none' }} /> : <span className="image-fallback" aria-hidden="true">{product.retailer.slice(0, 1)}</span>}
        {cheapest && <span className="price-badge">Lowest price</span>}
      </div>
      <div className="product-card-body">
        <div className="product-meta"><span className="retailer-label">{product.retailer}</span><span>{product.brand || 'Independent seller'}</span></div>
        <h3>{product.title || 'Untitled product'}</h3>
        <div className="price-line"><strong>{formatPrice(product.currentPrice)}</strong>{discount != null && <span className="discount">{discount}% off</span>}</div>
        {product.originalPrice && product.originalPrice > (product.currentPrice ?? 0) && <span className="original-price">{formatPrice(product.originalPrice)}</span>}
        <div className="rating-row"><span className="rating">★ {product.rating?.toFixed(1) ?? '—'}</span><span>{product.reviewCount != null ? `${product.reviewCount.toLocaleString('en-IN')} reviews` : 'No reviews'}</span></div>
        <a className="view-link" href={product.productUrl || undefined} target="_blank" rel="noreferrer">View product <span aria-hidden="true">↗</span></a>
      </div>
    </article>
  )
}
