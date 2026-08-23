type Props = { page: number; totalPages: number; onChange: (page: number) => void }

export function Pagination({ page, totalPages, onChange }: Props) {
  if (totalPages <= 1) return null
  return <nav className="pagination" aria-label="Product pages"><button type="button" onClick={() => onChange(page - 1)} disabled={page === 1}>← Previous</button><span>Page <strong>{page}</strong> of {totalPages}</span><button type="button" onClick={() => onChange(page + 1)} disabled={page === totalPages}>Next →</button></nav>
}
