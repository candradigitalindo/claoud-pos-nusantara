// ── Label & badge bersama modul Gudang ────────────────────────
// Satu sumber untuk tipe movement stok dan status transfer — sebelumnya tiap
// halaman (StockLedger, StockTransfers, WarehouseDashboard) punya salinan sendiri
// yang labelnya saling beda dan tidak lengkap.

export const MOVEMENT_LABELS = {
  purchase_in: 'Pembelian', adjustment: 'Penyesuaian', waste: 'Pemborosan',
  spoiled: 'Rusak', expired: 'Kadaluarsa', return_in: 'Retur Masuk',
  transfer_in: 'Transfer Masuk', transfer_out: 'Transfer Keluar',
  sale: 'Penjualan', production_in: 'Hasil Produksi', production_out: 'Bahan Produksi',
  // Pemotongan: satu bahan (mis. ayam utuh) → beberapa hasil (paha, dada, …).
  breakdown_out: 'Pemotongan (bahan)', breakdown_in: 'Pemotongan (hasil)',
}

export function movementLabel(type) {
  return MOVEMENT_LABELS[type] ?? type
}

// Kelas badge (chip Tailwind) per tipe movement.
export function movementClass(type) {
  const map = {
    purchase_in: 'bg-emerald-100 text-emerald-700',
    transfer_in: 'bg-blue-100 text-blue-700',
    transfer_out: 'bg-orange-100 text-orange-700',
    adjustment: 'bg-gray-100 text-gray-600',
    waste: 'bg-red-100 text-red-600',
    spoiled: 'bg-amber-100 text-amber-700',
    expired: 'bg-rose-100 text-rose-700',
    return_in: 'bg-purple-100 text-purple-700',
    sale: 'bg-cyan-100 text-cyan-700',
    production_in: 'bg-teal-100 text-teal-700',
    production_out: 'bg-yellow-100 text-yellow-700',
    breakdown_in: 'bg-lime-100 text-lime-700',
    breakdown_out: 'bg-pink-100 text-pink-700',
  }
  return map[type] || 'bg-gray-100 text-gray-600'
}

export const TRANSFER_STATUS_LABELS = {
  draft: 'Draft', approved: 'Disetujui', sent: 'Dikirim',
  received: 'Diterima', cancelled: 'Dibatalkan',
}

export function transferStatusLabel(status) {
  return TRANSFER_STATUS_LABELS[status] ?? status
}

// Transfer berasal dari Gudang Induk = permintaan stok outlet: draft-nya
// menunggu Gudang Induk menyetujui & mengirim (role semua outlet).
export function isCentralRequest(transfer) {
  return (transfer?.from_warehouse_type ?? '') === 'central'
}

// Label status untuk satu baris transfer (menyertakan konteks permintaan).
export function transferRowStatusLabel(transfer) {
  if (transfer?.status === 'draft' && isCentralRequest(transfer)) return 'Menunggu Gudang Induk'
  return transferStatusLabel(transfer?.status)
}

// Kelas badge (chip Tailwind) per status transfer.
export function transferStatusClass(status) {
  const map = {
    draft: 'bg-gray-100 text-gray-600',
    approved: 'bg-blue-100 text-blue-700',
    sent: 'bg-amber-100 text-amber-700',
    received: 'bg-emerald-100 text-emerald-700',
    cancelled: 'bg-red-100 text-red-600',
  }
  return map[status] || 'bg-gray-100 text-gray-600'
}

export function formatWarehouseOptionLabel(warehouse) {
  if (!warehouse) return ''

  const name = warehouse.name ?? warehouse.warehouse_name ?? ''
  const type = warehouse.type ?? warehouse.warehouse_type ?? ''
  const outletName = warehouse.outlet_name ?? warehouse.outletName ?? ''
  const code = warehouse.code ?? warehouse.warehouse_code ?? ''

  if (type === 'central') {
    return name ? `${name} · Gudang Induk${code ? ` · ${code}` : ''}` : 'Gudang Induk'
  }

  if (outletName) {
    return `${name} · Outlet ${outletName}${code ? ` · ${code}` : ''}`
  }

  return name ? `${name} · Gudang Outlet${code ? ` · ${code}` : ''}` : 'Gudang Outlet'
}

export function describeWarehouse(warehouse) {
  if (!warehouse) return 'Pilih gudang untuk melihat konteks outlet dan tipenya.'

  const type = warehouse.type ?? warehouse.warehouse_type ?? ''
  const code = warehouse.code ?? warehouse.warehouse_code ?? ''
  const outletName = warehouse.outlet_name ?? warehouse.outletName ?? ''

  if (type === 'central') {
    return `Tipe: Gudang Induk${code ? ` · Kode: ${code}` : ''}`
  }

  const outletInfo = outletName ? `Outlet: ${outletName}` : 'Gudang outlet tanpa outlet terkait'
  return `Tipe: Gudang Outlet · ${outletInfo}${code ? ` · Kode: ${code}` : ''}`
}