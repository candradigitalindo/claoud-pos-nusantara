<!--
  DashboardLayout.vue — Modern Glassmorphism App Shell 2026
  Sidebar: deep emerald liquid glass + specular edges + accent-bar nav
  Topbar:  frosted white glass with backdrop-filter
-->
<template>
  <div class="app-shell">

    <!-- ── Mobile overlay ── -->
    <Transition name="fade-overlay">
      <div
        v-if="sidebarOpen"
        class="fixed inset-0 z-20 lg:hidden bg-overlay"
        @click="sidebarOpen = false"
      />
    </Transition>

    <!-- ── Sidebar ── -->
    <aside :class="['sidebar', sidebarOpen ? 'translate-x-0' : '-translate-x-full', 'lg:static lg:translate-x-0']">

      <!-- Specular edges -->
      <div class="sb-edge-right" aria-hidden="true" />
      <div class="sb-edge-top"   aria-hidden="true" />

      <!-- Brand -->
      <div class="sb-brand">
        <div class="brand-mark">
          <svg viewBox="0 0 32 32" fill="none">
            <rect x="3"  y="3"  width="12" height="12" rx="3.5" fill="white" fill-opacity=".9"/>
            <rect x="17" y="3"  width="12" height="12" rx="3.5" fill="white" fill-opacity=".4"/>
            <rect x="3"  y="17" width="12" height="12" rx="3.5" fill="white" fill-opacity=".55"/>
            <rect x="17" y="17" width="12" height="12" rx="3.5" fill="white" fill-opacity=".75"/>
          </svg>
        </div>
        <div class="brand-text">
          <span class="brand-name">Cloud POS</span>
          <span class="brand-sub">Nusantara</span>
        </div>
        <span class="version-badge">v2</span>
      </div>

      <!-- Nav -->
      <div class="sb-nav-wrap">
        <p class="sb-section-lbl">Navigation</p>
        <nav class="sb-nav">
          <template v-for="item in NAV_ITEMS" :key="item.to || item.label">
            <!-- Simple nav item (no children) -->
            <RouterLink
              v-if="!item.children"
              :to="item.to"
              :class="['nav-item', isActive(item.to) ? 'nav-item--active' : '']"
              @click="sidebarOpen = false; closeAllGroups()"
            >
              <span class="nav-accent" />
              <span class="nav-icon-wrap" v-html="item.icon" />
              <span class="nav-label">{{ item.label }}</span>
              <span v-if="isActive(item.to)" class="nav-active-dot" />
            </RouterLink>

            <!-- Dropdown group -->
            <div v-else class="nav-group">
              <button
                :class="['nav-item', 'nav-item--group', isGroupActive(item) ? 'nav-item--active' : '']"
                @click="toggleGroup(item.label)"
              >
                <span class="nav-accent" />
                <span class="nav-icon-wrap" v-html="item.icon" />
                <span class="nav-label">{{ item.label }}</span>
                <svg :class="['nav-chevron', openGroups[item.label] ? 'nav-chevron--open' : '']" fill="none" viewBox="0 0 20 20" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 8l4 4 4-4" />
                </svg>
              </button>
              <Transition name="dropdown">
                <div v-show="openGroups[item.label]" class="nav-children">
                  <RouterLink
                    v-for="child in item.children"
                    :key="child.to"
                    :to="child.to"
                    :class="['nav-child', isActive(child.to) ? 'nav-child--active' : '']"
                    @click="sidebarOpen = false"
                  >
                    <span class="nav-child-dot" />
                    <span class="nav-label">{{ child.label }}</span>
                  </RouterLink>
                </div>
              </Transition>
            </div>
          </template>
        </nav>
      </div>

    </aside>

    <!-- ── Main column ── -->
    <div class="main-col">

      <!-- Topbar -->
      <header class="topbar">
        <div class="tb-shimmer" aria-hidden="true" />

        <!-- Hamburger (mobile) -->
        <button class="hamburger lg:hidden" @click="sidebarOpen = true" aria-label="Open menu">
          <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h7"/>
          </svg>
        </button>

        <!-- Breadcrumb -->
        <div class="tb-crumb">
          <svg class="crumb-home" fill="none" viewBox="0 0 20 20" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8"
              d="M3 9.5L10 3l7 6.5V17a1 1 0 01-1 1H6a1 1 0 01-1-1v-4.5"/>
          </svg>
          <svg class="crumb-sep" fill="none" viewBox="0 0 12 20" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M4 4l6 6-6 6"/>
          </svg>
          <span class="crumb-active">{{ currentPageTitle }}</span>
        </div>

        <div class="flex-1" />

        <!-- Right actions -->
        <div class="tb-actions">
          <button class="tb-action-btn" title="Notifikasi">
            <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8"
                d="M15 17H9m3-14a7 7 0 017 7c0 3.87-.5 5.5-1 6H6c-.5-.5-1-2.13-1-6a7 7 0 017-7z"/>
            </svg>
          </button>
          <span class="tb-vdivider" />
          <div class="tb-user-block cursor-pointer" @click="showProfileModal = true">
            <div class="tb-avatar">{{ adminInitial }}</div>
            <div class="tb-user-meta">
              <span class="tb-user-name">{{ authStore.admin?.username }}</span>
              <span class="tb-user-role capitalize">{{ authStore.admin?.role }}</span>
            </div>
            <svg class="w-4 h-4 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 20 20" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 8l4 4 4-4"/>
            </svg>
          </div>
        </div>
      </header>

      <!-- Content -->
      <main class="content-area">
        <RouterView />
      </main>
    </div>

    <!-- ── Profile Modal ── -->
    <AppModal v-model="showProfileModal" title="Profil Saya" size="md">
      <div class="space-y-6">
        <!-- Profile Info -->
        <div class="flex items-center gap-4 p-4 bg-gray-50 rounded-xl">
          <div class="w-14 h-14 rounded-full bg-gradient-to-br from-emerald-500 to-emerald-700 flex items-center justify-center text-white text-xl font-bold shadow-md flex-shrink-0">
            {{ adminInitial }}
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-lg font-bold text-gray-900 truncate">{{ authStore.admin?.name || authStore.admin?.username }}</p>
            <p class="text-sm text-gray-500">@{{ authStore.admin?.username }}</p>
            <span class="inline-block mt-1 px-2.5 py-0.5 text-xs font-medium bg-emerald-100 text-emerald-700 rounded-full capitalize">{{ authStore.admin?.role }}</span>
          </div>
        </div>

        <!-- Edit Name -->
        <div>
          <h3 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
            <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
            Ubah Nama
          </h3>
          <div class="flex gap-2">
            <AppInput v-model="profileForm.name" placeholder="Nama lengkap" class="flex-1" />
            <AppButton :loading="profileLoading" @click="updateProfile" size="sm">Simpan</AppButton>
          </div>
          <p v-if="profileMsg" class="text-xs mt-1.5" :class="profileSuccess ? 'text-emerald-600' : 'text-red-600'">{{ profileMsg }}</p>
        </div>

        <!-- Change Password -->
        <div>
          <h3 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
            <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
            Ubah Password
          </h3>
          <div class="space-y-3">
            <AppInput v-model="pwForm.currentPassword" type="password" label="Password Lama" placeholder="Masukkan password lama" />
            <AppInput v-model="pwForm.newPassword" type="password" label="Password Baru" placeholder="Minimal 6 karakter" />
            <AppInput v-model="pwForm.confirmPassword" type="password" label="Konfirmasi Password Baru" placeholder="Ulangi password baru" />
          </div>
          <p v-if="pwMsg" class="text-xs mt-2" :class="pwSuccess ? 'text-emerald-600' : 'text-red-600'">{{ pwMsg }}</p>
          <AppButton class="mt-3 w-full" :loading="pwLoading" @click="changePassword">Ubah Password</AppButton>
        </div>

        <!-- Logout -->
        <div class="pt-3 border-t border-gray-200">
          <button class="w-full flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium text-red-600 hover:bg-red-50 rounded-lg transition-colors" @click="logout">
            <svg class="w-4 h-4" fill="none" viewBox="0 0 20 20" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8"
                d="M13 4.5l3.5 3.5L13 11.5M16.5 8H7M10 3H5a1 1 0 00-1 1v12a1 1 0 001 1h5"/>
            </svg>
            Logout
          </button>
        </div>
      </div>
    </AppModal>

    <ToastContainer />
  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth.js'
