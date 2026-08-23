import { useEffect, useState } from 'react'
import { getProducts, refreshProducts } from '../api/products'
import { getSearches } from '../api/searches'
import { EmptyState } from '../components/EmptyState'
import { LoadingSkeleton } from '../components/LoadingSkeleton'
import { Pagination } from '../components/Pagination'
import { ProductGrid } from '../components/ProductGrid'
import { SearchFilters } from '../components/SearchFilters'
import { StaleDataBanner } from '../components/StaleDataBanner'
import type { ProductResponse } from '../types/product'
import type { SearchOption, SearchSelection } from '../types/search'

const emptySelection: SearchSelection = { brand: '', item: '', gender: '' }
const initialParams = { page: 1, pageSize: 20, sort: 'price_asc' as const, retailer: '' }

export function Home() {
  const [options, setOptions] = useState<SearchOption[]>([])
  const [selection, setSelection] = useState(emptySelection)
  const [activeSelection, setActiveSelection] = useState<SearchSelection | null>(null)
  const [params, setParams] = useState(initialParams)
  const [response, setResponse] = useState<ProductResponse | null>(null)
  const [loadingOptions, setLoadingOptions] = useState(true)
  const [loadingProducts, setLoadingProducts] = useState(false)
  const [error, setError] = useState('')
  const [refreshNotice, setRefreshNotice] = useState('')
  const [refreshAttempt, setRefreshAttempt] = useState(0)

  useEffect(() => {
    getSearches().then((searches) => {
      setOptions(searches)
    }).catch(() => setError('We could not load the available searches.')).finally(() => setLoadingOptions(false))
  }, [])

  useEffect(() => {
    if (!activeSelection) return
    setLoadingProducts(true)
    setError('')
    getProducts({ ...activeSelection, ...params }).then(setResponse).catch((reason: Error) => setError(reason.message || 'We could not load products.')).finally(() => setLoadingProducts(false))
  }, [activeSelection, params, refreshAttempt])

  useEffect(() => {
    if (!response?.metadata.refreshInProgress || refreshAttempt >= 5 || !activeSelection) return
    const timer = window.setTimeout(() => setRefreshAttempt((attempt) => attempt + 1), 4000)
    return () => window.clearTimeout(timer)
  }, [response, refreshAttempt, activeSelection])

  function submitSearch() {
    setParams(initialParams)
    setRefreshNotice('')
    setActiveSelection(selection)
  }

  async function handleRefresh() {
    if (!activeSelection) return
    setRefreshNotice('')
    try {
      const result = await refreshProducts(activeSelection)
      setRefreshNotice(result.status === 'refresh_started' ? 'Price refresh started.' : 'Prices are already being updated.')
      setRefreshAttempt((attempt) => attempt + 1)
    } catch {
      setRefreshNotice('We could not start a price refresh. Please try again.')
    }
  }

  const metadata = response?.metadata
  return <div className="app-shell">
    <header className="topbar"><a className="brandmark" href="/"><span className="brandmark-icon" aria-hidden="true">↗</span><span>Compare<span className="brandmark-accent">Cart</span></span></a><span className="header-note">Live prices, one clear view</span></header>
    <main>
      <section className="intro"><p className="eyebrow">SHOP WITH SIGNAL</p><h1>Find the sharper price.</h1><p className="intro-copy">Compare products across India&apos;s most-loved retailers, with every result backed by a recent refresh.</p></section>
      <section className="filter-panel" aria-labelledby="search-heading"><div className="panel-heading"><div><p className="eyebrow">START A COMPARISON</p><h2 id="search-heading">Choose your essentials</h2></div><span className="catalog-count">{options.length} supported searches</span></div>{loadingOptions ? <div className="filters-loading">Loading the catalog...</div> : <SearchFilters options={options} value={selection} onChange={setSelection} onSubmit={submitSearch} disabled={loadingOptions} />}</section>
      {error && <div className="error-panel" role="alert"><span aria-hidden="true">!</span><div><strong>Something went wrong</strong><p>{error}</p></div><button type="button" onClick={() => { setError(''); setRefreshAttempt((attempt) => attempt + 1) }}>Retry</button></div>}
      {!activeSelection && !error && <section className="welcome-state"><span className="welcome-mark" aria-hidden="true">⌁</span><h2>Your next find starts here.</h2><p>Select a brand, item, and gender to see what the market is asking today.</p></section>}
      {activeSelection && <section className="results-section" aria-labelledby="results-heading"><div className="results-heading"><div><p className="eyebrow">YOUR COMPARISON</p><h2 id="results-heading">{activeSelection.brand} {activeSelection.item} <span>· {activeSelection.gender}</span></h2></div><button className="refresh-button" type="button" onClick={handleRefresh} disabled={Boolean(metadata?.refreshInProgress)}><span aria-hidden="true">↻</span>{metadata?.refreshInProgress ? 'Updating...' : 'Refresh prices'}</button></div>{refreshNotice && <p className="refresh-notice" role="status">{refreshNotice}</p>}{metadata?.stale && <StaleDataBanner refreshing={metadata.refreshInProgress} />}<div className="results-toolbar"><p>{loadingProducts ? 'Updating comparison...' : <><strong>{metadata?.total ?? 0}</strong> products found</>}</p><div className="toolbar-controls"><label>Sort <select value={params.sort} onChange={(event) => setParams({ ...params, page: 1, sort: event.target.value as typeof params.sort })}><option value="price_asc">Price: low to high</option><option value="price_desc">Price: high to low</option><option value="rating_desc">Rating: highest</option></select></label><label>Retailer <select value={params.retailer} onChange={(event) => setParams({ ...params, page: 1, retailer: event.target.value })}><option value="">All retailers</option><option value="amazon">Amazon</option><option value="myntra">Myntra</option><option value="ajio">AJIO</option></select></label></div></div>{loadingProducts && !response ? <LoadingSkeleton /> : response?.products.length ? <ProductGrid products={response.products} /> : <EmptyState refreshing={Boolean(metadata?.refreshInProgress)} />}<Pagination page={metadata?.page ?? 1} totalPages={metadata?.totalPages ?? 0} onChange={(page) => setParams({ ...params, page })} /></section>}
    </main>
    <footer><span>CompareCart</span><span>Prices sourced from Amazon, Myntra &amp; AJIO</span></footer>
  </div>
}
