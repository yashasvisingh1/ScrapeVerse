import { request } from './client'
import type { SearchOption } from '../types/search'

export function getSearches() {
  return request<SearchOption[]>('/api/v1/searches')
}
