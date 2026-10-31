export const PRODUCT_KINDS = [
  { value: 'food', label: 'Makanan' },
  { value: 'beverage', label: 'Minuman' },
  { value: 'activity', label: 'Aktivitas' },
  { value: 'other', label: 'Lainnya' },
]

export function productKindLabel(value) {
  return PRODUCT_KINDS.find(kind => kind.value === value)?.label || 'Belum ditentukan'
}

export function canApplyClassification(row) {
  return row.status === 'suggested' && row.confidence === 'high' &&
    !row.current_kind && !!row.fingerprint &&
    PRODUCT_KINDS.some(kind => kind.value === row.suggested_kind)
}
