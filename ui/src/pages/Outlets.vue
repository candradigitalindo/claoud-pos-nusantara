<!--
  Outlets.vue — Outlet management (Tahoe Glass redesign)
  ───────────────────────────────────────────────────────────────
  Route: /outlets   meta: { title: 'Outlet' }
-->
<template>
  <div class="outlets-root">

    <!-- Page header -->
    <div class="page-header">
      <div>
        <h1 class="page-title">Manajemen Outlet</h1>
        <p class="page-sub">Kelola semua cabang &amp; outlet bisnis Anda</p>
      </div>
      <button class="add-btn" @click="showCreate = true">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
        </svg>
        Tambah Outlet
      </button>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Stat chips -->
    <div class="stat-row">
      <div class="stat-chip sc-blue">
        <div class="sc-icon">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>
          </svg>
        </div>
        <div><span class="sc-num">{{ total }}</span><span class="sc-lbl">Total Outlet</span></div>
      </div>
      <div class="stat-chip sc-emerald">
        <div class="sc-icon">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/>
          </svg>
        </div>
        <div><span class="sc-num">{{ activeCount }}</span><span class="sc-lbl">Aktif</span></div>
      </div>
      <div class="stat-chip sc-rose">
        <div class="sc-icon">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/>
          </svg>
        </div>
        <div><span class="sc-num">{{ inactiveCount }}</span><span class="sc-lbl">Nonaktif</span></div>
      </div>
    </div>

    <!-- Toolbar -->
    <div class="toolbar">
      <div class="search-wrap">
        <svg class="search-ico" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
        <input v-model="search" class="search-input" placeholder="Cari outlet, kode, atau alamat…" type="search" />
      </div>
      <div class="filter-pills">
        <button
          v-for="f in FILTERS" :key="f.value"
          :class="['filter-pill', statusFilter === f.value ? 'filter-pill--active' : '']"
          @click="statusFilter = f.value"
        >{{ f.label }}</button>
      </div>
    </div>

    <!-- Table panel -->
    <div class="table-panel">
      <!-- Mobile cards -->
      <div class="mobile-cards">
        <div v-if="loading" class="m-state">Memuat…</div>
        <div v-else-if="!filteredOutlets.length" class="m-state">{{ search || statusFilter !== 'all' ? 'Tidak ada outlet yang cocok' : 'Belum ada outlet' }}</div>
        <div v-else v-for="row in filteredOutlets" :key="row.id" class="mcard">
          <div :class="['row-avatar', row.is_active ? 'ava--active' : 'ava--inactive']">{{ row.name?.charAt(0)?.toUpperCase() ?? 'O' }}</div>
          <div class="mcard-body">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="row-name">{{ row.name }}</span>
              <span class="row-code">{{ row.code }}</span>
              <span :class="['status-badge', row.is_active ? 'status-active' : 'status-inactive']"><span class="status-dot" />{{ row.is_active ? 'Aktif' : 'Nonaktif' }}</span>
            </div>
            <span class="row-muted text-xs">{{ row.address || '—' }}<span v-if="row.phone"> · {{ row.phone }}</span></span>
          </div>
          <div class="mcard-acts">
            <AppActionButton icon="view" label="Detail" @click="openDetail(row)" />
            <AppActionButton icon="edit" label="Edit" @click="openEdit(row)" />
            <AppActionButton icon="delete" label="Hapus" @click="confirmDelete(row)" />
          </div>
        </div>
      </div>
      <div class="table-scroll desktop-only">
        <table class="otable">
          <thead>
            <tr>
              <th class="th-avatar"></th>
              <th>Nama Outlet</th>
              <th>Kode</th>
              <th>Alamat</th>
              <th>Telepon</th>
              <th>Status</th>
              <th class="th-actions">Aksi</th>
            </tr>
          </thead>
          <tbody>
            <!-- skeleton rows -->
            <template v-if="loading">
              <tr v-for="n in 6" :key="n" class="tr-skeleton">
                <td><div class="skel skel-avatar" /></td>
                <td><div class="skel skel-w60" /><div class="skel skel-w30 skel-mt1" /></td>
                <td><div class="skel skel-w44" /></td>
                <td><div class="skel skel-w80" /></td>
                <td><div class="skel skel-w44" /></td>
                <td><div class="skel skel-pill" /></td>
                <td><div class="skel skel-w80" /></td>
              </tr>
            </template>

            <!-- data rows -->
            <template v-else-if="filteredOutlets.length">
              <tr v-for="row in filteredOutlets" :key="row.id" class="tr-data">
                <td class="td-avatar">
                  <div :class="['row-avatar', row.is_active ? 'ava--active' : 'ava--inactive']">
                    {{ row.name?.charAt(0)?.toUpperCase() ?? 'O' }}
                  </div>
                </td>
                <td>
                  <span class="row-name">{{ row.name }}</span>
                </td>
                <td>
                  <span class="row-code">{{ row.code }}</span>
                </td>
                <td class="td-truncate">
                  <span class="row-muted">{{ row.address || '—' }}</span>
                </td>
                <td>
                  <span class="row-muted">{{ row.phone || '—' }}</span>
                </td>
                <td>
                  <span :class="['status-badge', row.is_active ? 'status-active' : 'status-inactive']">
                    <span class="status-dot" />{{ row.is_active ? 'Aktif' : 'Nonaktif' }}
                  </span>
                </td>
                <td class="td-actions">
                  <AppActionButton icon="view" label="Detail" @click="openDetail(row)" />
                  <AppActionButton icon="edit" label="Edit" @click="openEdit(row)" />
                  <AppActionButton icon="delete" label="Hapus" :disabled="deletingId === row.id" @click="confirmDelete(row)" />
                  <AppActionButton :icon="row.is_active ? 'disable' : 'enable'" :label="row.is_active ? 'Nonaktifkan' : 'Aktifkan'"
                    :disabled="togglingId === row.id"
                    @click="toggleOutlet(row)" />
                </td>
              </tr>
            </template>

            <!-- empty -->
            <tr v-else>
              <td colspan="7">
                <div class="empty-state">
                  <div class="empty-icon">
                    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>
                    </svg>
                  </div>
                  <p class="empty-title">{{ search || statusFilter !== 'all' ? 'Tidak ada outlet yang cocok' : 'Belum ada outlet' }}</p>
                  <p class="empty-sub">{{ search || statusFilter !== 'all' ? 'Coba ubah filter atau kata kunci.' : 'Mulai dengan menambahkan outlet pertama.' }}</p>
                  <button v-if="!search && statusFilter === 'all'" class="add-btn" style="margin-top:.85rem" @click="showCreate = true">
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                      <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
                    </svg>
                    Tambah Outlet
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Pagination -->
    <div v-if="!loading && total > PAGE_SIZE" class="pagination-wrap">
      <AppPagination v-model="page" :total="total" :perPage="PAGE_SIZE" />
    </div>

    <!-- Detail Modal -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showDetail" class="modal-backdrop" @click.self="closeDetail">
          <div class="modal-panel modal-panel--detail">
            <div class="modal-glare" aria-hidden="true" />

            <!-- Detail header -->
            <div class="dh-header">
              <div :class="['dh-avatar', detailOutlet?.is_active ? 'dha--active' : 'dha--inactive']">
                {{ detailOutlet?.name?.charAt(0)?.toUpperCase() ?? 'O' }}
              </div>
              <div class="dh-meta">
                <div class="dh-name-row">
                  <span class="dh-name">{{ detailOutlet?.name ?? '…' }}</span>
                  <span v-if="detailOutlet" class="row-code" style="font-size:.7rem">{{ detailOutlet.code }}</span>
                </div>
                <span v-if="detailOutlet" :class="['status-badge', detailOutlet.is_active ? 'status-active' : 'status-inactive']" style="align-self:flex-start;margin-top:.2rem">
                  <span class="status-dot" />{{ detailOutlet.is_active ? 'Aktif' : 'Nonaktif' }}
                </span>
              </div>
              <AppActionButton icon="access" label="Kelola" :to="detailOutlet ? `/outlets/${detailOutlet.id}` : '#'" @click="closeDetail" />
              <button class="modal-close" @click="closeDetail" aria-label="Tutup">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>

            <div v-if="detailLoading" class="detail-loading">
              <span class="btn-spinner" style="width:22px;height:22px;border-width:2.5px" />
              <span style="font-size:.8rem;color:#5a7866">Memuat data outlet…</span>
            </div>

            <div v-else-if="detailOutlet" class="outlet-detail-grid">
              <!-- Info grid -->
              <div class="detail-section">
                <p class="detail-section-title">Informasi Outlet</p>
                <div class="info-grid">
                  <div class="info-row">
                    <span class="info-lbl">Alamat</span>
                    <span class="info-val">{{ detailOutlet.address || '—' }}</span>
                  </div>
                  <div class="info-row">
                    <span class="info-lbl">Telepon</span>
                    <span class="info-val">{{ detailOutlet.phone || '—' }}</span>
                  </div>
                  <div class="info-row" v-if="detailOutlet.webhook_url">
                    <span class="info-lbl">Webhook URL</span>
                    <span class="info-val info-val--mono">{{ detailOutlet.webhook_url }}</span>
                  </div>
                  <div class="info-row">
                    <span class="info-lbl">Dibuat</span>
                    <span class="info-val">{{ fmtDate(detailOutlet.created_at) }}</span>
                  </div>
                  <div class="info-row">
                    <span class="info-lbl">Diperbarui</span>
                    <span class="info-val">{{ fmtDate(detailOutlet.updated_at) }}</span>
                  </div>
                </div>
              </div>

              <!-- API Key section -->
              <div class="detail-section apikey-section">
                <div class="apikey-section-header">
                  <div class="apikey-icon">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>
                  </div>
                  <p class="detail-section-title" style="margin:0">API Key</p>
                </div>
                <p class="apikey-hint">Gunakan key ini di aplikasi POS untuk koneksi ke outlet.</p>
                <div class="apikey-bar">
                  <code class="apikey-val">{{ showApiKey ? detailOutlet.api_key : maskedKey }}</code>
                  <AppActionButton :icon="showApiKey ? 'hide' : 'view'" :label="showApiKey ? 'Sembunyikan' : 'Tampilkan'" @click="showApiKey = !showApiKey" />
                  <AppActionButton :icon="keyCopied ? 'approve' : 'copy'" :label="keyCopied ? 'Tersalin!' : 'Salin'" @click="copyKey" />
                </div>

                <div v-if="!showRegen" class="regen-row">
                  <AppActionButton icon="password" label="Regenerate API Key" @click="showRegen = true" />
                </div>

                <div v-else class="regen-confirm">
                  <p class="regen-warn">
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
                    API key lama tidak bisa digunakan lagi!
                  </p>
                  <div class="regen-actions">
                    <button class="btn-regen-cancel" @click="showRegen = false">Batal</button>
                    <button class="btn-regen-do" :disabled="regenerating" @click="doRegen">
                      <span v-if="regenerating" class="btn-spinner btn-spinner--light" />
                      <template v-else>Ya, Regenerate</template>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Delete Confirmation Modal -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showDeleteConfirm" class="modal-backdrop" @click.self="showDeleteConfirm = false">
          <div class="modal-panel" style="max-width:420px">
            <div class="modal-glare" aria-hidden="true" />
            <div class="modal-header" style="border-bottom:none">
              <div class="modal-icon" style="background:rgba(239,68,68,.12);color:#dc2626">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
                </svg>
              </div>
              <h2 class="modal-title" style="color:#991b1b">Hapus Outlet</h2>
              <button class="modal-close" @click="showDeleteConfirm = false" aria-label="Tutup">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
              </button>
            </div>
            <div class="modal-body" style="padding-top:0">
              <p style="font-size:.85rem;color:#64748b;line-height:1.6;margin:0">
                Yakin ingin menghapus outlet <strong style="color:#0f172a">{{ deleteTarget?.name }}</strong>?
                Semua data terkait (order, transaksi, produk, kategori, dll) akan <strong style="color:#dc2626">dihapus permanen</strong>.
              </p>
            </div>
            <div class="modal-footer">
              <button class="modal-btn-cancel" @click="showDeleteConfirm = false">Batal</button>
              <button class="modal-btn-save" style="background:linear-gradient(135deg,#dc2626,#991b1b)" :disabled="deleting" @click="doDelete">
                <span v-if="deleting" class="btn-spinner btn-spinner--light" />
                <template v-else>
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                  Ya, Hapus
                </template>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Edit Modal -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showEdit" class="modal-backdrop" @click.self="closeEdit">
          <div class="modal-panel">
            <div class="modal-glare" aria-hidden="true" />
            <div class="modal-header">
              <div class="modal-icon" style="background:linear-gradient(135deg,#1d4ed8,#1e40af)">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
              </div>
              <h2 class="modal-title">Edit Outlet</h2>
              <button class="modal-close" @click="closeEdit" aria-label="Tutup">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
              </button>
            </div>
            <div class="modal-body">
              <AppAlert type="error" :message="editError" />
              <div class="form-grid">
                <div class="field" :class="{ 'field--error': editErrors.name }">
                  <label class="field-label">Nama Outlet <span class="req">*</span></label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
                    <input v-model="editForm.name" class="field-input" placeholder="cth: Cabang Sudirman" @input="editErrors.name=''" />
                  </div>
                  <p v-if="editErrors.name" class="field-err">{{ editErrors.name }}</p>
                </div>
                <div class="field" :class="{ 'field--error': editErrors.code }">
                  <label class="field-label">Kode Outlet <span class="req">*</span></label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>
                    <input v-model="editForm.code" class="field-input field-input--mono" placeholder="cth: CBG-01" @input="editErrors.code=''" />
                  </div>
                  <p v-if="editErrors.code" class="field-err">{{ editErrors.code }}</p>
                </div>
                <div class="field field-span2">
                  <label class="field-label">Alamat</label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z"/><circle cx="12" cy="10" r="3"/></svg>
                    <input v-model="editForm.address" class="field-input" placeholder="Alamat lengkap outlet" />
                  </div>
                </div>
                <div class="field">
                  <label class="field-label">No. Telepon</label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07A19.5 19.5 0 0 1 4.69 12 19.79 19.79 0 0 1 1.61 3.35 2 2 0 0 1 3.6 1h3a2 2 0 0 1 2 1.72c.127.96.361 1.903.7 2.81a2 2 0 0 1-.45 2.11L7.91 8.6a16 16 0 0 0 8 8l.96-.9a2 2 0 0 1 2.11-.45c.907.339 1.85.573 2.81.7A2 2 0 0 1 22 17.92v-.001z"/></svg>
                    <input v-model="editForm.phone" class="field-input" placeholder="cth: 08xxxxxxxxxx" type="tel" />
                  </div>
                </div>
                <div class="field">
                  <label class="field-label">Webhook URL</label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>
                    <input v-model="editForm.webhook_url" class="field-input" placeholder="https://..." />
                  </div>
                </div>
              </div>
            </div>
            <div class="modal-footer">
              <button class="modal-btn-cancel" @click="closeEdit">Batal</button>
              <button class="modal-btn-save" style="background:linear-gradient(135deg,#1d4ed8,#1e40af)" :disabled="updating" @click="doUpdate">
                <span v-if="updating" class="btn-spinner btn-spinner--light" />
                <template v-else>
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>
                  Simpan Perubahan
                </template>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Create Modal -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showCreate" class="modal-backdrop" @click.self="closeCreate">
          <div class="modal-panel">
            <div class="modal-glare" aria-hidden="true" />
            <div class="modal-header">
              <div class="modal-icon">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>
                </svg>
              </div>
              <h2 class="modal-title">Tambah Outlet Baru</h2>
              <button class="modal-close" @click="closeCreate" aria-label="Tutup">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
              </button>
            </div>
            <div class="modal-body">
              <AppAlert type="error" :message="createError" />
              <div class="form-grid">
                <div class="field" :class="{ 'field--error': formErrors.name }">
                  <label class="field-label">Nama Outlet <span class="req">*</span></label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
                    <input v-model="form.name" class="field-input" placeholder="cth: Cabang Sudirman" @input="formErrors.name=''" />
                  </div>
                  <p v-if="formErrors.name" class="field-err">{{ formErrors.name }}</p>
                </div>
                <div class="field" :class="{ 'field--error': formErrors.code }">
                  <label class="field-label">Kode Outlet <span class="req">*</span></label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>
                    <input v-model="form.code" class="field-input field-input--mono" placeholder="cth: CBG-01" @input="formErrors.code=''" />
                  </div>
                  <p v-if="formErrors.code" class="field-err">{{ formErrors.code }}</p>
                </div>
                <div class="field field-span2">
                  <label class="field-label">Alamat</label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z"/><circle cx="12" cy="10" r="3"/></svg>
                    <input v-model="form.address" class="field-input" placeholder="Alamat lengkap outlet" />
                  </div>
                </div>
                <div class="field field-span2">
                  <label class="field-label">No. Telepon</label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07A19.5 19.5 0 0 1 4.69 12 19.79 19.79 0 0 1 1.61 3.35 2 2 0 0 1 3.6 1h3a2 2 0 0 1 2 1.72c.127.96.361 1.903.7 2.81a2 2 0 0 1-.45 2.11L7.91 8.6a16 16 0 0 0 8 8l.96-.9a2 2 0 0 1 2.11-.45c.907.339 1.85.573 2.81.7A2 2 0 0 1 22 17.92v-.001z"/></svg>
                    <input v-model="form.phone" class="field-input" placeholder="cth: 08xxxxxxxxxx" type="tel" />
                  </div>
                </div>
              </div>
            </div>
            <div class="modal-footer">
              <button class="modal-btn-cancel" @click="closeCreate">Batal</button>
              <button class="modal-btn-save" :disabled="creating" @click="createOutlet">
                <span v-if="creating" class="btn-spinner btn-spinner--light" />
                <template v-else>
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>
                  Simpan Outlet
                </template>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { outletsApi }      from '@/api/outlets.js'