import { authApi } from '@/api/auth.js'
import ToastContainer from '@/components/ToastContainer.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppInput from '@/components/ui/AppInput.vue'
import AppButton from '@/components/ui/AppButton.vue'

const route       = useRoute()
const router      = useRouter()
const authStore   = useAuthStore()
const sidebarOpen = ref(false)
const openGroups  = reactive({})

// ── Profile modal state ──
const showProfileModal = ref(false)
const profileForm      = reactive({ name: authStore.admin?.name || '' })
const profileLoading   = ref(false)
const profileMsg       = ref('')
const profileSuccess   = ref(false)

const pwForm    = reactive({ currentPassword: '', newPassword: '', confirmPassword: '' })
const pwLoading = ref(false)
const pwMsg     = ref('')
const pwSuccess = ref(false)

async function updateProfile() {
  profileMsg.value = ''
  if (!profileForm.name.trim()) {
    profileMsg.value = 'Nama tidak boleh kosong'
    profileSuccess.value = false
    return
  }
  profileLoading.value = true
  try {
    const admin = await authApi.updateProfile(profileForm.name.trim())
    authStore.admin.name = admin.name
    profileMsg.value = 'Nama berhasil diperbarui'
    profileSuccess.value = true
  } catch (e) {
    profileMsg.value = e?.message || 'Gagal memperbarui profil'
    profileSuccess.value = false
  } finally {
    profileLoading.value = false
  }
}

