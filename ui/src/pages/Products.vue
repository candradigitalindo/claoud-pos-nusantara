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
      <div v-if="showProdModal" class="modal-overlay product-overlay" :inert="showRecipePicker" :aria-hidden="showRecipePicker || undefined" @click.self="closeProductModal" @keydown.esc="closeProductModal">
        <div class="modal-card product-modal" role="dialog" aria-modal="true" aria-labelledby="product-modal-title" aria-describedby="product-modal-description">
          <div class="modal-header product-modal-header">
            <div class="product-heading">
              <span class="product-heading-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="m12 3 9 5-9 5-9-5 9-5Zm-9 5v9l9 5 9-5V8M12 13v9M7.5 5.5l9 5" /></svg></span>
              <div>
                <h2 id="product-modal-title" class="modal-title">{{ editingProd ? 'Edit Produk' : 'Tambah Produk' }}</h2>
                <p id="product-modal-description" class="product-modal-description">Atur informasi jual, foto, dan resep produk dalam satu tempat.</p>
              </div>
            </div>
            <AppActionButton icon="close" tone="neutral" label="Tutup formulir produk" :disabled="savingProd" @click="closeProductModal" />
          </div>
          <form ref="productForm" class="product-form" novalidate @submit.prevent="saveProd">
            <div class="product-tabs" role="group" aria-label="Bagian formulir produk">
              <button type="button" :aria-pressed="productPane === 'info'" aria-controls="product-info-panel" @click="productPane = 'info'">Informasi produk</button>
              <button type="button" :aria-pressed="productPane === 'media'" aria-controls="product-media-panel" @click="productPane = 'media'">Foto &amp; resep</button>
            </div>
            <div class="product-form-panels">
              <section id="product-info-panel" class="product-info-panel" :class="{ 'product-pane-inactive': productPane !== 'info' }" aria-labelledby="product-info-title">
                <div class="product-section-heading"><h3 id="product-info-title">Informasi produk</h3><span><span class="req">*</span> Wajib diisi</span></div>
                <div class="product-fields">
                  <div class="form-group product-field-wide">
                    <label for="product-outlet">Outlet <span v-if="!editingProd" class="req">*</span><span v-else class="product-field-note">Lokasi produk</span></label>
                    <SearchSelect v-if="!editingProd" id="product-outlet" v-model="formProd.outlet_id" :options="outlets" placeholder="Pilih outlet…" searchPlaceholder="Cari outlet…" aria-required="true" />
                    <div v-else id="product-outlet" class="product-outlet-readonly" :title="productOutletName">
                      <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M3 10h18l-2-6H5l-2 6Zm2 0v10h14V10M9 20v-6h6v6" /></svg>
                      <span>{{ productOutletName }}</span>
                    </div>
                  </div>
                  <div class="form-group product-field-wide">
                    <label for="product-name">Nama produk <span class="req">*</span></label>
                    <input id="product-name" v-model="formProd.name" class="form-input" placeholder="Contoh: Nasi Goreng Spesial" required />
                  </div>
                  <div class="form-group">
                    <label for="product-category">Kategori</label>
                    <SearchSelect id="product-category" v-model="formProd.category_id" :options="categoryModalOptions" :disabled="!formProdOutletId" :placeholder="formProdOutletId ? 'Pilih kategori…' : 'Pilih outlet dahulu'" searchPlaceholder="Cari kategori…" @change="onCategorySelect" />
                  </div>
                  <div class="form-group">
                    <label for="product-code">Kode produk <span class="product-field-note">Otomatis</span></label>
                    <input id="product-code" v-model="formProd.code" class="form-input form-input--code" placeholder="Dibuat saat disimpan" readonly />
                  </div>
                  <div class="form-group product-field-wide">
                    <label for="product-price">Harga jual <span class="req">*</span><span class="product-field-note">Rupiah per produk</span></label>
                    <input id="product-price" :value="priceDisplay" @input="onPriceInput" class="form-input product-price-input" type="text" inputmode="numeric" placeholder="Rp 0" required />
                  </div>
                  <div class="form-group product-field-wide">
                    <label for="product-description">Deskripsi <span class="product-field-note">Opsional</span></label>
                    <input id="product-description" v-model="formProd.description" class="form-input" placeholder="Komposisi, rasa, atau keterangan singkat produk" />
                  </div>
                </div>
              </section>
              <section id="product-media-panel" class="product-media-panel" :class="{ 'product-pane-inactive': productPane !== 'media' }" aria-labelledby="product-media-title">
                <div class="product-section-heading"><h3 id="product-media-title">Foto &amp; resep</h3><span>Pelengkap produk</span></div>
                <div class="product-preview">
                  <div class="product-preview-image">
                    <img v-if="photoPreview || formProd.photo_url" :src="photoPreview || formProd.photo_url" :alt="formProd.name || 'Foto produk'" />
                    <div v-else class="product-preview-placeholder"><svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M3 5h18v15H3V5Zm0 11 6-6 5 5 3-3 4 4M16 8h.01" /></svg><span>Foto produk</span></div>
                  </div>
                  <div class="product-preview-copy">
                    <span class="product-preview-eyebrow">Pratinjau produk</span>
                    <strong :title="formProd.name">{{ formProd.name || 'Nama produk' }}</strong>
                    <span class="product-preview-category">{{ formProd.category_name || 'Tanpa kategori' }}</span>
                    <span class="product-preview-price">{{ formatRupiah(formProd.price || 0) }}</span>
                  </div>
                </div>
                <div class="product-photo-tools">
                  <p>Foto tampil di reservasi publik.<span>JPG, PNG, WEBP · Maks. 10 MB</span></p>
                  <div class="photo-actions">
                    <input ref="photoInput" type="file" accept="image/*" hidden @change="onPickPhoto" />
                    <AppActionButton icon="upload" :label="(photoPreview || formProd.photo_url) ? 'Ganti foto produk' : 'Pilih foto produk'" @click="photoInput?.click()" />
                    <AppActionButton v-if="photoPreview || formProd.photo_url" icon="delete" label="Hapus foto produk" @click="clearPhoto" />
                  </div>
                </div>
                <div class="product-recipe-section">
                  <div class="form-group">
                    <label for="product-recipe">Resep <span class="product-field-note">Opsional</span></label>
                    <button id="product-recipe" type="button" class="product-recipe-trigger" :disabled="!formProdOutletId || savingProd" aria-haspopup="dialog" :aria-expanded="showRecipePicker" aria-describedby="product-recipe-help" :title="selectedRecipeName || 'Pilih resep dan lihat bahan serta SOP'" @click="showRecipePicker = true">
                      <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M4 3h13a3 3 0 0 1 3 3v15H7a3 3 0 0 1-3-3V3Zm0 14h16M8 7h8M8 11h6"/></svg>
                      <span>{{ selectedRecipeName || 'Pilih resep' }}</span>
                      <svg class="product-recipe-chevron" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="m9 5 7 7-7 7"/></svg>
                    </button>
                  </div>
                  <div id="product-recipe-help" class="product-recipe-help" :class="{ 'product-recipe-help-warning': recipeLoadError || currentRecipeUnavailable }" :role="recipeLoadError ? 'alert' : undefined">
                    <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10h.01"/></svg>
                    <div><strong>{{ productRecipeInfo.title }}</strong><p>{{ productRecipeInfo.description }}</p><button v-if="recipeLoadError" type="button" class="product-recipe-retry" @click="fetchRecipeOptions(formProdOutletId)">Coba lagi</button></div>
                  </div>
                </div>
              </section>
            </div>
            <div class="product-modal-footer">
              <p v-if="productSaveError" class="product-save-error" role="alert">{{ productSaveError }}</p>
              <p v-else class="product-footer-note">{{ editingProd ? 'Perubahan diterapkan setelah disimpan.' : 'Pilih outlet, lalu isi nama dan harga jual.' }}</p>
              <div class="product-footer-actions">
                <button type="button" class="btn-cancel" :disabled="savingProd" @click="closeProductModal">Batal</button>
                <button type="submit" class="btn-save" :disabled="savingProd">{{ savingProd ? 'Menyimpan…' : (editingProd ? 'Simpan Perubahan' : 'Tambah Produk') }}</button>
              </div>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <ProductRecipePicker v-if="showProdModal && showRecipePicker" :key="formProdOutletId" :recipes="recipeOptions" :loading="loadingRecipes" :error="recipeLoadError" :outlet-id="formProdOutletId" :outlet-name="productOutletName" :product-name="formProd.name" :selected="formProd.recipes" @close="showRecipePicker = false" @retry="fetchRecipeOptions(formProdOutletId)" @select="selectProductRecipe" />

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
import ProductRecipePicker from '@/components/ProductRecipePicker.vue'
import { ref, watch, onMounted, onBeforeUnmount, nextTick, computed } from 'vue'
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
const productForm = ref(null)
const productPane = ref('info')
const productSaveError = ref('')
const formProd      = ref({ outlet_id: '', name: '', code: '', description: '', category_id: '', category_name: '', price: 0, photo_url: '', recipe_master_id: '', recipes: [] })
const recipeOptions = ref([])
const loadingRecipes = ref(false)
const recipeLoadError = ref('')
const recipeSelectionChanged = ref(false)
const showRecipePicker = ref(false)
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
const selectedRecipeName = computed(() => {
  const links = formProd.value.recipes
  if (!links.length) return ''
  return links.length > 1 ? `${links.length} resep ditautkan` : `${links[0].name || 'Resep dipilih'}${links[0].multiplier !== 1 ? ` × ${links[0].multiplier}` : ''}`
})
const currentRecipeUnavailable = computed(() => !loadingRecipes.value && !recipeLoadError.value &&
  formProd.value.recipes.some(link => !recipeOptions.value.some(r => r.id === link.recipe_master_id)))

