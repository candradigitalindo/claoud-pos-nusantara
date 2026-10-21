export const MENU_KINDS = [{ value: 'food', label: 'Makanan' }, { value: 'beverage', label: 'Minuman' }]
export const kindLabel = value => MENU_KINDS.find(x => x.value === value)?.label || 'Belum dikategorikan'
export const DEV_STAGES = [
  { value: 'idea', label: 'Ide' }, { value: 'trial', label: 'Uji coba' },
  { value: 'review', label: 'Review' }, { value: 'approved', label: 'Disetujui' },
  { value: 'launched', label: 'Diluncurkan' }, { value: 'archived', label: 'Arsip' },
]
export const stageLabel = value => DEV_STAGES.find(x => x.value === value)?.label || value
export const costStatusLabel = value => ({ ready: 'Biaya lengkap', no_recipe: 'Tanpa sumber HPP', missing_cost: 'Harga bahan belum lengkap', restricted: 'Resep dibatasi' }[value] || value)