import { useToastStore }   from '@/stores/toast.js'
import { PAGE_SIZE }       from '@/utils/constants.js'
import { formatDateTime }  from '@/utils/format.js'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'

// ── Detail modal ────────────────────────────────────────────
const showDetail   = ref(false)
const detailOutlet = ref(null)
const detailLoading = ref(false)
const showApiKey   = ref(false)
const keyCopied    = ref(false)
const showRegen    = ref(false)
const regenerating = ref(false)

const maskedKey = computed(() => {
  const k = detailOutlet.value?.api_key ?? ''
  if (!k) return '—'
  return k.slice(0, 8) + '•'.repeat(16) + k.slice(-4)
})

async function openDetail(row) {
  showDetail.value   = true
  detailOutlet.value = null
  detailLoading.value = true
  showApiKey.value   = false
  showRegen.value    = false
  keyCopied.value    = false
  try {
    detailOutlet.value = await outletsApi.get(row.id)
  } catch {
    toast.error('Gagal memuat detail outlet.')
    showDetail.value = false
  } finally {
    detailLoading.value = false
  }
}

function closeDetail() {
  showDetail.value  = false
  detailOutlet.value = null
  showRegen.value   = false
  showApiKey.value  = false
}

function copyKey() {
  const k = detailOutlet.value?.api_key
  if (!k) return
  navigator.clipboard.writeText(k)
  keyCopied.value = true
  setTimeout(() => { keyCopied.value = false }, 2000)
}

