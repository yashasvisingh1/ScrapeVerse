import { request } from './client'
import type { ProductResponse } from '../types/product'
import type { SearchSelection } from '../types/search'

export type ProductParams = SearchSelection & {
  page: number
  pageSize: number
  sort: 'price_asc' | 'price_desc' | 'rating_desc'
  retailer: string
}

export function getProducts(params: ProductParams) {
  const query = new URLSearchParams({
    brand: params.brand,
    item: params.item,
    gender: params.gender,
    page: String(params.page),
    page_size: String(params.pageSize),
    sort: params.sort,
  })
  if (params.retailer) query.set('retailer', params.retailer)
  return request<ProductResponse>(`/api/v1/products?${query}`)
}

export function refreshProducts(selection: SearchSelection) {
  return request<{ status: 'refresh_started' | 'refresh_already_in_progress' }>('/api/v1/products/refresh', {
    method: 'POST',
    body: JSON.stringify(selection),
  })
}