const productOutletName = computed(() => outlets.value.find(o => o.id === formProdOutletId.value)?.name || editingProd.value?.outlet_name || 'Outlet produk')
const productRecipeInfo = computed(() => {
  if (!formProdOutletId.value) return { title: 'Pilih outlet terlebih dahulu', description: 'Pilihan resep mengikuti akses outlet produk.' }
  if (recipeLoadError.value) return { title: 'Pilihan resep belum dimuat', description: recipeLoadError.value }
  if (loadingRecipes.value) return { title: 'Memuat pilihan resep', description: 'Menyiapkan resep yang tersedia untuk outlet ini.' }
  if (currentRecipeUnavailable.value) return { title: 'Resep tertaut tidak tersedia', description: 'Tautan yang ada tetap tersimpan. Pilih resep lain atau Tanpa resep untuk menggantinya.' }
  if (formProd.value.recipes.length) return { title: 'Takaran untuk 1 produk terjual', description: 'HPP dan pemotongan stok menjumlahkan seluruh resep sesuai pengali takarannya.' }
  if (!recipeSelectionChanged.value && editingProd.value?.stock_type === 'single') return { title: 'Menggunakan stok langsung', description: 'Memilih resep akan mengganti pengaturan stok langsung produk ini.' }
  if (!recipeSelectionChanged.value && editingProd.value?.stock_type === 'recipe') return { title: 'Menggunakan resep khusus', description: 'Pilih master resep untuk menggantinya, atau Tanpa resep untuk melepas tautan.' }
  return { title: 'Produk tetap bisa dijual tanpa resep', description: 'Tidak ada pemotongan bahan berdasarkan resep. Tautkan resep jika produk menggunakan komposisi bahan.' }
})