async function doRegen() {
  regenerating.value = true
  try {
    const res = await outletsApi.regenerateKey(detailOutlet.value.id)
    const newKey = res?.api_key ?? res
    detailOutlet.value = { ...detailOutlet.value, api_key: newKey }
    showApiKey.value = true
    showRegen.value  = false
    toast.success('API key berhasil di-regenerate!')
  } catch {
    toast.error('Gagal regenerate API key.')
  } finally {
    regenerating.value = false
  }
}

const fmtDate = formatDateTime

const toast = useToastStore()

const FILTERS = [
  { value: 'all',      label: 'Semua'    },
  { value: 'active',   label: 'Aktif'    },
  { value: 'inactive', label: 'Nonaktif' },
]

const outlets      = ref([])
const total        = ref(0)
const page         = ref(1)
const loading      = ref(false)
const errorMsg     = ref('')
const togglingId   = ref(null)
const search       = ref('')
const statusFilter = ref('all')

const activeCount   = computed(() => outlets.value.filter(o => o.is_active).length)
const inactiveCount = computed(() => outlets.value.filter(o => !o.is_active).length)

const filteredOutlets = computed(() => {
  let list = outlets.value
  if (statusFilter.value === 'active')   list = list.filter(o => o.is_active)
  if (statusFilter.value === 'inactive') list = list.filter(o => !o.is_active)
  if (search.value.trim()) {
    const q = search.value.trim().toLowerCase()
    list = list.filter(o =>
      o.name?.toLowerCase().includes(q) ||
      o.code?.toLowerCase().includes(q) ||
      o.address?.toLowerCase().includes(q)
    )
  }
  return list
})

