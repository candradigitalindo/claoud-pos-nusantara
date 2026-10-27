export const usageStatuses = {
  over_limit: { label: 'Melebihi batas', tone: 'danger' },
  data_incomplete: { label: 'Data perlu dilengkapi', tone: 'warning' },
  unset: { label: 'Batas belum ditetapkan', tone: 'neutral' },
  no_basis: { label: 'Belum ada dasar pemakaian', tone: 'warning' },
  needs_count: { label: 'Perlu cek fisik', tone: 'warning' },
  within_limit: { label: 'Dalam batas tercatat', tone: 'success' },
  no_activity: { label: 'Tidak ada pemakaian', tone: 'neutral' },
}
export const usageStatus = status => usageStatuses[status] || { label: 'Perlu diperiksa', tone: 'warning' }
export const usageQty = value => value == null ? '—' : Number(value).toLocaleString('id-ID', { maximumFractionDigits: 4 })

const flagLabels = {
  policy_unset: 'PPIC belum menetapkan batas untuk bahan dan gudang ini.',
  no_reference: 'Belum ada acuan pemakaian untuk menghitung batas persentase.',
  stocktake_missing: 'Belum ada opname yang disetujui dalam periode ini.',
  stocktake_shortage: 'Ada kekurangan stok pada penyesuaian hasil opname.',
  stocktake_surplus_not_offset: 'Ada kelebihan opname; jumlah ini tidak menghapus kekurangan stok.',
  manual_adjustment_shortage: 'Ada koreksi stok negatif di luar opname yang perlu dicocokkan dengan bukti.',
  void_waste_included: 'Waste dari void sudah termasuk dalam total waste.',
  usage_over_limit: 'Pemakaian dan koreksi negatif melewati batas PPIC.',
  waste_over_limit: 'Waste melewati batas PPIC.',
  sale_basis_missing: 'Ada pengeluaran penjualan tanpa pasangan transaksi pembayaran yang valid.',
  pending_deductions: 'Ada pencatatan pemakaian POS yang belum berhasil.',
  sales_without_stock_basis: 'Ada produk terbayar yang belum memiliki catatan bahan; periksa tautan resep dan sinkronisasi.',
  production_snapshot_missing: 'Acuan produksi belum lengkap; periksa dokumen dan rincian bahannya.',
  production_movement_missing: 'Dokumen produksi belum cocok dengan catatan bahan keluar.',
  production_documented_basis: 'Produksi langsung memakai jumlah bahan yang tercatat sebagai acuan; verifikasi dengan hitungan fisik.',
  production_plan_snapshot: 'Acuan produksi memakai rincian standar yang disimpan pada work order.',
  breakdown_documented_basis: 'Pemotongan memakai bahan masuk yang tercatat pada dokumen; periksa hasil dan susut secara terpisah.',
  breakdown_snapshot_missing: 'Catatan pemotongan belum cocok dengan dokumen sumber.',
  waste_document_missing: 'Ada pengeluaran waste tanpa dokumen waste yang lengkap.',
  returns_excluded: 'Retur ditampilkan terpisah dari penilaian pemakaian.',
  other_usage_review: 'Ada pemakaian lain yang perlu dicocokkan dengan dokumen sumber.',
}
export const usageFlagLabel = flag => flagLabels[flag] || 'Ada catatan bahan yang perlu diperiksa lebih lanjut.'
