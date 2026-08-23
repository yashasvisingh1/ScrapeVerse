import type { Product } from '../types/product'
import { ProductCard } from './ProductCard'

export function ProductGrid({ products }: { products: Product[] }) {
  const prices = products.map((product) => product.currentPrice).filter((price): price is number => price != null)
  const lowest = prices.length ? Math.min(...prices) : null
  return <div className="product-grid">{products.map((product) => <ProductCard key={`${product.retailer}-${product.externalId}-${product.id}`} product={product} cheapest={lowest != null && product.currentPrice === lowest} />)}</div>
}