const showCreate  = ref(false)
const creating    = ref(false)
const createError = ref('')
const form        = reactive({ name: '', code: '', address: '', phone: '' })
const formErrors  = reactive({ name: '', code: '' })

onMounted(fetchOutlets)
watch(page, fetchOutlets)

async function fetchOutlets() {
  loading.value = true
  errorMsg.value = ''
  try {
    const data    = await outletsApi.list({ page: page.value, limit: PAGE_SIZE })
    outlets.value = data.outlets ?? data ?? []
    total.value   = data.total   ?? outlets.value.length
  } catch (err) {
    errorMsg.value = err?.response?.data?.message ?? 'Gagal memuat daftar outlet.'
  } finally {
    loading.value = false
  }
}

async function createOutlet() {
  formErrors.name = form.name ? '' : 'Nama outlet wajib diisi'
  formErrors.code = form.code ? '' : 'Kode outlet wajib diisi'
  if (formErrors.name || formErrors.code) return

  creating.value = true
  createError.value = ''
  try {
    await outletsApi.create(form)
    toast.success('Outlet berhasil ditambahkan!')
    closeCreate()
    page.value = 1
    await fetchOutlets()
  } catch (err) {
    createError.value = err?.response?.data?.message ?? 'Gagal membuat outlet.'
  } finally {
    creating.value = false
  }
}

async function toggleOutlet(row) {
  togglingId.value = row.id
  try {
    await outletsApi.toggle(row.id)
    toast.success(`Outlet ${row.is_active ? 'dinonaktifkan' : 'diaktifkan'}.`)
    row.is_active = !row.is_active
  } catch (err) {
    toast.error(err?.response?.data?.message ?? 'Gagal mengubah status outlet.')
  } finally {
    togglingId.value = null
  }
}

function closeCreate() {
  showCreate.value = false
  form.name = ''; form.code = ''; form.address = ''; form.phone = ''
  formErrors.name = ''; formErrors.code = ''; createError.value = ''
}

// ── Delete outlet ───────────────────────────────────────────
const showDeleteConfirm = ref(false)
const deleteTarget      = ref(null)
const deleting           = ref(false)
const deletingId         = ref(null)

function confirmDelete(row) {
  deleteTarget.value      = row
  showDeleteConfirm.value = true
}

