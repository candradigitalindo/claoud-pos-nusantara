// Saldo, stok minimum, dan HPP selalu disimpan dalam satuan dasar.
// Satuan kemasan hanya mengubah cara pengguna memasukkan/melihat jumlah.
export function stockUnitOptions(item) {
  if (!item?.base_unit) return []
  const options = [{ value: item.base_unit, label: item.base_unit, ratio: 1 }]
  const ratio = Number(item.dist_ratio)
  if (item.dist_unit && item.dist_unit !== item.base_unit && Number.isFinite(ratio) && ratio > 0) {
    options.push({ value: item.dist_unit, label: item.dist_unit_label || item.dist_unit, ratio })
  }
  return options
}

export function stockUnitRatio(item, unit) {
  return stockUnitOptions(item).find(option => option.value === unit)?.ratio ?? 1
}

export function preferredStockUnit(item, warehouseType) {
  const options = stockUnitOptions(item)
  return (warehouseType === 'central' ? options.at(-1) : options[0])?.value || ''
}

export function stockUnitLabel(item, unit) {
  return stockUnitOptions(item).find(option => option.value === unit)?.label || unit || 'unit'
}

export function stockQtyText(item, qtyBase, warehouseType) {
  const unit = preferredStockUnit(item, warehouseType)
  const qty = Number(qtyBase || 0) / stockUnitRatio(item, unit)
  return `${new Intl.NumberFormat('id-ID', { maximumFractionDigits: 4 }).format(qty)} ${stockUnitLabel(item, unit)}`
}
