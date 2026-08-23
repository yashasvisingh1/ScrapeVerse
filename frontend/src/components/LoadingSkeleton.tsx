export function LoadingSkeleton() {
  return <div className="product-grid" aria-label="Loading products">{Array.from({ length: 6 }, (_, index) => <div className="skeleton-card" key={index}><div className="skeleton skeleton-image" /><div className="skeleton skeleton-line short" /><div className="skeleton skeleton-line" /><div className="skeleton skeleton-line medium" /></div>)}</div>
}