async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value  = true
  deletingId.value = deleteTarget.value.id
  try {
    await outletsApi.delete(deleteTarget.value.id)
    toast.success(`Outlet "${deleteTarget.value.name}" berhasil dihapus.`)
    showDeleteConfirm.value = false
    deleteTarget.value = null
    // Also close detail modal if open for same outlet
    if (detailOutlet.value?.id === deletingId.value) closeDetail()
    await fetchOutlets()
  } catch (err) {
    toast.error(err?.response?.data?.message ?? 'Gagal menghapus outlet.')
  } finally {
    deleting.value   = false
    deletingId.value = null
  }
}

// ── Edit outlet ─────────────────────────────────────────────
const showEdit   = ref(false)
const editTarget = ref(null)
const updating   = ref(false)
const editError  = ref('')
const editForm   = reactive({ name: '', code: '', address: '', phone: '', webhook_url: '' })
const editErrors = reactive({ name: '', code: '' })

function openEdit(row) {
  editTarget.value      = row
  editForm.name         = row.name         || ''
  editForm.code         = row.code         || ''
  editForm.address      = row.address      || ''
  editForm.phone        = row.phone        || ''
  editForm.webhook_url  = row.webhook_url  || ''
  editErrors.name = ''; editErrors.code = ''
  editError.value = ''
  showEdit.value = true
}

function closeEdit() {
  showEdit.value   = false
  editTarget.value = null
}

async function doUpdate() {
  editErrors.name = editForm.name ? '' : 'Nama outlet wajib diisi'
  editErrors.code = editForm.code ? '' : 'Kode outlet wajib diisi'
  if (editErrors.name || editErrors.code) return
  updating.value  = true
  editError.value = ''
  try {
    await outletsApi.update(editTarget.value.id, editForm)
    toast.success('Outlet berhasil diperbarui!')
    closeEdit()
    await fetchOutlets()
  } catch (err) {
    editError.value = err?.response?.data?.message ?? 'Gagal memperbarui outlet.'
  } finally {
    updating.value = false
  }
}
</script>

<style scoped>
.outlets-root { display: flex; flex-direction: column; gap: 1.25rem; }

