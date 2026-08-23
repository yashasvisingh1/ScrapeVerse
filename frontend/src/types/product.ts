export type Product = {
  id: number
  retailer: string
  externalId: string
  title: string
  brand: string
  currentPrice: number | null
  originalPrice: number | null
  currency?: string
  rating: number | null
  reviewCount: number | null
  imageUrl: string
  productUrl: string
  scrapedAt: string | null
}

export type ProductResponse = {
  search: {
    brand: string
    item: string
    gender: string
  }
  products: Product[]
  metadata: import('./search').ProductMetadata
}
