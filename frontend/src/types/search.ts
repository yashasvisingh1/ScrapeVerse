export type SearchOption = {
  brand: string
  item: string
  gender: string
}

export type SearchSelection = SearchOption

export type ProductMetadata = {
  total: number
  lastScrapedAt: string | null
  expiresAt: string | null
  stale: boolean
  refreshInProgress: boolean
  page: number
  pageSize: number
  totalPages: number
}