.page-header { display: flex; align-items: flex-end; justify-content: space-between; flex-wrap: wrap; gap: .75rem; }
.page-title  { font-size: 1.5rem; font-weight: 800; color: #0f4226; letter-spacing: -.03em; line-height: 1.15; }
.page-sub    { margin-top: .2rem; font-size: .82rem; color: #5a7866; }

.add-btn {
  display: inline-flex; align-items: center; gap: .4rem;
  padding: .48rem 1.1rem; border-radius: .75rem;
  background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff;
  font-size: .8rem; font-weight: 700; cursor: pointer; border: none;
  box-shadow: 0 3px 12px rgba(15,66,38,.32); transition: opacity .15s, transform .15s;
}
.add-btn:hover { opacity: .88; transform: translateY(-2px); }

.btn-icon {
  display: inline-flex; align-items: center; justify-content: center;
  width: 30px; height: 30px; border-radius: .5rem;
  border: 1px solid transparent; cursor: pointer;
  transition: background .14s, border-color .14s, opacity .14s;
  flex-shrink: 0;
}
.btn-icon:disabled { opacity: .4; cursor: not-allowed; }
.btn-icon--detail { background: rgba(45,143,86,.1);  color: #2d8f56; border-color: rgba(45,143,86,.18); }
.btn-icon--detail:hover { background: rgba(45,143,86,.18); border-color: rgba(45,143,86,.32); }
.btn-icon--edit   { background: rgba(217,119,6,.1);  color: #b45309; border-color: rgba(217,119,6,.18); }
.btn-icon--edit:hover   { background: rgba(217,119,6,.18); border-color: rgba(217,119,6,.32); }
.btn-icon--delete { background: rgba(239,68,68,.07); color: #dc2626; border-color: rgba(239,68,68,.18); }
.btn-icon--delete:not(:disabled):hover { background: rgba(239,68,68,.15); border-color: rgba(239,68,68,.35); }
.btn-icon--off    { background: rgba(220,38,38,.07); color: #dc2626; border-color: rgba(220,38,38,.18); }
.btn-icon--off:not(:disabled):hover    { background: rgba(220,38,38,.15); border-color: rgba(220,38,38,.35); }
.btn-icon--on     { background: rgba(22,163,74,.08); color: #16a34a; border-color: rgba(22,163,74,.18); }
.btn-icon--on:not(:disabled):hover     { background: rgba(22,163,74,.16); border-color: rgba(22,163,74,.35); }

.stat-row { display: flex; gap: .75rem; flex-wrap: wrap; }
.stat-chip {
  display: flex; align-items: center; gap: .65rem; padding: .55rem 1rem; border-radius: 999px;
  background: rgba(255,255,255,.78); backdrop-filter: blur(18px) saturate(170%);
  border: 1px solid rgba(255,255,255,.65); box-shadow: 0 1px 8px rgba(0,0,0,.06);
}
.sc-icon { display: flex; align-items: center; justify-content: center; width: 28px; height: 28px; border-radius: .55rem; flex-shrink: 0; }
.sc-blue    .sc-icon { background: rgba(59,130,246,.12);  color: #2563eb; }
.sc-emerald .sc-icon { background: rgba(16,185,129,.12);  color: #059669; }
.sc-rose    .sc-icon { background: rgba(244,63,94,.10);   color: #e11d48; }
.stat-chip > div:last-child { display: flex; flex-direction: column; }
.sc-num { font-size: 1.05rem; font-weight: 800; line-height: 1; color: #0f2d1d; }
.sc-lbl { font-size: .67rem; color: #7a9e8a; font-weight: 500; margin-top: .08rem; }

.toolbar { display: flex; align-items: center; gap: .75rem; flex-wrap: wrap; }
.search-wrap { position: relative; flex: 1; min-width: 200px; max-width: 380px; }
.search-ico  { position: absolute; left: .7rem; top: 50%; transform: translateY(-50%); color: #8aaa96; pointer-events: none; }
.search-input {
  width: 100%; padding: .45rem .75rem .45rem 2.1rem; border-radius: .75rem;
  background: rgba(255,255,255,.8); backdrop-filter: blur(16px) saturate(160%);
  border: 1px solid rgba(255,255,255,.65); box-shadow: 0 1px 6px rgba(0,0,0,.06);
  font-size: .8rem; color: #1a3d2a; outline: none; transition: border-color .15s, box-shadow .15s;
}
.search-input:focus { border-color: rgba(45,143,86,.4); box-shadow: 0 0 0 3px rgba(45,143,86,.1), 0 1px 6px rgba(0,0,0,.06); }
.search-input::placeholder { color: #9ab5a5; }
.filter-pills { display: flex; gap: .3rem; }
.filter-pill {
  padding: .3rem .8rem; border-radius: 999px; cursor: pointer; font-size: .72rem; font-weight: 600;
  background: rgba(255,255,255,.72); backdrop-filter: blur(12px);
  border: 1px solid rgba(0,0,0,.07); color: #5a7866; transition: all .15s;
}
.filter-pill:hover { background: rgba(255,255,255,.92); color: #2d5c40; }
.filter-pill--active { background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff; border-color: transparent; box-shadow: 0 2px 8px rgba(15,66,38,.28); }

/* ── Table panel ─────────────────────────────────── */
.table-panel {
  border-radius: 1.15rem; overflow: hidden;
  background: rgba(255,255,255,.78); backdrop-filter: blur(24px) saturate(180%);
  border: 1px solid rgba(255,255,255,.7);
  box-shadow: 0 2px 16px rgba(0,0,0,.07), 0 1px 0 rgba(255,255,255,.95) inset;
}
.table-scroll { overflow-x: auto; }
.mobile-cards { display: none; }
.m-state { padding: 1.5rem; text-align: center; color: #8ca898; font-size: .85rem; }
@media (max-width: 639px) {
  .desktop-only { display: none !important; }
  .mobile-cards { display: flex; flex-direction: column; }
  .mcard { display: flex; gap: .7rem; padding: .8rem .85rem; border-bottom: 1px solid rgba(0,0,0,.05); align-items: center; }
  .mcard-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: .25rem; }
  .mcard-acts { display: flex; gap: .3rem; flex-shrink: 0; }
}

.otable { width: 100%; border-collapse: collapse; }
.otable thead tr {
  background: linear-gradient(135deg, rgba(26,92,56,.08) 0%, rgba(15,66,38,.06) 100%);
  border-bottom: 1.5px solid rgba(15,66,38,.1);
}
.otable th {
  padding: .75rem 1rem; font-size: .68rem; font-weight: 700; color: #4a6555;
  text-transform: uppercase; letter-spacing: .07em; text-align: left; white-space: nowrap;
}
.th-avatar   { width: 48px; padding-right: 0; }
.th-actions  { width: 180px; text-align: right; }

.tr-data { border-bottom: 1px solid rgba(0,0,0,.05); transition: background .12s; }
.tr-data:last-child { border-bottom: none; }
.tr-data:hover { background: rgba(45,143,86,.04); }
.otable td { padding: .72rem 1rem; vertical-align: middle; }

.td-avatar  { width: 48px; padding-right: 0; }
.td-truncate { max-width: 200px; }
.td-truncate .row-muted { display: block; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.td-actions { text-align: right; }

.row-avatar {
  width: 34px; height: 34px; border-radius: .6rem; flex-shrink: 0;
  font-size: .85rem; font-weight: 800; color: #fff;
  display: flex; align-items: center; justify-content: center;
}
.ava--active   { background: linear-gradient(135deg, #1a5c38, #0f4226); box-shadow: 0 2px 6px rgba(15,66,38,.28); }
.ava--inactive { background: linear-gradient(135deg, #6b7280, #4b5563); box-shadow: 0 2px 6px rgba(75,85,99,.2); }

.row-name  { font-size: .82rem; font-weight: 700; color: #0f2d1d; }
.row-code  { font-size: .75rem; font-family: monospace; font-weight: 600; color: #2d8f56; background: rgba(45,143,86,.08); padding: .15rem .45rem; border-radius: .4rem; }
.row-muted { font-size: .78rem; color: #7a9e8a; }

.status-badge { display: inline-flex; align-items: center; gap: .28rem; padding: .2rem .6rem; border-radius: 999px; font-size: .65rem; font-weight: 700; white-space: nowrap; }
.status-active   { background: rgba(22,163,74,.13); color: #16a34a; }
.status-inactive { background: rgba(107,114,128,.11); color: #6b7280; }
.status-dot { width: 5px; height: 5px; border-radius: 50%; background: currentColor; flex-shrink: 0; }
.status-active .status-dot { animation: pulse-dot 2s ease-in-out infinite; }
@keyframes pulse-dot { 0%,100% { opacity: 1; } 50% { opacity: .4; } }

.td-actions { display: flex; align-items: center; justify-content: flex-end; gap: .4rem; padding-right: 1rem; }

/* skeleton */
.tr-skeleton td { padding: .75rem 1rem; }
.skel { border-radius: 5px; background: linear-gradient(90deg,#e5eae7 25%,#f0f4f1 50%,#e5eae7 75%); background-size: 200% 100%; animation: shimmer 1.4s infinite; }
.skel-avatar { width: 34px; height: 34px; border-radius: .6rem; }
.skel-w30    { height: 8px;  width: 30%; }
.skel-w44    { height: 10px; width: 44%; }
.skel-w60    { height: 10px; width: 60%; }
.skel-w80    { height: 10px; width: 80%; }
.skel-pill   { height: 20px; width: 56px; border-radius: 999px; }
.skel-mt1    { margin-top: .3rem; }
@keyframes shimmer { 0% { background-position: 200% 0; } 100% { background-position: -200% 0; } }

.empty-state { display: flex; flex-direction: column; align-items: center; gap: .4rem; padding: 3rem 1rem; }
.empty-icon  { width: 56px; height: 56px; border-radius: 1rem; background: rgba(16,185,129,.08); color: #059669; display: flex; align-items: center; justify-content: center; margin-bottom: .3rem; }
.empty-title { font-size: .9rem; font-weight: 700; color: #0f2d1d; }
.empty-sub   { font-size: .78rem; color: #7a9e8a; }

.pagination-wrap {
  display: flex; justify-content: flex-end; padding: .75rem 1.2rem; border-radius: 1rem;
  background: rgba(255,255,255,.72); backdrop-filter: blur(16px);
  border: 1px solid rgba(255,255,255,.65); box-shadow: 0 1px 8px rgba(0,0,0,.05);
}

.modal-backdrop {
  position: fixed; inset: 0; z-index: 60; display: flex; align-items: center; justify-content: center;
  padding: 1rem; background: rgba(7,26,13,.55); backdrop-filter: blur(6px);
}
.modal-panel {
  position: relative; overflow: hidden; width: 100%; max-width: 768px; min-width: 0; max-height: calc(100dvh - 2rem); display: flex; flex-direction: column; border-radius: 1.35rem;
  background: rgba(255,255,255,.88); backdrop-filter: blur(32px) saturate(180%);
  border: 1px solid rgba(255,255,255,.75);
  box-shadow: 0 24px 64px rgba(0,0,0,.18), 0 1px 0 rgba(255,255,255,.9) inset;
}
.modal-glare { position: absolute; inset: 0; border-radius: inherit; pointer-events: none; background: linear-gradient(135deg, rgba(255,255,255,.6) 0%, rgba(255,255,255,0) 50%); }
.modal-header {
  display: flex; align-items: center; gap: .75rem; padding: 1.15rem 1.35rem;
  border-bottom: 1px solid rgba(0,0,0,.07);
  background: linear-gradient(135deg, rgba(255,255,255,.55) 0%, rgba(240,249,244,.35) 100%);
}
.modal-icon {
  width: 36px; height: 36px; border-radius: .75rem; flex-shrink: 0;
  background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff;
  display: flex; align-items: center; justify-content: center; box-shadow: 0 2px 8px rgba(15,66,38,.28);
}
.modal-title { flex: 1; font-size: 1rem; font-weight: 800; color: #0f2d1d; }
.modal-close {
  width: 30px; height: 30px; border-radius: .55rem; border: none; cursor: pointer;
  background: rgba(0,0,0,.05); color: #5a7866; display: flex; align-items: center; justify-content: center; transition: background .14s;
}
.modal-close:hover { background: rgba(0,0,0,.1); color: #0f2d1d; }
.modal-body { padding: 1.25rem 1.35rem; min-height: 0; overflow-y: auto; }

.form-grid  { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .85rem; }
.field-span2 { grid-column: span 2; }
.field { display: flex; flex-direction: column; gap: .3rem; }
.field-label { font-size: .72rem; font-weight: 700; color: #4a6555; text-transform: uppercase; letter-spacing: .06em; }
.req { color: #e11d48; margin-left: .1rem; }
.field-input-wrap { position: relative; }
.field-ico { position: absolute; left: .65rem; top: 50%; transform: translateY(-50%); color: #8aaa96; pointer-events: none; }
.field-input {
  width: 100%; padding: .45rem .65rem .45rem 2rem; border-radius: .65rem;
  background: rgba(240,248,243,.85); border: 1px solid rgba(0,0,0,.09);
  font-size: .82rem; color: #1a3d2a; outline: none;
  transition: border-color .15s, box-shadow .15s; box-sizing: border-box;
}
.field-input:focus { border-color: rgba(45,143,86,.4); box-shadow: 0 0 0 3px rgba(45,143,86,.1); background: #fff; }
.field-input--mono { font-family: monospace; letter-spacing: .04em; }
.field--error .field-input { border-color: rgba(220,38,38,.4); background: rgba(255,240,240,.6); }
.field-err { font-size: .68rem; color: #dc2626; font-weight: 500; }

.modal-footer {
  display: flex; justify-content: flex-end; gap: .65rem; padding: 1rem 1.35rem;
  border-top: 1px solid rgba(0,0,0,.07); background: rgba(248,252,249,.5);
}
.modal-btn-cancel {
  padding: .45rem 1.1rem; border-radius: .65rem; cursor: pointer;
  background: rgba(0,0,0,.06); border: none; font-size: .8rem; font-weight: 600; color: #5a7866; transition: background .14s;
}
.modal-btn-cancel:hover { background: rgba(0,0,0,.1); }
.modal-btn-save {
  display: inline-flex; align-items: center; gap: .4rem; padding: .45rem 1.2rem; border-radius: .65rem;
  cursor: pointer; background: linear-gradient(135deg, #1a5c38, #0f4226); border: none;
  color: #fff; font-size: .8rem; font-weight: 700; box-shadow: 0 3px 10px rgba(15,66,38,.28);
  transition: opacity .14s, transform .14s; min-width: 120px; justify-content: center;
}
.modal-btn-save:disabled { opacity: .5; cursor: not-allowed; transform: none; }
.modal-btn-save:not(:disabled):hover { opacity: .88; transform: translateY(-1px); }

.btn-spinner { display: inline-block; width: 14px; height: 14px; border-radius: 50%; border: 2px solid rgba(0,0,0,.15); border-top-color: #059669; animation: spin .7s linear infinite; }
.btn-spinner--light { border-color: rgba(255,255,255,.25); border-top-color: #fff; }
@keyframes spin { to { transform: rotate(360deg); } }

.modal-fade-enter-active { transition: all .22s ease-out; }
.modal-fade-leave-active { transition: all .17s ease-in; }
.modal-fade-enter-from, .modal-fade-leave-to { opacity: 0; }
.modal-fade-enter-from .modal-panel { transform: scale(.96) translateY(12px); }

/* ── Detail modal ─────────────────────────────────────────── */
.modal-panel--detail { max-width: 1024px; }

.dh-header {
  display: flex; align-items: center; gap: .85rem; padding: 1.15rem 1.35rem;
  border-bottom: 1px solid rgba(0,0,0,.07);
  background: linear-gradient(135deg, rgba(255,255,255,.55) 0%, rgba(240,249,244,.35) 100%);
}
.dh-avatar {
  width: 44px; height: 44px; border-radius: .85rem; flex-shrink: 0;
  font-size: 1.1rem; font-weight: 800; color: #fff;
  display: flex; align-items: center; justify-content: center;
}
.dha--active   { background: linear-gradient(135deg, #1a5c38, #0f4226); box-shadow: 0 3px 10px rgba(15,66,38,.28); }
.dha--inactive { background: linear-gradient(135deg, #6b7280, #4b5563); }
.dh-meta { flex: 1; display: flex; flex-direction: column; gap: .2rem; min-width: 0; }
.dh-name-row { display: flex; align-items: center; gap: .5rem; flex-wrap: wrap; }
.dh-name { font-size: 1rem; font-weight: 800; color: #0f2d1d; }

.btn-manage {
  display: inline-flex; align-items: center; gap: .3rem;
  padding: .3rem .75rem; border-radius: .6rem;
  background: rgba(45,143,86,.1); color: #2d8f56;
  font-size: .73rem; font-weight: 700; text-decoration: none; flex-shrink: 0;
  transition: background .14s;
}
.btn-manage:hover { background: rgba(45,143,86,.18); }

.detail-loading {
  display: flex; align-items: center; justify-content: center; gap: .75rem;
  padding: 2.5rem 1.35rem;
}

.detail-section {
  padding: 1.1rem 1.35rem;
  border-bottom: 1px solid rgba(0,0,0,.06);
}
.detail-section:last-child { border-bottom: none; }
.detail-section-title {
  font-size: .65rem; font-weight: 800; color: #7a9e8a;
  text-transform: uppercase; letter-spacing: .09em; margin: 0 0 .75rem;
}

.info-grid { display: flex; flex-direction: column; gap: .45rem; }
.info-row { display: flex; align-items: baseline; gap: .5rem; }
.info-lbl {
  flex-shrink: 0; width: 96px;
  font-size: .72rem; font-weight: 600; color: #8aaa96;
}
.info-val { font-size: .82rem; color: #0f2d1d; word-break: break-all; }
.info-val--mono { font-family: monospace; font-size: .75rem; color: #2d8f56; }

/* API Key section */
.apikey-section { background: rgba(251,243,220,.35); }
.apikey-section-header { display: flex; align-items: center; gap: .55rem; margin-bottom: .35rem; }
.apikey-icon {
  width: 28px; height: 28px; border-radius: .5rem; flex-shrink: 0;
  background: linear-gradient(135deg, #d97706, #b45309); color: #fff;
  display: flex; align-items: center; justify-content: center;
}
.apikey-hint { font-size: .73rem; color: #7a9e8a; margin: 0 0 .75rem; }
.apikey-bar {
  display: flex; align-items: center; gap: .35rem;
  background: rgba(255,255,255,.7); border: 1px solid rgba(217,119,6,.2);
  border-radius: .65rem; padding: .4rem .65rem; margin-bottom: .6rem;
}
.apikey-val {
  flex: 1; font-family: monospace; font-size: .75rem; color: #0f2d1d;
  word-break: break-all; letter-spacing: .03em; overflow: hidden;
  text-overflow: ellipsis; white-space: nowrap;
}
.apikey-btn {
  flex-shrink: 0; width: 26px; height: 26px; border: none; cursor: pointer;
  background: rgba(0,0,0,.05); border-radius: .4rem; color: #5a7866;
  display: flex; align-items: center; justify-content: center; transition: background .13s;
}
.apikey-btn:hover { background: rgba(0,0,0,.1); color: #0f2d1d; }

.regen-row { display: flex; }
.btn-regen {
  display: inline-flex; align-items: center; gap: .35rem;
  padding: .3rem .8rem; border-radius: .55rem; border: none; cursor: pointer;
  background: rgba(217,119,6,.1); color: #b45309;
  font-size: .73rem; font-weight: 600; transition: background .14s;
}
.btn-regen:hover { background: rgba(217,119,6,.18); }

.regen-confirm {
  border-radius: .75rem; border: 1px solid rgba(220,38,38,.18);
  background: rgba(254,242,242,.7); padding: .75rem .9rem;
}
.regen-warn {
  display: flex; align-items: flex-start; gap: .45rem;
  font-size: .75rem; color: #b91c1c; font-weight: 600; margin: 0 0 .65rem;
}
.regen-actions { display: flex; gap: .5rem; justify-content: flex-end; }
.btn-regen-cancel {
  padding: .3rem .85rem; border-radius: .5rem; border: none; cursor: pointer;
  background: rgba(0,0,0,.07); color: #5a7866; font-size: .75rem; font-weight: 600; transition: background .14s;
}
.btn-regen-cancel:hover { background: rgba(0,0,0,.12); }
.btn-regen-do {
  display: inline-flex; align-items: center; gap: .3rem; min-width: 110px; justify-content: center;
  padding: .3rem .9rem; border-radius: .5rem; border: none; cursor: pointer;
  background: linear-gradient(135deg, #dc2626, #b91c1c); color: #fff;
  font-size: .75rem; font-weight: 700; box-shadow: 0 2px 8px rgba(185,28,28,.25);
  transition: opacity .14s;
}
.btn-regen-do:disabled { opacity: .55; cursor: not-allowed; }
.btn-regen-do:not(:disabled):hover { opacity: .87; }

.modal-header, .dh-header, .modal-footer { flex-shrink: 0; }
.modal-footer { flex-wrap: wrap; }
.field, .info-row { min-width: 0; }
.outlet-detail-grid { display: grid; grid-template-columns: minmax(0, 1fr); min-height: 0; overflow-y: auto; }
@media (min-width: 768px) {
  .outlet-detail-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 639px) {
  .form-grid { grid-template-columns: minmax(0, 1fr); }
  .field-span2 { grid-column: 1 / -1; }
}
</style>
