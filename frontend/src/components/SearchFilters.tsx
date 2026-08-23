import type { SearchOption, SearchSelection } from '../types/search'

type Props = {
  options: SearchOption[]
  value: SearchSelection
  onChange: (selection: SearchSelection) => void
  onSubmit: () => void
  disabled: boolean
}

export function SearchFilters({ options, value, onChange, onSubmit, disabled }: Props) {
  const brands = [...new Set(options.map((option) => option.brand))]
  const items = [...new Set(options.filter((option) => !value.brand || option.brand === value.brand).map((option) => option.item))]
  const genders = [...new Set(options.filter((option) => (!value.brand || option.brand === value.brand) && (!value.item || option.item === value.item)).map((option) => option.gender))]

  return (
    <form className="search-filters" onSubmit={(event) => { event.preventDefault(); onSubmit() }}>
      <label>
        <span>Brand</span>
        <select value={value.brand} onChange={(event) => onChange({ ...value, brand: event.target.value, item: '', gender: '' })} disabled={disabled}>
          <option value="">Choose a brand</option>
          {brands.map((brand) => <option key={brand} value={brand}>{brand === 'all' ? 'All brands' : brand}</option>)}
        </select>
      </label>
      <label>
        <span>Item <b>*</b></span>
        <select value={value.item} onChange={(event) => onChange({ ...value, item: event.target.value, gender: '' })} disabled={disabled} required>
          <option value="">Choose an item</option>
          {items.map((item) => <option key={item} value={item}>{item}</option>)}
        </select>
      </label>
      <label>
        <span>Gender</span>
        <select value={value.gender} onChange={(event) => onChange({ ...value, gender: event.target.value })} disabled={disabled}>
          <option value="">Choose a gender</option>
          {genders.map((gender) => <option key={gender} value={gender}>{gender === 'all' ? 'All genders' : gender}</option>)}
        </select>
      </label>
      <button className="primary-button" type="submit" disabled={disabled || !value.item}>
        <span aria-hidden="true">↗</span> Compare prices
      </button>
    </form>
  )
}