async function changePassword() {
  pwMsg.value = ''
  if (!pwForm.currentPassword || !pwForm.newPassword) {
    pwMsg.value = 'Semua field password wajib diisi'
    pwSuccess.value = false
    return
  }
  if (pwForm.newPassword.length < 6) {
    pwMsg.value = 'Password baru minimal 6 karakter'
    pwSuccess.value = false
    return
  }
  if (pwForm.newPassword !== pwForm.confirmPassword) {
    pwMsg.value = 'Konfirmasi password tidak cocok'
    pwSuccess.value = false
    return
  }
  pwLoading.value = true
  try {
    await authApi.changePassword(pwForm.currentPassword, pwForm.newPassword)
    pwMsg.value = 'Password berhasil diubah'
    pwSuccess.value = true
    pwForm.currentPassword = ''
    pwForm.newPassword = ''
    pwForm.confirmPassword = ''
  } catch (e) {
    pwMsg.value = e?.message || 'Gagal mengubah password'
    pwSuccess.value = false
  } finally {
    pwLoading.value = false
  }
}

const adminInitial     = computed(() => (authStore.admin?.username ?? 'A').charAt(0).toUpperCase())
const currentPageTitle = computed(() => route.meta?.title?.replace(' — Cloud POS', '') ?? 'Dashboard')

// Penanda menu aktif.
//
// `startsWith` polos membuat menu induk ikut menyala di setiap anaknya: membuka
// /perlengkapan/dashboard juga menyalakan "Daftar Aset" (/perlengkapan), karena
// alamatnya memang berawalan sama. Karena itu sub-alamat hanya menyalakan menu
// ini bila TIDAK ADA menu lain yang cocok lebih spesifik.
//
// Halaman yang tidak punya menunya sendiri — misalnya detail aset
// /perlengkapan/<id> — tetap menyalakan menu induknya, dan itu memang yang
// diharapkan.
function isActive(to) {
  if (to === '/') return route.path === '/'
  if (route.path === to) return true
  if (!route.path.startsWith(to + '/')) return false
  return !ALL_NAV_PATHS.some(
    (p) => p !== to && p.length > to.length &&
      (route.path === p || route.path.startsWith(p + '/')),
  )
}

function isGroupActive(item) {
  return item.children?.some(c => isActive(c.to))
}

function closeAllGroups() {
  Object.keys(openGroups).forEach(k => { openGroups[k] = false })
}

function toggleGroup(label) {
  const willOpen = !openGroups[label]
  closeAllGroups()
  openGroups[label] = willOpen
}

async function logout() {
  showProfileModal.value = false
  await authStore.logout()
  router.push('/login')
}