function closeProductModal() {
  if (!savingProd.value && !showRecipePicker.value) showProdModal.value = false
}
function selectProductRecipe(links) {
  formProd.value.recipes = links
  formProd.value.recipe_master_id = links[0]?.recipe_master_id || ''
  recipeSelectionChanged.value = true
  showRecipePicker.value = false
}
let productPreviousFocus, productBodyOverflow
watch(showProdModal, async visible => {
  if (visible) {
    productPreviousFocus = document.activeElement
    productBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    await nextTick()
    document.getElementById(!formProdOutletId.value ? 'product-outlet' : 'product-name')?.focus({ preventScroll: true })
  } else {
    showRecipePicker.value = false
    document.body.style.overflow = productBodyOverflow ?? ''
    productPreviousFocus?.focus?.({ preventScroll: true })
  }
})
onBeforeUnmount(() => {
  if (showProdModal.value) document.body.style.overflow = productBodyOverflow ?? ''
})

function productRecipeLabel(product) {
  if (product.recipes?.length > 1) return `${product.recipes.length} resep ditautkan`
  if (product.recipes?.length) return `Resep: ${product.recipes[0].name}`
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
  showRecipePicker.value = false
  fetchOutletCategories(newId)
  fetchRecipeOptions(newId)
  // Reset category when outlet changes (only for new products)
  if (!editingProd.value) {
    formProd.value.category_id = ''
    formProd.value.category_name = ''
    formProd.value.code = ''
    formProd.value.recipe_master_id = ''
    formProd.value.recipes = []
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
  productPane.value = 'info'
  productSaveError.value = ''
  editingProd.value = null
  formProd.value = { outlet_id: selectedOutlet.value, name: '', code: '', description: '', category_id: '', category_name: '', price: 0, photo_url: '', recipe_master_id: '', recipes: [] }
  recipeSelectionChanged.value = false
  priceDisplay.value = ''
  resetPhoto()
  fetchOutletCategories(selectedOutlet.value)
  fetchRecipeOptions(selectedOutlet.value)
  showProdModal.value = true
}
function openEditProd(p) {
  productPane.value = 'info'
  productSaveError.value = ''
  editingProd.value = p
  formProd.value = { outlet_id: p.outlet_id, name: p.name, code: p.code ?? '', description: p.description ?? '', category_id: p.category_id ?? '', category_name: p.category_name ?? '', price: p.price, photo_url: p.photo_url ?? '', recipe_master_id: p.recipe_master_id || '', recipes: (p.recipes || (p.recipe_master_id ? [{ recipe_master_id: p.recipe_master_id, name: p.recipe_master_name, multiplier: 1 }] : [])).map(link => ({ ...link })) }
  recipeSelectionChanged.value = false
  priceDisplay.value = p.price ? fmtRupiahInput(p.price) : ''
  resetPhoto()
  fetchOutletCategories(p.outlet_id)
  fetchRecipeOptions(p.outlet_id)
  showProdModal.value = true
}
async function saveProd() {
  if (savingProd.value) return
  productSaveError.value = ''
  if (!formProdOutletId.value || !productForm.value?.checkValidity()) {
    productPane.value = 'info'
    await nextTick()
    if (!formProdOutletId.value) {
      productSaveError.value = 'Pilih outlet untuk produk ini.'
      document.getElementById('product-outlet')?.focus()
    } else productForm.value?.reportValidity()
    return
  }
  savingProd.value = true
  try {
    let productId = editingProd.value?.id
    if (editingProd.value) {
      await productsApi.updateProduct(editingProd.value.id, {
        name: formProd.value.name, code: formProd.value.code, description: formProd.value.description,
        category_id: formProd.value.category_id, category_name: formProd.value.category_name,
        price: formProd.value.price,
        ...(recipeSelectionChanged.value ? { recipes: formProd.value.recipes } : {}),
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
    productSaveError.value = e?.message || e?.response?.data?.error || 'Gagal menyimpan produk'
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

/* Product editor: complete form on desktop, two compact sections on phones. */
.product-overlay { padding: 16px; }
.product-modal { max-width: 960px; max-height: calc(100dvh - 32px); display: flex; flex-direction: column; overflow: hidden; background: #fff; }
.product-modal-header { flex-shrink: 0; padding: 18px 24px; gap: 16px; }
.product-heading { display: flex; align-items: center; gap: 12px; min-width: 0; }
.product-heading-icon { width: 40px; height: 40px; padding: 9px; flex-shrink: 0; color: #28714a; background: #edf6f0; border: 1px solid #d9e9de; border-radius: 12px; }
.product-heading-icon svg { width: 100%; height: 100%; }
.product-modal .modal-title { font-size: 1.12rem; line-height: 1.4; }
.product-modal-description { margin-top: 3px; font-size: .78rem; line-height: 1.5; color: #6b7d72; }
.product-form { display: flex; flex-direction: column; min-height: 0; }
.product-form-panels { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr); gap: 24px; padding: 20px 24px; min-height: 0; overflow-y: auto; overscroll-behavior: contain; }
.product-info-panel, .product-media-panel { min-width: 0; }
.product-media-panel { border-left: 1px solid #e6ece8; padding-left: 24px; }
.product-section-heading { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; margin-bottom: 14px; }
.product-section-heading h3 { font-size: .83rem; font-weight: 700; color: #244332; }
.product-section-heading > span { font-size: .68rem; color: #7b8b81; }
.product-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.product-field-wide { grid-column: 1 / -1; }
.product-modal .form-group { margin-bottom: 0; gap: 5px; min-width: 0; }
.product-modal .form-group label { display: flex; align-items: baseline; gap: 4px; font-size: .75rem; }
.product-field-note { margin-left: auto; font-size: .65rem; font-weight: 400; color: #7b8b81; }
.product-modal .form-input, .product-modal :deep(.ss-trigger) { width: 100%; min-width: 0; min-height: 38px; border-radius: 8px; font-size: .82rem; background-color: #fff; }
.product-modal .form-input--code { background: #f5f7f6; font-size: .76rem; }
.product-modal .product-price-input { font-weight: 700; color: #185b35; }
.product-outlet-readonly { display: flex; align-items: center; gap: 8px; min-height: 38px; border: 1px solid #dce6df; padding: 8px 11px; border-radius: 8px; background: #f6f9f7; font-size: .82rem; color: #385e47; }
.product-outlet-readonly svg { width: 17px; height: 17px; flex-shrink: 0; }
.product-outlet-readonly span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.product-preview { display: flex; align-items: center; gap: 14px; padding: 12px; border: 1px solid #e0e9e3; border-radius: 12px; background: #f8fbf9; }
.product-preview-image { flex: 0 0 96px; height: 96px; border-radius: 9px; overflow: hidden; background: #edf2ee; }
.product-preview-image img { width: 100%; height: 100%; object-fit: cover; }
.product-preview-placeholder { height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; color: #91a697; font-size: .64rem; }
.product-preview-placeholder svg { width: 29px; height: 29px; }
.product-preview-copy { min-width: 0; display: flex; flex-direction: column; align-items: flex-start; gap: 4px; }
.product-preview-eyebrow { color: #7b8b81; font-size: .6rem; letter-spacing: .04em; text-transform: uppercase; }
.product-preview-copy strong { font-size: .87rem; line-height: 1.4; color: #224531; overflow-wrap: anywhere; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.product-preview-category { color: #7b8b81; font-size: .7rem; }
.product-preview-price { font-size: .95rem; font-weight: 700; color: #25633d; }
.product-photo-tools { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 10px 0 16px; }
.product-photo-tools p { font-size: .68rem; line-height: 1.6; color: #607769; }
.product-photo-tools p span { display: block; font-size: .63rem; color: #89998f; }
.product-photo-tools .photo-actions { flex-wrap: nowrap; gap: 2px; }
.product-recipe-section { border-top: 1px solid #e6ece8; padding-top: 16px; }
.product-recipe-trigger { display: flex; align-items: center; gap: 9px; min-height: 38px; padding: 8px 11px; border: 1px solid #dce5df; border-radius: 8px; background: #fff; font-size: .8rem; text-align: left; color: #345d44; cursor: pointer; }
.product-recipe-trigger > svg { width: 17px; height: 17px; flex-shrink: 0; }
.product-recipe-trigger > span { min-width: 0; flex: 1; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.product-recipe-trigger .product-recipe-chevron { width: 14px; color: #8c9e92; }
.product-recipe-trigger:hover:not(:disabled) { border-color: #8ab499; background: #f8fbf9; }
.product-recipe-trigger:focus-visible { outline: 2px solid #5a9170; outline-offset: 2px; }
.product-recipe-trigger:disabled { opacity: .5; cursor: not-allowed; }
.product-recipe-help { display: flex; align-items: flex-start; gap: 9px; margin-top: 12px; padding: 12px; border: 1px solid #e0ece3; border-radius: 9px; background: #f2f8f4; color: #38664a; }
.product-recipe-help > svg { width: 17px; height: 17px; flex-shrink: 0; margin-top: 1px; }
.product-recipe-help strong { display: block; font-size: .72rem; line-height: 1.5; font-weight: 650; }
.product-recipe-help p { margin-top: 3px; font-size: .7rem; line-height: 1.55; color: #617c6b; overflow-wrap: anywhere; }
.product-recipe-help-warning { background: #fffbef; border-color: #f0dfb7; color: #885d18; }
.product-recipe-help-warning p { color: #8b744c; }
.product-recipe-retry { margin-top: 6px; font-size: .72rem; text-decoration: underline; }
.product-modal-footer { flex-shrink: 0; display: flex; align-items: center; justify-content: space-between; gap: 16px; border-top: 1px solid #e6ece8; padding: 14px 24px; background: #fbfcfb; }
.product-footer-note { font-size: .71rem; color: #819086; }
.product-save-error { font-size: .75rem; color: #b42318; overflow-wrap: anywhere; }
.product-footer-actions { display: flex; gap: 8px; flex-shrink: 0; }
.product-footer-actions button { min-height: 38px; }
.product-tabs { display: none; }
@media (max-width: 767px) {
  .product-overlay { padding: 12px; }
  .product-modal { max-height: calc(100dvh - 24px); border-radius: 16px; }
  .product-modal-header { padding: 14px 16px; gap: 8px; }
  .product-heading { gap: 10px; }
  .product-heading-icon { width: 34px; height: 34px; padding: 7px; border-radius: 10px; }
  .product-modal .modal-title { font-size: 1rem; }
  .product-modal-description { font-size: .69rem; }
  .product-tabs { display: flex; gap: 4px; padding: 12px 16px 0; flex-shrink: 0; }
  .product-tabs button { flex: 1; min-height: 38px; padding: 7px 9px; border-radius: 8px; border: 1px solid #e2e8e4; background: #fff; font-size: .76rem; font-weight: 600; color: #6b7d72; }
  .product-tabs button[aria-pressed='true'] { background: #edf6f0; border-color: #c4ddcc; color: #245c39; }
  .product-form-panels { display: block; padding: 16px; }
  .product-fields { gap: 10px; }
  .product-media-panel { border: 0; padding: 0; }
  .product-section-heading { display: none; }
  .product-pane-inactive { display: none; }
  .product-modal-footer { padding: 12px 16px; flex-wrap: wrap; gap: 8px; }
  .product-footer-note { display: none; }
  .product-footer-actions { margin-left: auto; }
  .product-save-error { width: 100%; }
}
@media (max-height: 700px) {
  .product-modal-header { padding-top: 12px; padding-bottom: 12px; }
  .product-form-panels { padding-top: 14px; padding-bottom: 14px; }
  .product-fields { row-gap: 8px; }
  .product-section-heading { margin-bottom: 10px; }
  .product-preview-image { flex-basis: 80px; height: 80px; }
  .product-recipe-section { padding-top: 12px; }
  .product-recipe-help { padding: 10px; margin-top: 10px; }
}
@media (max-width: 767px) and (max-height: 640px) {
  .product-modal-description { display: none; }
  .product-modal .form-input, .product-modal :deep(.ss-trigger), .product-outlet-readonly { min-height: 34px; }
  .product-form-panels { padding-top: 12px; padding-bottom: 12px; }
}
@media (min-width: 768px) and (max-height: 540px) {
  .product-modal-description, .product-section-heading, .product-preview-eyebrow, .product-preview-category { display: none; }
  .product-field-wide { grid-column: auto; }
  .product-form-panels { gap: 16px; }
  .product-media-panel { padding-left: 16px; }
  .product-preview { padding: 8px; }
  .product-preview-image { flex-basis: 56px; height: 56px; }
  .product-photo-tools { padding: 5px 0 8px; }
  .product-recipe-section { padding-top: 8px; }
  .product-recipe-help { padding: 8px; margin-top: 8px; }
  .product-modal-footer { padding-top: 10px; padding-bottom: 10px; }
}
@media (min-width: 768px) and (max-height: 450px) {
  .product-modal-header { padding-top: 8px; padding-bottom: 8px; }
  .product-form-panels { padding-top: 6px; padding-bottom: 6px; }
  .product-media-panel { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; align-content: start; }
  .product-preview { gap: 8px; padding: 6px; }
  .product-preview-image { flex-basis: 40px; height: 40px; }
  .product-preview-placeholder span, .product-photo-tools p { display: none; }
  .product-photo-tools { padding: 0; }
  .product-photo-tools .photo-actions { flex-direction: column; }
  .product-recipe-section { grid-column: 1 / -1; }
  .product-modal-footer { padding-top: 8px; padding-bottom: 8px; }
}
</style>
