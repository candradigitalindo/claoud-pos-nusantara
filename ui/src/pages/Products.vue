<!--
  Products.vue — Global products & categories management (all outlets)
  Route: /products
-->
<template>
  <div class="prod-root">

    <!-- Page header -->
    <div class="page-header">
      <div>
        <h1 class="page-title">Produk &amp; Kategori</h1>
        <p class="page-sub">Lihat dan kelola data produk dari seluruh outlet</p>
      </div>
    </div>

    <!-- Tab bar -->
    <div class="tab-bar">
      <button
        :class="['tab-btn', tab === 'products' ? 'tab-btn--active' : '']"
        @click="switchTab('products')"
      >
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10"/>
        </svg>
        Produk
        <span v-if="!loadingProd" class="tab-count">{{ totalProd }}</span>
      </button>
      <button
        :class="['tab-btn', tab === 'categories' ? 'tab-btn--active' : '']"
        @click="switchTab('categories')"
      >
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M3 3h7v7H3zM14 3h7v7h-7zM14 14h7v7h-7zM3 14h7v7H3z"/>
        </svg>
        Kategori
        <span v-if="!loadingCat" class="tab-count">{{ totalCat }}</span>
      </button>
    </div>

    <!-- Toolbar -->
    <div class="toolbar">
      <div class="search-wrap">
        <svg class="search-ico" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
        <input v-model="search" class="search-input" :placeholder="tab === 'products' ? 'Cari produk, kode, atau kategori…' : 'Cari kategori…'" type="search" />
      </div>
      <div class="outlet-select-wrap" style="min-width:200px">
        <SearchSelect
          v-model="selectedOutlet"
          :options="outletFilterOptions"
          placeholder="Semua Outlet"
          searchPlaceholder="Cari outlet…"
          @change="onOutletChange"
        />
      </div>
      <button class="btn-add" @click="tab === 'products' ? openCreateProd() : openCreateCat()">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        Tambah {{ tab === 'products' ? 'Produk' : 'Kategori' }}
      </button>
    </div>

    <!-- ── PRODUCTS TABLE ── -->
    <div v-if="tab === 'products'" class="table-panel">
      <!-- Mobile cards -->
      <div class="mobile-cards">
        <div v-if="loadingProd" class="m-state">Memuat…</div>
        <div v-else-if="!products.length" class="m-state">{{ search ? 'Produk tidak ditemukan' : 'Belum ada produk' }}</div>
        <div v-else v-for="p in products" :key="p.id" class="mcard">
          <div class="mcard-photo">
            <img v-if="p.photo_url" :src="p.photo_url" :alt="p.name" loading="lazy" />
            <div v-else class="row-avatar">{{ p.name?.charAt(0)?.toUpperCase() ?? 'P' }}</div>
          </div>
          <div class="mcard-body">
            <p class="row-name">{{ p.name }}</p>
            <p class="row-desc">{{ productRecipeLabel(p) }}</p>
            <p class="mcard-meta">
              <span v-if="p.code" class="row-code">{{ p.code }}</span>
              <span v-if="p.category_name" class="tag-category">{{ p.category_name }}</span>
            </p>
            <p class="row-price">{{ formatRupiah(p.price) }}</p>
            <p v-if="!selectedOutlet" class="row-outlet">{{ p.outlet_name || '—' }}</p>
          </div>
          <div class="mcard-acts">
            <AppActionButton icon="edit" label="Edit" @click="openEditProd(p)" />
            <AppActionButton icon="delete" label="Hapus" @click="openDelete(p, 'product')" />
          </div>
        </div>
      </div>
      <div class="table-scroll desktop-only">
        <table class="otable">
          <thead>
            <tr>
              <th class="th-avatar"></th>
              <th>Kode</th>
              <th>Nama Produk</th>
              <th>Kategori</th>
              <th>Harga</th>
              <th v-if="!selectedOutlet">Outlet</th>
              <th>Diperbarui</th>
              <th class="th-actions"></th>
            </tr>
          </thead>
          <tbody>
            <template v-if="loadingProd">
              <tr v-for="n in 8" :key="n" class="tr-skeleton">
                <td><div class="skel skel-avatar"/></td>
                <td><div class="skel skel-w60"/><div class="skel skel-w30 skel-mt"/></td>
                <td><div class="skel skel-w44"/></td>
                <td><div class="skel skel-w44"/></td>
                <td><div class="skel skel-w30"/></td>
                <td v-if="!selectedOutlet"><div class="skel skel-w60"/></td>
                <td><div class="skel skel-w50"/></td>
                <td></td>
              </tr>
            </template>
            <template v-else-if="products.length">
              <tr v-for="p in products" :key="p.id" class="tr-data">
                <td class="td-avatar">
                  <div class="row-avatar">{{ p.name?.charAt(0)?.toUpperCase() ?? 'P' }}</div>
                </td>
                <td>
                  <span v-if="p.code" class="row-code">{{ p.code }}</span>
                  <span v-else class="row-muted">—</span>
                </td>
                <td>
                  <span class="row-name">{{ p.name }}</span>
                  <span class="row-desc">{{ productRecipeLabel(p) }}</span>
                  <span v-if="p.description" class="row-desc">{{ p.description }}</span>
                </td>
                <td>
                  <span v-if="p.category_name" class="tag-category">{{ p.category_name }}</span>
                  <span v-else class="row-muted">—</span>
                </td>
                <td>
                  <span class="row-price">{{ formatRupiah(p.price) }}</span>
                </td>
                <td v-if="!selectedOutlet">
                  <span class="row-outlet">{{ p.outlet_name || '—' }}</span>
                </td>
                <td>
                  <span class="row-muted">{{ fmtDate(p.updated_at) }}</span>
                </td>
                <td class="td-actions">
                  <div class="action-btns">
                    <AppActionButton icon="edit" label="Edit" @click="openEditProd(p)" />
                    <AppActionButton icon="delete" label="Hapus" @click="openDelete(p, 'product')" />
                  </div>
                </td>
              </tr>
            </template>
            <tr v-else>
              <td :colspan="selectedOutlet ? 7 : 8">
                <div class="empty-state">
                  <div class="empty-icon">
                    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10"/>
                    </svg>
                  </div>
                  <p class="empty-title">{{ search ? 'Produk tidak ditemukan' : 'Belum ada produk' }}</p>
                  <p class="empty-sub">{{ search ? 'Coba ubah kata kunci pencarian.' : 'Produk akan muncul setelah outlet melakukan sync.' }}</p>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="!loadingProd && totalProd > PAGE_SIZE" class="table-footer">
        <AppPagination v-model="pageProd" :total="totalProd" :perPage="PAGE_SIZE" />
      </div>
    </div>

    <!-- ── CATEGORIES TABLE ── -->
    <div v-if="tab === 'categories'" class="table-panel">
      <!-- Mobile cards -->
      <div class="mobile-cards">
        <div v-if="loadingCat" class="m-state">Memuat…</div>
        <div v-else-if="!categories.length" class="m-state">{{ search ? 'Kategori tidak ditemukan' : 'Belum ada kategori' }}</div>
        <div v-else v-for="c in categories" :key="c.id" class="mcard">
          <div class="mcard-photo"><div class="row-avatar ava-cat">{{ c.name?.charAt(0)?.toUpperCase() ?? 'K' }}</div></div>
          <div class="mcard-body">
            <p class="row-name">{{ c.name }}</p>
            <p class="mcard-meta"><span v-if="c.code_prefix" class="row-code">{{ c.code_prefix }}</span></p>
            <p v-if="!selectedOutlet" class="row-outlet">{{ c.outlet_name || '—' }}</p>
          </div>
          <div class="mcard-acts">
            <AppActionButton icon="edit" label="Edit" @click="openEditCat(c)" />
            <AppActionButton icon="delete" label="Hapus" @click="openDelete(c, 'category')" />
          </div>
        </div>
      </div>
      <div class="table-scroll desktop-only">
        <table class="otable">
          <thead>
            <tr>
              <th class="th-avatar"></th>
              <th>Nama Kategori</th>
              <th>Kode Prefix</th>
              <th v-if="!selectedOutlet">Outlet</th>
              <th>Diperbarui</th>
              <th class="th-actions"></th>
            </tr>
          </thead>
          <tbody>
            <template v-if="loadingCat">
              <tr v-for="n in 6" :key="n" class="tr-skeleton">
                <td><div class="skel skel-avatar"/></td>
                <td><div class="skel skel-w60"/></td>
                <td><div class="skel skel-w30"/></td>
                <td v-if="!selectedOutlet"><div class="skel skel-w60"/></td>
                <td><div class="skel skel-w50"/></td>
                <td></td>
              </tr>
            </template>
            <template v-else-if="categories.length">
              <tr v-for="c in categories" :key="c.id" class="tr-data">
                <td class="td-avatar">
                  <div class="row-avatar ava-cat">{{ c.name?.charAt(0)?.toUpperCase() ?? 'K' }}</div>
                </td>
                <td>
                  <span class="row-name">{{ c.name }}</span>
                </td>
                <td>
                  <span v-if="c.code_prefix" class="row-code">{{ c.code_prefix }}</span>
                  <span v-else class="row-muted">—</span>
                </td>
                <td v-if="!selectedOutlet">
                  <span class="row-outlet">{{ c.outlet_name || '—' }}</span>
                </td>
                <td>
                  <span class="row-muted">{{ fmtDate(c.updated_at) }}</span>
                </td>
                <td class="td-actions">
                  <div class="action-btns">
                    <AppActionButton icon="edit" label="Edit" @click="openEditCat(c)" />
                    <AppActionButton icon="delete" label="Hapus" @click="openDelete(c, 'category')" />
                  </div>
                </td>
              </tr>
            </template>
            <tr v-else>
              <td :colspan="selectedOutlet ? 5 : 6">
                <div class="empty-state">
                  <div class="empty-icon">
                    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M3 3h7v7H3zM14 3h7v7h-7zM14 14h7v7h-7zM3 14h7v7H3z"/>
                    </svg>
                  </div>
                  <p class="empty-title">{{ search ? 'Kategori tidak ditemukan' : 'Belum ada kategori' }}</p>
                  <p class="empty-sub">{{ search ? 'Coba ubah kata kunci.' : 'Kategori akan muncul setelah outlet melakukan sync.' }}</p>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="!loadingCat && totalCat > PAGE_SIZE" class="table-footer">
        <AppPagination v-model="pageCat" :total="totalCat" :perPage="PAGE_SIZE" />
      </div>
    </div>

    <!-- ── Product Modal ──────────────────────────────────── -->
    <Teleport to="body">
      <div v-if="showProdModal" class="modal-overlay" @click.self="showProdModal = false">
        <div class="modal-card">
          <div class="modal-header">
            <span class="modal-title">{{ editingProd ? 'Edit Produk' : 'Tambah Produk' }}</span>
            <button class="modal-close" @click="showProdModal = false">&times;</button>
          </div>
          <form class="modal-body" @submit.prevent="saveProd">
            <div v-if="!editingProd" class="form-group">
              <label>Outlet <span class="req">*</span></label>
              <SearchSelect
                v-model="formProd.outlet_id"
                :options="outlets"
                placeholder="Pilih outlet…"
                searchPlaceholder="Cari outlet…"
              />
            </div>
            <div class="form-group">
              <label>Nama Produk <span class="req">*</span></label>
              <input v-model="formProd.name" class="form-input" placeholder="Contoh: Nasi Goreng" required />
            </div>
            <div class="form-group">
              <label>Kategori</label>
              <SearchSelect
                v-model="formProd.category_id"
                :options="categoryModalOptions"
                placeholder="Pilih kategori…"
                searchPlaceholder="Cari kategori…"
                @change="onCategorySelect"
              />
              <span v-if="!formProdOutletId" class="form-hint">Pilih outlet untuk memuat daftar kategori</span>
            </div>
            <div class="form-group">
              <label>Kode Produk</label>
              <input v-model="formProd.code" class="form-input form-input--code" placeholder="Otomatis dari nama produk" readonly />
              <span class="form-hint">Kode dibuat otomatis dari huruf pertama nama produk</span>
            </div>
            <div class="form-group">
              <label>Deskripsi</label>
              <input v-model="formProd.description" class="form-input" placeholder="Deskripsi singkat produk" />
            </div>
            <div class="form-row">
              <div class="form-group">
                <label>Harga <span class="req">*</span></label>
                <input :value="priceDisplay" @input="onPriceInput" class="form-input" type="text" inputmode="numeric" placeholder="Rp 0" required />
              </div>
            </div>
            <div class="form-group">
              <label>Resep (opsional)</label>
              <SearchSelect
                v-model="formProd.recipe_master_id"
                :options="recipeModalOptions"
                :disabled="!formProdOutletId || loadingRecipes"
                :placeholder="loadingRecipes ? 'Memuat resep…' : 'Tanpa resep'"
                searchPlaceholder="Cari resep…"
                @change="recipeSelectionChanged = true"
              />
              <span v-if="!formProdOutletId" class="form-hint">Pilih outlet untuk melihat resep yang tersedia.</span>
              <span v-else-if="recipeLoadError" class="form-hint text-red-600" role="alert">
                {{ recipeLoadError }}
                <button type="button" class="underline" @click="fetchRecipeOptions(formProdOutletId)">Coba lagi</button>
              </span>
              <span v-else-if="currentRecipeUnavailable" class="form-hint">Resep tertaut tidak tersedia dalam pilihan outlet. Anda dapat mempertahankan atau melepas tautannya.</span>
              <span v-else-if="formProd.recipe_master_id" class="form-hint">Bahan dipotong sesuai resep saat penjualan tersinkron. Takaran resep berlaku untuk 1 produk terjual.</span>
              <span v-else-if="!recipeSelectionChanged && editingProd?.stock_type === 'single'" class="form-hint">Produk ini memakai stok langsung. Memilih resep akan mengganti pengaturan tersebut.</span>
              <span v-else-if="!recipeSelectionChanged && editingProd?.stock_type === 'recipe'" class="form-hint">Produk ini memakai resep khusus. Pilih master resep untuk menggantinya, atau pilih Tanpa resep untuk melepasnya.</span>
              <span v-else class="form-hint">Produk bisa dijual tanpa resep. Tidak ada pemotongan bahan berdasarkan resep.</span>
            </div>
            <div class="form-group">
              <label>Foto Produk</label>
              <div class="photo-row">
                <div class="photo-thumb">
                  <img v-if="photoPreview || formProd.photo_url" :src="photoPreview || formProd.photo_url" alt="Foto produk" />
                  <span v-else class="photo-empty">Tidak ada foto</span>
                </div>
                <div class="photo-actions">
                  <input ref="photoInput" type="file" accept="image/*" style="display:none" @change="onPickPhoto" />
                  <button type="button" class="btn-cancel" @click="photoInput?.click()">{{ (photoPreview || formProd.photo_url) ? 'Ganti Foto' : 'Pilih Foto' }}</button>
                  <button v-if="photoPreview || formProd.photo_url" type="button" class="btn-photo-del" @click="clearPhoto">Hapus</button>
                </div>
              </div>
              <span class="form-hint">JPG/PNG/WEBP, maks 10MB. Tampil di halaman reservasi publik.</span>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn-cancel" @click="showProdModal = false">Batal</button>
              <button type="submit" class="btn-save" :disabled="savingProd">
                {{ savingProd ? 'Menyimpan…' : (editingProd ? 'Simpan Perubahan' : 'Tambah Produk') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- ── Category Modal ─────────────────────────────────── -->
    <Teleport to="body">
      <div v-if="showCatModal" class="modal-overlay" @click.self="showCatModal = false">
        <div class="modal-card">
          <div class="modal-header">
            <span class="modal-title">{{ editingCat ? 'Edit Kategori' : 'Tambah Kategori' }}</span>
            <button class="modal-close" @click="showCatModal = false">&times;</button>
          </div>
          <form class="modal-body" @submit.prevent="saveCat">
            <div v-if="!editingCat" class="form-group">
              <label>Outlet <span class="req">*</span></label>
              <SearchSelect
                v-model="formCat.outlet_id"
                :options="outlets"
                placeholder="Pilih outlet…"
                searchPlaceholder="Cari outlet…"
              />
            </div>
            <div class="form-group">
              <label>Nama Kategori <span class="req">*</span></label>
              <input v-model="formCat.name" class="form-input" placeholder="Contoh: Makanan" required />
            </div>
            <div class="form-group">
              <label>Kode Prefix</label>
              <input v-model="formCat.code_prefix" class="form-input" placeholder="Auto-generate jika kosong" maxlength="10" />
            </div>
            <div class="modal-footer">
              <button type="button" class="btn-cancel" @click="showCatModal = false">Batal</button>
              <button type="submit" class="btn-save" :disabled="savingCat">
                {{ savingCat ? 'Menyimpan…' : (editingCat ? 'Simpan Perubahan' : 'Tambah Kategori') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- ── Delete Confirm ─────────────────────────────────── -->
    <Teleport to="body">
      <div v-if="showDeleteConfirm" class="modal-overlay" @click.self="showDeleteConfirm = false">
        <div class="modal-card modal-card--sm">
          <div class="modal-header">
            <span class="modal-title">Konfirmasi Hapus</span>
            <button class="modal-close" @click="showDeleteConfirm = false">&times;</button>
          </div>
          <div class="modal-body">
            <p class="del-text">Hapus <strong>{{ deleteTarget?.name }}</strong>? Data akan ditandai dihapus dan tidak dapat dipulihkan dari sini.</p>
            <div class="modal-footer">
              <button type="button" class="btn-cancel" @click="showDeleteConfirm = false">Batal</button>
              <button type="button" class="btn-danger" :disabled="deleting" @click="confirmDelete">
                {{ deleting ? 'Menghapus…' : 'Hapus' }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, watch, onMounted, computed } from 'vue'
import { productsApi } from '@/api/products.js'
import { outletsApi }  from '@/api/outlets.js'
import { formatRupiah, formatDateTime } from '@/utils/format.js'
import { PAGE_SIZE }    from '@/utils/constants.js'
import AppPagination from '@/components/ui/AppPagination.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'

// ── State ────────────────────────────────────────────────────
const tab            = ref('products')
const search         = ref('')
const selectedOutlet = ref('')
const outlets        = ref([])

// Products
const products   = ref([])
const totalProd  = ref(0)
const pageProd   = ref(1)
const loadingProd = ref(false)

// Categories
const categories = ref([])
const totalCat   = ref(0)
const pageCat    = ref(1)
const loadingCat = ref(false)

// ── Rupiah input helpers ──────────────────────────────────
const priceDisplay = ref('')
function fmtRupiahInput(val) {
  const d = String(val ?? '').replace(/[^\d]/g, '')
  if (!d) return ''
  return 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(d))
}
function parseRupiahInput(val) {
  const d = String(val || '').replace(/[^\d]/g, '')
  return d ? parseInt(d, 10) : 0
}
function onPriceInput(e) {
  const num = parseRupiahInput(e.target.value)
  formProd.value.price = num
  priceDisplay.value = num ? fmtRupiahInput(num) : ''
}

// ── CRUD — Product ────────────────────────────────────────
const showProdModal = ref(false)
const editingProd   = ref(null)
const savingProd    = ref(false)
const formProd      = ref({ outlet_id: '', name: '', code: '', description: '', category_id: '', category_name: '', price: 0, photo_url: '', recipe_master_id: '' })
const recipeOptions = ref([])
const loadingRecipes = ref(false)
const recipeLoadError = ref('')
const recipeSelectionChanged = ref(false)
let recipeRequest = 0
const photoFile     = ref(null)
const photoPreview  = ref('')
const photoRemoved  = ref(false)
const photoInput    = ref(null)
const outletCategories = ref([])  // categories for the selected outlet in modal
const formProdOutletId = computed(() => editingProd.value ? editingProd.value.outlet_id : formProd.value.outlet_id)

// Options for SearchSelect components
const outletFilterOptions = computed(() => [{ id: '', name: 'Semua Outlet' }, ...outlets.value])
const categoryModalOptions = computed(() => [{ id: '', name: 'Tanpa Kategori' }, ...outletCategories.value])
const recipeModalOptions = computed(() => {
  const options = [{ id: '', name: 'Tanpa resep' }, ...recipeOptions.value]
  const current = editingProd.value
  if (current?.recipe_master_id && !options.some(r => r.id === current.recipe_master_id)) {
    options.push({ id: current.recipe_master_id, name: current.recipe_master_name || 'Resep tertaut' })
  }
  return options
})
const currentRecipeUnavailable = computed(() => !loadingRecipes.value && !recipeLoadError.value &&
  formProd.value.recipe_master_id && formProd.value.recipe_master_id === editingProd.value?.recipe_master_id &&
  !recipeOptions.value.some(r => r.id === formProd.value.recipe_master_id))

function productRecipeLabel(product) {
  if (product.recipe_master_id) return `Resep: ${product.recipe_master_name || 'Resep tertaut'}`
  if (product.stock_type === 'single') return 'Stok langsung'
  if (product.stock_type === 'recipe') return 'Resep khusus produk'
  return 'Tanpa resep'
}

// ── CRUD — Category ───────────────────────────────────────
const showCatModal = ref(false)
const editingCat   = ref(null)
const savingCat    = ref(false)
const formCat      = ref({ outlet_id: '', name: '', code_prefix: '' })

// ── Delete confirm ─────────────────────────────────────────
const showDeleteConfirm = ref(false)
const deleteTarget      = ref(null)
const deleteType        = ref('')   // 'product' | 'category'
const deleting          = ref(false)

// ── Debounced search watcher ─────────────────────────────────
let _searchTimer = null
watch(search, () => {
  clearTimeout(_searchTimer)
  _searchTimer = setTimeout(() => {
    pageProd.value = 1
    pageCat.value  = 1
    fetchProducts()
    fetchCategories()
  }, 350)
})

// ── Fetch ────────────────────────────────────────────────────
async function fetchProducts() {
  loadingProd.value = true
  try {
    const res = await productsApi.listProducts({
      outlet_id: selectedOutlet.value || undefined,
      search: search.value.trim() || undefined,
      page: pageProd.value,
      limit: PAGE_SIZE,
    })
    products.value  = res.items ?? []
    totalProd.value = res.total ?? 0
  } catch {
    products.value  = []
    totalProd.value = 0
  } finally {
    loadingProd.value = false
  }
}

async function fetchCategories() {
  loadingCat.value = true
  try {
    const res = await productsApi.listCategories({
      outlet_id: selectedOutlet.value || undefined,
      search: search.value.trim() || undefined,
      page: pageCat.value,
      limit: PAGE_SIZE,
    })
    categories.value = res.items ?? []
    totalCat.value   = res.total ?? 0
  } catch {
    categories.value = []
    totalCat.value   = 0
  } finally {
    loadingCat.value = false
  }
}

async function fetchOutlets() {
  try {
    const data = await outletsApi.myOutlets()
    outlets.value = data?.outlets ?? data ?? []
  } catch { outlets.value = [] }
}

function onOutletChange() {
  pageProd.value = 1
  pageCat.value  = 1
  fetchProducts()
  fetchCategories()
}

function switchTab(t) {
  tab.value    = t
  search.value = ''
}

const fmtDate = formatDateTime

// ── CRUD handlers ─────────────────────────────────────────
// Fetch categories for a specific outlet (used in product modal)
async function fetchOutletCategories(outletId) {
  if (!outletId) { outletCategories.value = []; return }
  try {
    const res = await productsApi.listCategories({ outlet_id: outletId, limit: 100 })
    outletCategories.value = res.items ?? []
  } catch { outletCategories.value = [] }
}

async function fetchRecipeOptions(outletId) {
  const request = ++recipeRequest
  recipeOptions.value = []
  recipeLoadError.value = ''
  loadingRecipes.value = !!outletId
  if (!outletId) return
  try {
    const result = await productsApi.recipeOptions(outletId)
    if (request === recipeRequest) recipeOptions.value = result || []
  } catch (e) {
    if (request === recipeRequest) recipeLoadError.value = e?.message || 'Gagal memuat pilihan resep.'
  } finally {
    if (request === recipeRequest) loadingRecipes.value = false
  }
}

// When outlet changes in the product form, re-fetch categories
watch(formProdOutletId, (newId) => {
  fetchOutletCategories(newId)
  fetchRecipeOptions(newId)
  // Reset category when outlet changes (only for new products)
  if (!editingProd.value) {
    formProd.value.category_id = ''
    formProd.value.category_name = ''
    formProd.value.code = ''
    formProd.value.recipe_master_id = ''
    recipeSelectionChanged.value = false
  }
})

function onCategorySelect() {
  const cat = outletCategories.value.find(c => c.id === formProd.value.category_id)
  formProd.value.category_name = cat ? cat.name : ''
  // code will be auto-generated server-side from product name
  formProd.value.code = ''
}

function resetPhoto() {
  photoFile.value = null
  photoPreview.value = ''
  photoRemoved.value = false
  if (photoInput.value) photoInput.value.value = ''
}
function onPickPhoto(e) {
  const file = e.target.files?.[0]
  if (!file) return
  photoFile.value = file
  photoPreview.value = URL.createObjectURL(file)
  photoRemoved.value = false
}
function clearPhoto() {
  photoFile.value = null
  photoPreview.value = ''
  formProd.value.photo_url = ''
  photoRemoved.value = true
  if (photoInput.value) photoInput.value.value = ''
}

function openCreateProd() {
  editingProd.value = null
  formProd.value = { outlet_id: selectedOutlet.value, name: '', code: '', description: '', category_id: '', category_name: '', price: 0, photo_url: '', recipe_master_id: '' }
  recipeSelectionChanged.value = false
  priceDisplay.value = ''
  resetPhoto()
  fetchOutletCategories(selectedOutlet.value)
  fetchRecipeOptions(selectedOutlet.value)
  showProdModal.value = true
}
function openEditProd(p) {
  editingProd.value = p
  formProd.value = { outlet_id: p.outlet_id, name: p.name, code: p.code ?? '', description: p.description ?? '', category_id: p.category_id ?? '', category_name: p.category_name ?? '', price: p.price, photo_url: p.photo_url ?? '', recipe_master_id: p.recipe_master_id || '' }
  recipeSelectionChanged.value = false
  priceDisplay.value = p.price ? fmtRupiahInput(p.price) : ''
  resetPhoto()
  fetchOutletCategories(p.outlet_id)
  fetchRecipeOptions(p.outlet_id)
  showProdModal.value = true
}
async function saveProd() {
  savingProd.value = true
  try {
    let productId = editingProd.value?.id
    if (editingProd.value) {
      await productsApi.updateProduct(editingProd.value.id, {
        name: formProd.value.name, code: formProd.value.code, description: formProd.value.description,
        category_id: formProd.value.category_id, category_name: formProd.value.category_name,
        price: formProd.value.price,
        ...(recipeSelectionChanged.value ? { recipe_master_id: formProd.value.recipe_master_id } : {}),
      })
    } else {
      const created = await productsApi.createProduct({ ...formProd.value })
      productId = created?.id || created?.data?.id
    }
    // Foto: upload bila ada file baru; hapus bila ditandai dihapus.
    if (productId && photoFile.value) {
      await productsApi.uploadPhoto(productId, photoFile.value)
    } else if (productId && photoRemoved.value && editingProd.value) {
      await productsApi.removePhoto(productId)
    }
    showProdModal.value = false
    await fetchProducts()
  } catch (e) {
    alert(e?.message || e?.response?.data?.error || 'Gagal menyimpan produk')
  } finally {
    savingProd.value = false
  }
}

function openCreateCat() {
  editingCat.value = null
  formCat.value = { outlet_id: selectedOutlet.value, name: '', code_prefix: '' }
  showCatModal.value = true
}
function openEditCat(c) {
  editingCat.value = c
  formCat.value = { outlet_id: c.outlet_id, name: c.name, code_prefix: c.code_prefix ?? '' }
  showCatModal.value = true
}
async function saveCat() {
  savingCat.value = true
  try {
    if (editingCat.value) {
      await productsApi.updateCategory(editingCat.value.id, { name: formCat.value.name, code_prefix: formCat.value.code_prefix })
    } else {
      await productsApi.createCategory({ ...formCat.value })
    }
    showCatModal.value = false
    await fetchCategories()
  } catch (e) {
    alert(e?.response?.data?.error || 'Gagal menyimpan kategori')
  } finally {
    savingCat.value = false
  }
}

function openDelete(item, type) {
  deleteTarget.value = item
  deleteType.value   = type
  showDeleteConfirm.value = true
}
async function confirmDelete() {
  deleting.value = true
  try {
    if (deleteType.value === 'product') {
      await productsApi.deleteProduct(deleteTarget.value.id)
      await fetchProducts()
    } else {
      await productsApi.deleteCategory(deleteTarget.value.id)
      await fetchCategories()
    }
    showDeleteConfirm.value = false
  } catch (e) {
    alert(e?.response?.data?.error || 'Gagal menghapus')
  } finally {
    deleting.value = false
  }
}

// ── Lifecycle ────────────────────────────────────────────────
onMounted(() => {
  fetchOutlets()
  fetchProducts()
  fetchCategories()
})

watch(pageProd, fetchProducts)
watch(pageCat,  fetchCategories)
</script>

<style scoped>
.prod-root { display: flex; flex-direction: column; gap: 1.1rem; }

.page-header { display: flex; align-items: flex-end; justify-content: space-between; flex-wrap: wrap; gap: .75rem; }
.page-title  { font-size: 1.5rem; font-weight: 800; color: #0f4226; letter-spacing: -.03em; line-height: 1.15; }
.page-sub    { margin-top: .2rem; font-size: .82rem; color: #5a7866; }

/* ── Tabs ─────────────────────────────────────────────────── */
.tab-bar { display: flex; gap: .5rem; }
.tab-btn {
  display: inline-flex; align-items: center; gap: .4rem;
  padding: .45rem 1.05rem; border-radius: .75rem; border: none; cursor: pointer;
  font-size: .8rem; font-weight: 600; color: #5a7866;
  background: rgba(255,255,255,.72); backdrop-filter: blur(16px);
  border: 1px solid rgba(255,255,255,.6); transition: all .15s;
}
.tab-btn:hover { background: rgba(255,255,255,.9); color: #2d5c40; }
.tab-btn--active {
  background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff;
  border-color: transparent; box-shadow: 0 3px 12px rgba(15,66,38,.28);
}
.tab-count {
  font-size: .65rem; font-weight: 700; padding: .1rem .42rem; border-radius: 999px;
  background: rgba(255,255,255,.25); color: inherit;
}
.tab-btn--active .tab-count { background: rgba(255,255,255,.22); }

/* ── Toolbar ──────────────────────────────────────────────── */
.toolbar { display: flex; align-items: center; gap: .75rem; flex-wrap: wrap; }
.search-wrap { position: relative; flex: 1; min-width: 200px; max-width: 380px; }
.search-ico  { position: absolute; left: .7rem; top: 50%; transform: translateY(-50%); color: #8aaa96; pointer-events: none; }
.search-input {
  width: 100%; padding: .45rem .75rem .45rem 2.1rem; border-radius: .75rem;
  background: rgba(255,255,255,.8); backdrop-filter: blur(16px) saturate(160%);
  border: 1px solid rgba(255,255,255,.65); box-shadow: 0 1px 6px rgba(0,0,0,.06);
  font-size: .8rem; color: #1a3d2a; outline: none; transition: border-color .15s, box-shadow .15s;
}
.search-input:focus { border-color: rgba(45,143,86,.4); box-shadow: 0 0 0 3px rgba(45,143,86,.1); }
.search-input::placeholder { color: #9ab5a5; }

.outlet-select-wrap {
  position: relative; display: flex; align-items: center; flex-shrink: 0;
}
.select-ico {
  position: absolute; left: .65rem; top: 50%; transform: translateY(-50%);
  color: #8aaa96; pointer-events: none; z-index: 1;
}
.select-chevron {
  position: absolute; right: .6rem; top: 50%; transform: translateY(-50%);
  color: #8aaa96; pointer-events: none;
}
.outlet-select {
  padding: .43rem 2rem .43rem 2.1rem; border-radius: .75rem; appearance: none;
  background: rgba(255,255,255,.8); backdrop-filter: blur(16px);
  border: 1px solid rgba(255,255,255,.65); box-shadow: 0 1px 6px rgba(0,0,0,.06);
  font-size: .8rem; color: #1a3d2a; outline: none; cursor: pointer;
  transition: border-color .15s; min-width: 180px;
}
.outlet-select:focus { border-color: rgba(45,143,86,.4); }

/* ── Table ────────────────────────────────────────────────── */
.table-panel {
  border-radius: 1.15rem; overflow: hidden;
  background: rgba(255,255,255,.78); backdrop-filter: blur(24px) saturate(180%);
  border: 1px solid rgba(255,255,255,.7);
  box-shadow: 0 2px 16px rgba(0,0,0,.07), 0 1px 0 rgba(255,255,255,.95) inset;
}
.table-scroll { overflow-x: auto; }
.otable { width: 100%; border-collapse: collapse; }
.otable thead tr {
  background: linear-gradient(135deg, rgba(26,92,56,.08) 0%, rgba(15,66,38,.06) 100%);
  border-bottom: 1.5px solid rgba(15,66,38,.1);
}
.otable th {
  padding: .72rem 1rem; font-size: .68rem; font-weight: 700; color: #4a6555;
  text-transform: uppercase; letter-spacing: .07em; text-align: left; white-space: nowrap;
}
.th-avatar { width: 44px; padding-right: 0; }
.tr-data { border-bottom: 1px solid rgba(0,0,0,.04); transition: background .12s; }
.tr-data:last-child { border-bottom: none; }
.tr-data:hover { background: rgba(45,143,86,.04); }
.otable td { padding: .68rem 1rem; vertical-align: middle; }

.td-avatar { width: 44px; padding-right: 0; }
.row-avatar {
  width: 32px; height: 32px; border-radius: .55rem; flex-shrink: 0;
  font-size: .82rem; font-weight: 800; color: #fff; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #1a5c38, #0f4226); box-shadow: 0 2px 6px rgba(15,66,38,.28);
}
.ava-cat { background: linear-gradient(135deg, #7c3aed, #5b21b6); box-shadow: 0 2px 6px rgba(91,33,182,.28); }

.row-name   { font-size: .82rem; font-weight: 700; color: #0f2d1d; }
.row-desc   { display: block; font-size: .72rem; color: #6c8a7a; margin-top: .15rem; }
.row-code   { font-size: .73rem; font-family: monospace; font-weight: 600; color: #2d8f56; background: rgba(45,143,86,.1); padding: .15rem .45rem; border-radius: .4rem; }
.row-muted  { font-size: .76rem; color: #8aaa96; }
.row-outlet { font-size: .76rem; color: #4a6555; font-weight: 500; }
.row-price  { font-size: .8rem; font-weight: 700; color: #0f2d1d; font-family: monospace; }

.tag-category {
  display: inline-block; padding: .15rem .5rem; border-radius: .4rem;
  background: rgba(124,58,237,.09); color: #7c3aed;
  font-size: .72rem; font-weight: 600;
}

/* ── Skeleton ─────────────────────────────────────────────── */
.tr-skeleton td { padding: .72rem 1rem; }
.skel { border-radius: 5px; background: linear-gradient(90deg,#e5eae7 25%,#f0f4f1 50%,#e5eae7 75%); background-size: 200% 100%; animation: shimmer 1.4s infinite; }
.skel-avatar { width: 32px; height: 32px; border-radius: .55rem; }
.skel-w30 { height: 9px; width: 30%; }
.skel-w44 { height: 10px; width: 44%; }
.skel-w50 { height: 10px; width: 50%; }
.skel-w60 { height: 10px; width: 60%; }
.skel-mt  { margin-top: .28rem; }
@keyframes shimmer { 0% { background-position: 200% 0; } 100% { background-position: -200% 0; } }

/* ── Empty ────────────────────────────────────────────────── */
.empty-state { display: flex; flex-direction: column; align-items: center; gap: .35rem; padding: 2.5rem 1rem; }
.empty-icon  { width: 52px; height: 52px; border-radius: .9rem; background: rgba(16,185,129,.08); color: #059669; display: flex; align-items: center; justify-content: center; margin-bottom: .25rem; }
.empty-title { font-size: .88rem; font-weight: 700; color: #0f2d1d; }
.empty-sub   { font-size: .76rem; color: #7a9e8a; }

/* ── Footer ───────────────────────────────────────────────── */
.table-footer {
  display: flex; justify-content: flex-end; padding: .65rem 1.1rem;
  border-top: 1px solid rgba(0,0,0,.05);
}

/* ── Tambah button ─────────────────────────────────────────── */
.btn-add {
  display: inline-flex; align-items: center; gap: .4rem;
  padding: .43rem 1rem; border-radius: .75rem; border: none; cursor: pointer;
  font-size: .8rem; font-weight: 700; color: #fff; white-space: nowrap; flex-shrink: 0;
  background: linear-gradient(135deg, #1a5c38, #0f4226);
  box-shadow: 0 3px 12px rgba(15,66,38,.28); transition: opacity .15s;
}
.btn-add:hover { opacity: .88; }

/* ── Action column ─────────────────────────────────────────── */
.th-actions { width: 76px; padding: 0; }
.td-actions { width: 76px; }
.action-btns { display: flex; gap: .35rem; }
.act-edit, .act-del {
  width: 28px; height: 28px; border-radius: .45rem; border: none; cursor: pointer;
  display: flex; align-items: center; justify-content: center; transition: all .12s;
}
.act-edit { background: rgba(45,143,86,.1); color: #2d8f56; }
.act-edit:hover { background: rgba(45,143,86,.2); }
.act-del  { background: rgba(220,38,38,.07); color: #dc2626; }
.act-del:hover { background: rgba(220,38,38,.15); }

/* ── Modals ────────────────────────────────────────────────── */
.modal-overlay {
  position: fixed; inset: 0; z-index: 1000;
  background: rgba(0,0,0,.38); backdrop-filter: blur(4px);
  display: flex; align-items: center; justify-content: center; padding: 1.25rem;
}
.modal-card {
  background: rgba(255,255,255,.92); backdrop-filter: blur(28px) saturate(180%);
  border: 1px solid rgba(255,255,255,.8); border-radius: 1.25rem;
  width: 100%; max-width: 460px;
  box-shadow: 0 20px 60px rgba(0,0,0,.2);
}
.modal-card--sm { max-width: 360px; }
.modal-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1.15rem 1.4rem .75rem; border-bottom: 1px solid rgba(0,0,0,.06);
}
.modal-title { font-size: 1rem; font-weight: 800; color: #0f2d1d; }
.modal-close {
  width: 28px; height: 28px; border: none; background: rgba(0,0,0,.06);
  border-radius: 50%; cursor: pointer; font-size: 1.1rem; color: #5a7866;
  display: flex; align-items: center; justify-content: center; transition: background .12s;
}
.modal-close:hover { background: rgba(0,0,0,.12); }
.modal-body { padding: 1.1rem 1.4rem 0; }
.modal-footer { display: flex; justify-content: flex-end; gap: .65rem; padding: 1rem 0 1.15rem; }
.form-group { display: flex; flex-direction: column; gap: .3rem; margin-bottom: .85rem; }
.form-group label { font-size: .75rem; font-weight: 700; color: #4a6555; }
.req { color: #dc2626; }
.form-input {
  padding: .45rem .75rem; border-radius: .65rem;
  background: rgba(255,255,255,.85); border: 1px solid rgba(0,0,0,.12);
  font-size: .82rem; color: #0f2d1d; outline: none; transition: border-color .15s, box-shadow .15s;
}
.form-input:focus { border-color: rgba(45,143,86,.4); box-shadow: 0 0 0 3px rgba(45,143,86,.1); }
.form-input--code { background: rgba(0,0,0,.04); color: #4a6555; font-family: monospace; font-weight: 600; cursor: default; }
.form-hint { display: block; font-size: .68rem; color: #8ca898; margin-top: .2rem; }
.photo-row { display: flex; align-items: center; gap: .8rem; }
.photo-thumb { width: 72px; height: 72px; border-radius: .6rem; overflow: hidden; background: #f1f5f3; border: 1px solid rgba(0,0,0,.08); display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.photo-thumb img { width: 100%; height: 100%; object-fit: cover; }
.photo-empty { font-size: .6rem; color: #9fb3a8; text-align: center; padding: 0 .3rem; }
.photo-actions { display: flex; gap: .5rem; flex-wrap: wrap; }
.btn-photo-del { padding: .4rem .8rem; border-radius: .55rem; border: none; background: rgba(239,68,68,.1); color: #dc2626; font-size: .78rem; font-weight: 600; cursor: pointer; }
.btn-photo-del:hover { background: rgba(239,68,68,.18); }

/* ── Mobile cards ── */
.mobile-cards { display: none; }
.m-state { padding: 1.5rem; text-align: center; color: #8ca898; font-size: .85rem; }
@media (max-width: 639px) {
  .desktop-only { display: none !important; }
  .mobile-cards { display: flex; flex-direction: column; }
  .mcard { display: flex; gap: .7rem; padding: .75rem .85rem; border-bottom: 1px solid rgba(0,0,0,.05); align-items: center; }
  .mcard-photo { width: 46px; height: 46px; border-radius: .55rem; overflow: hidden; flex-shrink: 0; display: flex; align-items: center; justify-content: center; background: #f1f5f3; }
  .mcard-photo img { width: 100%; height: 100%; object-fit: cover; }
  .mcard-photo .row-avatar { width: 100%; height: 100%; border-radius: 0; }
  .mcard-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: .12rem; }
  .mcard-meta { display: flex; gap: .4rem; align-items: center; flex-wrap: wrap; margin: .1rem 0; }
  .mcard-acts { display: flex; gap: .35rem; flex-shrink: 0; }
}
.form-row { display: flex; gap: .75rem; }
.form-row .form-group { flex: 1; }
.select-wrap { position: relative; }
.form-select {
  width: 100%; padding: .45rem .75rem; border-radius: .65rem; appearance: none;
  background: rgba(255,255,255,.85); border: 1px solid rgba(0,0,0,.12);
  font-size: .82rem; color: #0f2d1d; outline: none; cursor: pointer; transition: border-color .15s;
}
.form-select:focus { border-color: rgba(45,143,86,.4); }
.btn-cancel {
  padding: .42rem 1rem; border-radius: .65rem; border: 1px solid rgba(0,0,0,.14);
  background: transparent; font-size: .8rem; font-weight: 600; color: #5a7866; cursor: pointer; transition: background .12s;
}
.btn-cancel:hover { background: rgba(0,0,0,.05); }
.btn-save {
  padding: .42rem 1.2rem; border-radius: .65rem; border: none; cursor: pointer;
  font-size: .8rem; font-weight: 700; color: #fff;
  background: linear-gradient(135deg, #1a5c38, #0f4226);
  box-shadow: 0 3px 10px rgba(15,66,38,.25); transition: opacity .15s;
}
.btn-save:disabled { opacity: .6; cursor: not-allowed; }
.btn-save:hover:not(:disabled) { opacity: .88; }
.btn-danger {
  padding: .42rem 1.2rem; border-radius: .65rem; border: none; cursor: pointer;
  font-size: .8rem; font-weight: 700; color: #fff;
  background: linear-gradient(135deg, #dc2626, #b91c1c);
  box-shadow: 0 3px 10px rgba(185,28,28,.25); transition: opacity .15s;
}
.btn-danger:disabled { opacity: .6; cursor: not-allowed; }
.btn-danger:hover:not(:disabled) { opacity: .88; }
.del-text { font-size: .85rem; color: #374151; line-height: 1.55; margin-bottom: .1rem; }
</style>