const NAV_ITEMS_DATA = [
  {
    to: '/',
    label: 'Dashboard',
    permission: 'dashboard',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M4 5a1 1 0 011-1h4a1 1 0 011 1v5a1 1 0 01-1 1H5a1 1 0 01-1-1V5z"/>
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M14 5a1 1 0 011-1h4a1 1 0 011 1v2a1 1 0 01-1 1h-4a1 1 0 01-1-1V5z"/>
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M4 15a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1v-4z"/>
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M14 13a1 1 0 011-1h4a1 1 0 011 1v6a1 1 0 01-1 1h-4a1 1 0 01-1-1v-6z"/>
    </svg>`,
  },
  {
    label: 'Master Data',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M12 3c4.418 0 8 1.343 8 3s-3.582 3-8 3-8-1.343-8-3 3.582-3 8-3z"/>
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M4 6v6c0 1.657 3.582 3 8 3s8-1.343 8-3V6"/>
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M4 12v6c0 1.657 3.582 3 8 3s8-1.343 8-3v-6"/>
    </svg>`,
    children: [
      { to: '/outlets',    label: 'Outlet',     permission: 'outlets.view' },
      { to: '/work-units', label: 'Unit Kerja', permission: 'workunits.view' },
      { to: '/warehouses', label: 'Gudang',     permission: 'warehouses.view' },
      { to: '/roles',      label: 'Role',       permission: 'roles.view' },
      { to: '/app-pos',    label: 'App POS',     permission: 'appfiles.view' },
    ],
  },
  {
    to: '/products',
    label: 'Produk',
    permission: 'products.view',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10"/>
    </svg>`,
  },
  {
    label: 'Penjualan',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M8 7V3m8 4V3M3 11h18M5 5h14a2 2 0 012 2v12a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2z"/>
    </svg>`,
    children: [
      { to: '/reservations', label: 'Reservasi', permission: 'reservations.view' },
      { to: '/customers', label: 'Pelanggan', permission: 'customers.view' },
    ],
  },
  {
    label: 'Laporan',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/>
    </svg>`,
    children: [
      { to: '/sales-report',         label: 'Pendapatan', permission: 'reports.sales.view' },
      { to: '/cashier-shifts',       label: 'Shift Kasir', permission: 'cashier_shifts.view' },
      { to: '/shift-reconciliation', label: 'Rekonsiliasi Shift', permission: 'shift_reconciliation.view' },
      { to: '/procurement-payments',  label: 'Pembayaran', permission: 'finance.payments.view' },
      { to: '/product-sales-report', label: 'Penjualan Produk', permission: 'reports.product_sales.view' },
      { to: '/general-ledger',       label: 'Buku Besar', permission: 'reports.ledger.view' },
      { to: '/cash-flow-report',     label: 'Pemasukan & Pengeluaran', permission: 'reports.cashflow.view' },
      { to: '/profit-loss-report',   label: 'Profit & Loss', permission: 'reports.pnl.view' },
      { to: '/balance-report',       label: 'Laporan Neraca', permission: 'reports.balance.view' },
      { to: '/tax-report',           label: 'Laporan Pajak', permission: 'reports.tax.view' },
      { to: '/void-report',          label: 'Void & Titipan', permission: ['reports.void.view', 'reports.titipan.view'] },
      { to: '/discount-report',      label: 'Diskon & Komplimen', permission: 'reports.discount.view' },
      { to: '/business-analysis',    label: 'Analisa Bisnis', permission: 'reports.business_analysis.view' },
      { to: '/social-performance',   label: 'Kinerja Markom', permission: 'social.view' },
      { to: '/bank-accounts',         label: 'Data Rekening', permission: 'finance.bank.view' },
    ],
  },
  {
    label: 'Pengadaan',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M2.25 3h1.386c.51 0 .955.343 1.087.835l.383 1.437M7.5 14.25a3 3 0 00-3 3h15.75m-12.75-3h11.218c1.121-2.3 2.1-4.684 2.924-7.138a60.114 60.114 0 00-16.536-1.84M7.5 14.25L5.106 5.272M6 20.25a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm12.75 0a.75.75 0 11-1.5 0 .75.75 0 011.5 0z"/>
    </svg>`,
    children: [
      { to: '/procurement-dashboard', label: 'Dashboard', permission: 'procurement.dashboard.view' },
      { to: '/purchase-goods', label: 'Barang', permission: 'procurement.requests.view' },
      { to: '/purchase-services', label: 'Jasa', permission: 'procurement.requests.view' },
      { to: '/projects', label: 'Projek', permission: 'procurement.projects.view' },
      { to: '/vendors', label: 'Vendor', permission: 'vendors.view' },
    ],
  },
  {
    label: 'PPIC',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.7" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.7" d="M8 12l2.5 2.5L15 10" />
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.7" d="M8 17h5" />
    </svg>`,
    children: [
      { to: '/ppic/dashboard',       label: 'Dashboard', permission: 'ppic.dashboard.view' },
      { to: '/ppic/planning-params', label: 'Par Level & ROP', permission: 'ppic.planning.view' },
      { to: '/ppic/expiry',          label: 'Monitor Kedaluwarsa', permission: 'ppic.expiry.view' },
      { to: '/ppic/opname',          label: 'Stock Opname', permission: 'ppic.opname.view' },
      { to: '/ppic/forecast',        label: 'Demand Forecast', permission: 'ppic.forecast.view' },
      { to: '/ppic/mrp',             label: 'Kebutuhan Bahan (MRP)', permission: 'ppic.mrp.view' },
      { to: '/ppic/production-plans', label: 'Rencana Produksi', permission: 'ppic.production.view' },
      { to: '/ppic/work-orders',     label: 'Work Order', permission: 'ppic.workorders.view' },
      { to: '/ppic/reports',         label: 'Laporan PPIC', permission: 'ppic.reports.view' },
    ],
  },
  {
    label: 'Pengguna',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M16 7a4 4 0 11-8 0 4 4 0 018 0z"/>
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
    </svg>`,
    children: [
      { to: '/admins', label: 'User', permission: 'users.view' },
    ],
  },
  {
    label: 'Gudang',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10"/>
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M21 6v12a2 2 0 01-2 2H5a2 2 0 01-2-2V6m2 4h3m-3 4h2.5m-.5 4h.5"/>
    </svg>`,
    children: [
      { to: '/warehouse-dashboard', label: 'Dashboard', permission: 'warehouse_dashboard.view' },
      { to: '/stock-items',     label: 'Item Stok', permission: 'stockitems.view' },
      { to: '/goods-receipts',  label: 'Penerimaan Barang', permission: 'stockledger.view' },
      { to: '/gudang/penerimaan-pengadaan', label: 'Penerimaan dari Pengadaan', permission: 'stockledger.adjust' },
      { to: '/stock-transfers', label: 'Transfer Stok', permission: 'stocktransfers.view' },
      { to: '/stock-wastes',    label: 'Stok Rusak/Hilang', permission: 'stockwastes.view' },
      { to: '/stock-ledger',    label: 'Buku Stok', permission: 'stockledger.view' },
      { to: '/recipes',         label: 'Resep', permission: 'recipes.view' },
    ],
  },
  {
    label: 'Perlengkapan',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M3 7l9-4 9 4v10l-9 4-9-4V7z"/>
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M3 7l9 4 9-4M12 11v10"/>
    </svg>`,
    children: [
      { to: '/perlengkapan/dashboard', label: 'Dashboard Aset',    permission: 'assets.dashboard.view' },
      { to: '/perlengkapan',        label: 'Daftar Aset',         permission: 'assets.view' },
      { to: '/perlengkapan/penerimaan', label: 'Penerimaan Peralatan', permission: 'assets.create' },
      { to: '/perlengkapan/distribusi', label: 'Distribusi ke PIC', permission: 'assets.view' },
      { to: '/perlengkapan/mutasi', label: 'Mutasi Antar Outlet', permission: 'assets.transfer.view' },
      { to: '/perlengkapan/perawatan', label: 'Perawatan', permission: 'assets.maintenance.view' },
      { to: '/perlengkapan/opname', label: 'Opname Aset', permission: 'assets.opname.view' },
      { to: '/perlengkapan/penghapusan', label: 'Penghapusan', permission: 'assets.disposal.view' },
      { to: '/perlengkapan/laporan', label: 'Laporan Aset', permission: 'assets.report.view' },
      { to: '/perlengkapan/kategori', label: 'Kategori Aset', permission: 'assets.view' },
    ],
  },
  {
    label: 'Pengaturan',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.573-1.066z"/>
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
    </svg>`,
    children: [
      { to: '/settings/company',  label: 'Identitas Perusahaan', permission: 'settings.company.view' },
      { to: '/settings/photo-backup', label: 'Cadangan Bukti Foto', permission: 'settings.company.view' },
      { to: '/settings/timezone', label: 'Zona Waktu', permission: 'settings.timezone.view' },
      { to: '/settings/tax',     label: 'Pajak', permission: 'settings.tax.view' },
      { to: '/settings/devices', label: 'Perangkat', permission: 'devices.view' },
    ],
  },
  {
    to: '/access-logs',
    label: 'Log Akses',
    permission: 'access_logs.view',
    icon: `<svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
    </svg>`,
  },
]

const NAV_ITEMS = computed(() =>
  NAV_ITEMS_DATA
    .filter(item => (!item.superadmin || authStore.isSuperadmin) && (!item.permission || authStore.hasPermission(item.permission)))
    .map(item => {
      if (!item.children) return item
      const filtered = item.children.filter(c => !c.permission || authStore.hasPermission(c.permission))
      return { ...item, children: filtered }
    })
    .filter(item => !item.children || item.children.length > 0)
)

// Seluruh alamat yang punya menunya sendiri — dipakai isActive untuk memilih
// menu yang paling spesifik.
const ALL_NAV_PATHS = NAV_ITEMS_DATA.flatMap((item) =>
  item.to ? [item.to] : (item.children || []).map((c) => c.to),
).filter(Boolean)

// Auto-open group if a child route is active
NAV_ITEMS_DATA.forEach(item => {
  if (item.children?.some(c => route.path.startsWith(c.to))) {
    openGroups[item.label] = true
  }
})
</script>

<style scoped>
/* ══════════════════════════════ SHELL ══════════════════════════════ */
.app-shell {
  display: flex; height: 100svh; overflow: hidden; background: #eef2ee;
}
.bg-overlay { background: rgba(0,0,0,.4); backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px); }
.fade-overlay-enter-active, .fade-overlay-leave-active { transition: opacity .25s; }
.fade-overlay-enter-from,   .fade-overlay-leave-to     { opacity: 0; }

/* ══════════════════════════════ SIDEBAR ═══════════════════════════ */
.sidebar {
  position: fixed; top: 0; bottom: 0; left: 0;
  z-index: 30; width: 230px;
  display: flex; flex-direction: column;
  background:
    linear-gradient(135deg, rgba(255,255,255,.10) 0%, rgba(255,255,255,.03) 40%, rgba(0,0,0,.06) 100%),
    linear-gradient(175deg, #0d3520 0%, #0f4226 55%, #0b2e1c 100%);
  backdrop-filter: blur(40px) saturate(180%) brightness(1.05);
  -webkit-backdrop-filter: blur(40px) saturate(180%) brightness(1.05);
  border-right: 1px solid rgba(255,255,255,.07);
  box-shadow: 6px 0 40px rgba(0,0,0,.35), 1px 0 0 rgba(255,255,255,.04) inset;
  transition: transform .22s cubic-bezier(.22,.68,0,1.15);
  overflow: hidden;
}
@media (min-width: 1024px) { .sidebar { position: static; flex-shrink: 0; } }

/* Specular edges */
.sb-edge-right {
  position: absolute; top: 10%; right: 0; bottom: 10%; width: 1px;
  background: linear-gradient(180deg, transparent, rgba(255,255,255,.2) 30%, rgba(255,255,255,.28) 55%, rgba(255,255,255,.2) 75%, transparent);
  pointer-events: none; z-index: 2;
}
.sb-edge-top {
  position: absolute; top: 0; left: 0; right: 0; height: 1px;
  background: linear-gradient(90deg, rgba(255,255,255,0), rgba(255,255,255,.27) 35%, rgba(255,255,255,.34) 55%, rgba(255,255,255,.27) 75%, rgba(255,255,255,0));
  pointer-events: none; z-index: 2;
}

/* Brand */
.sb-brand {
  position: relative; z-index: 1;
  display: flex; align-items: center; gap: .7rem;
  padding: 1.4rem 1.1rem 1.1rem;
}
.brand-mark {
  width: 38px; height: 38px; border-radius: 11px; flex-shrink: 0;
  background: linear-gradient(135deg, rgba(255,255,255,.2), rgba(255,255,255,.06));
  border: 1px solid rgba(255,255,255,.18);
  display: flex; align-items: center; justify-content: center;
  box-shadow: inset 0 1px 0 rgba(255,255,255,.28), 0 4px 16px rgba(0,0,0,.3);
}
.brand-mark svg { width: 22px; height: 22px; }
.brand-text { display: flex; flex-direction: column; flex: 1; min-width: 0; }
.brand-name { font-size: .875rem; font-weight: 700; color: #fff; letter-spacing: -.015em; line-height: 1.2; }
.brand-sub  { font-size: .6rem; color: rgba(255,255,255,.35); text-transform: uppercase; letter-spacing: .1em; margin-top: .12rem; }
.version-badge {
  font-size: .58rem; font-weight: 700; color: rgba(255,255,255,.5);
  background: rgba(255,255,255,.08); border: 1px solid rgba(255,255,255,.12);
  border-radius: 4px; padding: .1rem .32rem; align-self: flex-start; letter-spacing: .04em;
}

/* Nav */
.sb-nav-wrap {
  position: relative; z-index: 1; padding: 0 .65rem;
  flex: 1; overflow-y: auto; overflow-x: hidden;
  scrollbar-width: thin; scrollbar-color: rgba(255,255,255,.1) transparent;
}
.sb-nav-wrap::-webkit-scrollbar { width: 4px; }
.sb-nav-wrap::-webkit-scrollbar-thumb { background: rgba(255,255,255,.1); border-radius: 9999px; }
.sb-section-lbl {
  font-size: .575rem; font-weight: 700; text-transform: uppercase;
  letter-spacing: .12em; color: rgba(255,255,255,.22);
  padding: .75rem .5rem .4rem; margin: 0;
}
.sb-nav { display: flex; flex-direction: column; gap: .18rem; }

.nav-item {
  position: relative;
  display: flex; align-items: center; gap: .65rem;
  padding: .625rem .75rem .625rem .9rem;
  border-radius: 10px; font-size: .82rem; font-weight: 500;
  color: rgba(255,255,255,.45); text-decoration: none;
  border: 1px solid transparent; overflow: hidden;
  transition: color .18s, background .18s, border-color .18s;
}
.nav-item:hover:not(.nav-item--active) {
  color: rgba(255,255,255,.78); background: rgba(255,255,255,.07); border-color: rgba(255,255,255,.06);
}
.nav-item--active {
  color: #fff; font-weight: 600;
  background: linear-gradient(135deg, rgba(255,255,255,.18) 0%, rgba(255,255,255,.08) 100%);
  border-color: rgba(255,255,255,.15);
  box-shadow: inset 0 1px 0 rgba(255,255,255,.2), 0 4px 16px rgba(0,0,0,.25);
}

/* Left accent bar */
.nav-accent {
  position: absolute; left: 0; top: 18%; bottom: 18%;
  width: 3px; border-radius: 0 3px 3px 0;
  background: transparent; transition: background .18s, box-shadow .18s;
}
.nav-item--active .nav-accent {
  background: linear-gradient(180deg, #6ee7a0, #34d371);
  box-shadow: 0 0 10px rgba(110,231,160,.55);
}

/* Icon */
.nav-icon-wrap {
  width: 1.15rem; height: 1.15rem; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  opacity: .65; transition: opacity .18s;
}
.nav-item--active .nav-icon-wrap { opacity: 1; }
.nav-icon-wrap :deep(svg) { width: 100%; height: 100%; }
.nav-label { flex: 1; }
.nav-active-dot {
  width: 5px; height: 5px; border-radius: 50%; flex-shrink: 0;
  background: #6ee7a0;
  box-shadow: 0 0 6px #6ee7a0, 0 0 12px rgba(110,231,160,.4);
}

/* Dropdown group */
.nav-group { display: flex; flex-direction: column; }
.nav-item--group {
  cursor: pointer; background: none; border: none; width: 100%;
  font-family: inherit; text-align: left;
}
.nav-chevron {
  width: 14px; height: 14px; flex-shrink: 0;
  color: rgba(255,255,255,.3);
  transition: transform .2s ease, color .18s;
}
.nav-chevron--open { transform: rotate(180deg); color: rgba(255,255,255,.55); }
.nav-item--active .nav-chevron { color: rgba(255,255,255,.7); }

.nav-children {
  display: flex; flex-direction: column; gap: .1rem;
  padding: .15rem 0 .25rem 0;
  overflow: hidden;
}
.nav-child {
  display: flex; align-items: center; gap: .6rem;
  padding: .45rem .75rem .45rem 2.5rem;
  border-radius: 8px; font-size: .78rem; font-weight: 500;
  color: rgba(255,255,255,.4); text-decoration: none;
  transition: color .18s, background .18s;
}
.nav-child:hover { color: rgba(255,255,255,.72); background: rgba(255,255,255,.05); }
.nav-child--active {
  color: #a7f3c8; font-weight: 600;
  background: rgba(255,255,255,.08);
}
.nav-child-dot {
  width: 5px; height: 5px; border-radius: 50%; flex-shrink: 0;
  background: rgba(255,255,255,.2);
  transition: background .18s, box-shadow .18s;
}
.nav-child--active .nav-child-dot {
  background: #6ee7a0;
  box-shadow: 0 0 6px rgba(110,231,160,.5);
}

/* Dropdown transition */
.dropdown-enter-active, .dropdown-leave-active {
  transition: max-height .25s ease, opacity .2s ease;
}
.dropdown-enter-from, .dropdown-leave-to {
  max-height: 0; opacity: 0;
}
.dropdown-enter-to, .dropdown-leave-from {
  max-height: 200px; opacity: 1;
}



/* ══════════════════════════════ TOPBAR ════════════════════════════ */
.topbar {
  position: relative; z-index: 10; flex-shrink: 0;
  height: 56px;
  display: flex; align-items: center; gap: .75rem;
  padding: 0 1.5rem;
  background: rgba(243,247,243,.82);
  backdrop-filter: blur(28px) saturate(180%);
  -webkit-backdrop-filter: blur(28px) saturate(180%);
  border-bottom: 1px solid rgba(0,0,0,.07);
  box-shadow: 0 1px 0 rgba(255,255,255,.85) inset, 0 2px 12px rgba(0,0,0,.07);
}
/* Green shimmer on bottom edge */
.tb-shimmer {
  position: absolute; bottom: -1px; left: 5%; right: 5%; height: 1px;
  background: linear-gradient(90deg, transparent, rgba(52,211,113,.25) 35%, rgba(110,231,160,.4) 55%, rgba(52,211,113,.25) 75%, transparent);
  pointer-events: none;
}

.hamburger {
  padding: .4rem; border-radius: 8px;
  color: #4b7a5e; background: none; border: none; cursor: pointer;
  transition: color .15s, background .15s;
}
.hamburger:hover { color: #14532d; background: rgba(0,0,0,.06); }
.hamburger svg { width: 20px; height: 20px; display: block; }

/* Breadcrumb */
.tb-crumb   { display: flex; align-items: center; gap: .4rem; }
.crumb-home { width: 15px; height: 15px; color: #86a893; flex-shrink: 0; }
.crumb-sep  { width: 8px; height: 12px; color: #b0c4b8; flex-shrink: 0; }
.crumb-active { font-size: .875rem; font-weight: 600; color: #1a4731; letter-spacing: -.015em; }

/* Right actions */
.tb-actions { display: flex; align-items: center; gap: .6rem; }
.tb-action-btn {
  width: 34px; height: 34px; border-radius: 9px;
  background: rgba(255,255,255,.55); border: 1px solid rgba(0,0,0,.07); color: #4b7a5e;
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  box-shadow: 0 1px 3px rgba(0,0,0,.06), inset 0 1px 0 rgba(255,255,255,.8);
  transition: background .15s, box-shadow .15s;
}
.tb-action-btn:hover { background: rgba(255,255,255,.9); box-shadow: 0 2px 8px rgba(0,0,0,.1), inset 0 1px 0 rgba(255,255,255,.9); color: #14532d; }
.tb-action-btn svg { width: 16px; height: 16px; display: block; }
.tb-vdivider { width: 1px; height: 20px; background: rgba(0,0,0,.08); }
.tb-user-block {
  display: flex; align-items: center; gap: .55rem;
  padding: .35rem .5rem .35rem .35rem;
  border-radius: 12px;
  background: rgba(255,255,255,.5);
  border: 1px solid rgba(0,0,0,.07);
  box-shadow: 0 1px 3px rgba(0,0,0,.06), inset 0 1px 0 rgba(255,255,255,.8);
}
.tb-avatar {
  width: 32px; height: 32px; border-radius: 50%; flex-shrink: 0;
  background: linear-gradient(135deg, #22c55e, #16a34a);
  border: 2px solid rgba(255,255,255,.7);
  display: flex; align-items: center; justify-content: center;
  font-size: .72rem; font-weight: 700; color: #fff;
  box-shadow: 0 0 0 2px rgba(34,197,94,.2), 0 2px 6px rgba(0,0,0,.15);
}
.tb-user-meta { display: flex; flex-direction: column; min-width: 0; }
.tb-user-name { font-size: .78rem; font-weight: 600; color: #1a4731; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; line-height: 1.2; }
.tb-user-role { font-size: .6rem; color: #6b8f7b; margin-top: .05rem; line-height: 1.2; }

/* ══════════════════════════ MAIN + CONTENT ═════════════════════════ */
.main-col {
  flex: 1; display: flex; flex-direction: column;
  overflow: hidden; min-width: 0; background: #eef2ee;
}
.content-area {
  flex: 1; overflow-y: auto; padding: 1.5rem; background: #eef2ee;
  scrollbar-width: thin; scrollbar-color: rgba(0,0,0,.12) transparent;
}
.content-area::-webkit-scrollbar { width: 5px; }
.content-area::-webkit-scrollbar-thumb { background: rgba(0,0,0,.1); border-radius: 9999px; }
</style>
