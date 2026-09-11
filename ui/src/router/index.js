/**
 * src/router/index.js — Client-side routing
 *
 * Route structure:
 *   /login                       → Login page (public)
 *   /                            → DashboardLayout wrapper
 *     /dashboard                 → Overview stats
 *     /outlets                   → Outlet list
 *     /outlets/:id               → Outlet detail (orders, txn, products, etc.)
 *     /outlets/:id/transactions  → Outlet transactions
 *     /outlets/:id/orders        → Outlet orders
 *     /outlets/:id/products      → Outlet products
 *     /outlets/:id/categories    → Outlet categories
 *     /outlets/:id/printers      → Outlet printers
 *     /outlets/:id/sync-logs     → Outlet sync logs
 *     /outlets/:id/conflicts     → Outlet conflicts
 *     /admins                    → Admin user management
 *
 * Navigation guard:
 *   - Routes with `meta.requiresAuth: true` redirect to /login if no token.
 *   - /login redirects to /dashboard if already authenticated.
 *
 * AI NOTE: To add a new protected page:
 *   1. Create the .vue file in src/pages/
 *   2. Add the route object below with `meta: { requiresAuth: true }`
 *   3. Add a nav link in src/layouts/DashboardLayout.vue
 */

import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth.js'

// ── Lazy-loaded page components ─────────────────────────────
// Each import() creates a separate JS chunk → better initial load time
const Login          = () => import('@/pages/Login.vue')
const Dashboard      = () => import('@/pages/Dashboard.vue')
const Outlets        = () => import('@/pages/Outlets.vue')
const OutletDetail   = () => import('@/pages/OutletDetail.vue')
const Transactions   = () => import('@/pages/outlet/Transactions.vue')
const Orders         = () => import('@/pages/outlet/Orders.vue')
const Products       = () => import('@/pages/outlet/Products.vue')
const Categories     = () => import('@/pages/outlet/Categories.vue')
const Printers       = () => import('@/pages/outlet/Printers.vue')
const SyncLogs       = () => import('@/pages/outlet/SyncLogs.vue')
const Conflicts      = () => import('@/pages/outlet/Conflicts.vue')
const OutletInfo     = () => import('@/pages/outlet/OutletInfo.vue')
const ProcurementDashboard = () => import('@/pages/outlet/ProcurementDashboard.vue')
const ProductsAdmin       = () => import('@/pages/Products.vue')
const SalesReport         = () => import('@/pages/SalesReport.vue')
const CashierShiftReport  = () => import('@/pages/CashierShiftReport.vue')
const ShiftReconciliation = () => import('@/pages/ShiftReconciliation.vue')
const ProductSalesReport  = () => import('@/pages/ProductSalesReport.vue')
const Assets              = () => import('@/pages/Assets.vue')
const Reservations        = () => import('@/pages/Reservations.vue')
const Customers           = () => import('@/pages/Customers.vue')
const ReservePublic       = () => import('@/pages/ReservePublic.vue')
const NotFound            = () => import('@/pages/NotFound.vue')
const Forbidden           = () => import('@/pages/Forbidden.vue')
const TaxReport           = () => import('@/pages/TaxReport.vue')
const CashFlowReport      = () => import('@/pages/CashFlowReport.vue')
const BalanceReport       = () => import('@/pages/BalanceReport.vue')
const ProfitLossReport    = () => import('@/pages/ProfitLossReport.vue')
const GeneralLedger       = () => import('@/pages/GeneralLedger.vue')
const VoidReport          = () => import('@/pages/VoidReport.vue')
const DiscountReport      = () => import('@/pages/DiscountReport.vue')
const BusinessAnalysis    = () => import('@/pages/BusinessAnalysis.vue')
const SocialAccounts      = () => import('@/pages/SocialAccounts.vue')
const PurchaseGoods       = () => import('@/pages/PurchaseGoods.vue')
const PurchaseServices    = () => import('@/pages/PurchaseServices.vue')
const ProcurementDashboardPage = () => import('@/pages/ProcurementDashboard.vue')
const ProcurementPayments     = () => import('@/pages/ProcurementPayments.vue')
const WorkUnits           = () => import('@/pages/WorkUnits.vue')
const Vendors             = () => import('@/pages/Vendors.vue')
const VendorDetail        = () => import('@/pages/VendorDetail.vue')
const Projects            = () => import('@/pages/Projects.vue')
const ProjectDetail       = () => import('@/pages/ProjectDetail.vue')
const Admins              = () => import('@/pages/Admins.vue')
const Roles          = () => import('@/pages/Roles.vue')
const AppPos         = () => import('@/pages/AppPos.vue')
const AccessLogs     = () => import('@/pages/AccessLogs.vue')
const BankAccounts    = () => import('@/pages/BankAccounts.vue')
const ManagerDashboard = () => import('@/pages/ManagerDashboard.vue')
const CompanyIdentity     = () => import('@/pages/CompanyIdentity.vue')
const TimezoneSettings    = () => import('@/pages/TimezoneSettings.vue')
const TaxSettings         = () => import('@/pages/TaxSettings.vue')
const DeviceMonitor       = () => import('@/pages/DeviceMonitor.vue')
const Warehouses          = () => import('@/pages/Warehouses.vue')
const WarehouseDashboard  = () => import('@/pages/WarehouseDashboard.vue')
const StockItems          = () => import('@/pages/StockItems.vue')
const StockWastes         = () => import('@/pages/StockWastes.vue')
const StockTransfers      = () => import('@/pages/StockTransfers.vue')
const StockLedger         = () => import('@/pages/StockLedger.vue')
const GoodsReceipt        = () => import('@/pages/GoodsReceipt.vue')
const Recipes             = () => import('@/pages/Recipes.vue')
const PpicDashboard       = () => import('@/pages/ppic/PpicDashboard.vue')
const PlanningParams      = () => import('@/pages/ppic/PlanningParams.vue')
const ExpiryMonitor       = () => import('@/pages/ppic/ExpiryMonitor.vue')
const StockOpname         = () => import('@/pages/ppic/StockOpname.vue')
const DemandForecast      = () => import('@/pages/ppic/DemandForecast.vue')
const MrpWorksheet        = () => import('@/pages/ppic/MrpWorksheet.vue')
const ProductionPlans     = () => import('@/pages/ppic/ProductionPlans.vue')
const WorkOrders          = () => import('@/pages/ppic/WorkOrders.vue')
const PpicReports         = () => import('@/pages/ppic/PpicReports.vue')
const DashboardLayout = () => import('@/layouts/DashboardLayout.vue')
const AuthLayout     = () => import('@/layouts/AuthLayout.vue')

// ── Route definitions ────────────────────────────────────────
const routes = [
  // ── Public reservation page (no auth, no sidebar) ───────
  {
    path: '/r/:slug',
    name: 'ReservePublic',
    component: ReservePublic,
    meta: { title: 'Reservasi — Cloud POS' },
  },

  // ── Auth layout (no sidebar) ────────────────────────────
  {
    path: '/login',
    component: AuthLayout,
    children: [
      {
        path: '',
        name: 'Login',
        component: Login,
        meta: { title: 'Login — Cloud POS' },
      },
    ],
  },

  // ── Admin layout (with sidebar) ─────────────────────────
  {
    path: '/',
    component: DashboardLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: ManagerDashboard,
        meta: { title: 'Dashboard — Cloud POS', requiresAuth: true, permission: 'dashboard' },
      },
      // Alias lama → arahkan ke dashboard utama
      { path: 'manager-dashboard', redirect: '/' },
      {
        path: 'outlets',
        name: 'Outlets',
        component: Outlets,
        meta: { title: 'Outlets — Cloud POS', requiresAuth: true, permission: 'outlets.view' },
      },
      {
        path: 'outlets/:id',
        component: OutletDetail,
        meta: { title: 'Detail Outlet — Cloud POS', requiresAuth: true, permission: 'outlets.view' },
        // Outlet sub-pages are CHILDREN so OutletDetail's <RouterView> renders them.
        children: [
          { path: '',             redirect: 'info' },
          {
            path: 'info',
            name: 'OutletInfo',
            component: OutletInfo,
            meta: { title: 'Info Outlet — Cloud POS', requiresAuth: true },
          },
          {
            path: 'transactions',
            name: 'Transactions',
            component: Transactions,
            meta: { title: 'Transaksi — Cloud POS', requiresAuth: true },
          },
          {
            path: 'orders',
            name: 'Orders',
            component: Orders,
            meta: { title: 'Pesanan — Cloud POS', requiresAuth: true },
          },
          {
            path: 'products',
            name: 'Products',
            component: Products,
            meta: { title: 'Produk — Cloud POS', requiresAuth: true },
          },
          {
            path: 'categories',
            name: 'Categories',
            component: Categories,
            meta: { title: 'Kategori — Cloud POS', requiresAuth: true },
          },
          {
            path: 'printers',
            name: 'Printers',
            component: Printers,
            meta: { title: 'Printer — Cloud POS', requiresAuth: true },
          },
          {
            path: 'sync-logs',
            name: 'SyncLogs',
            component: SyncLogs,
            meta: { title: 'Sync Logs — Cloud POS', requiresAuth: true },
          },
          {
            path: 'conflicts',
            name: 'Conflicts',
            component: Conflicts,
            meta: { title: 'Konflik Sync — Cloud POS', requiresAuth: true },
          },
          {
            path: 'procurement',
            name: 'ProcurementDashboard',
            component: ProcurementDashboard,
            meta: { title: 'Pengadaan — Cloud POS', requiresAuth: true, permission: 'procurement.dashboard.view' },
          },
        ],
      },
      {
        path: 'admins',
        name: 'Admins',
        component: Admins,
        meta: { title: 'User — Cloud POS', requiresAuth: true, permission: 'users.view' },
      },
      {
        path: 'roles',
        name: 'Roles',
        component: Roles,
        meta: { title: 'Role — Cloud POS', requiresAuth: true, permission: 'roles.view' },
      },
      {
        path: 'app-pos',
        name: 'AppPos',
        component: AppPos,
        meta: { title: 'App POS — Cloud POS', requiresAuth: true, permission: 'appfiles.view' },
      },
      {
        path: 'access-logs',
        name: 'AccessLogs',
        component: AccessLogs,
        meta: { title: 'Log Akses — Cloud POS', requiresAuth: true, permission: 'access_logs.view' },
      },
      {
        path: 'products',
        name: 'ProductsAdmin',
        component: ProductsAdmin,
        meta: { title: 'Produk & Kategori — Cloud POS', requiresAuth: true, permission: 'products.view' },
      },
      {
        path: 'sales-report',
        name: 'SalesReport',
        component: SalesReport,
        meta: { title: 'Laporan Pendapatan — Cloud POS', requiresAuth: true, permission: 'reports.sales.view' },
      },
      {
        path: 'cashier-shifts',
        name: 'CashierShiftReport',
        component: CashierShiftReport,
        meta: { title: 'Laporan Shift Kasir — Cloud POS', requiresAuth: true, permission: 'cashier_shifts.view' },
      },
      {
        path: 'shift-reconciliation',
        name: 'ShiftReconciliation',
        component: ShiftReconciliation,
        meta: { title: 'Rekonsiliasi Shift — Cloud POS', requiresAuth: true, permission: 'shift_reconciliation.view' },
      },
      {
        path: 'product-sales-report',
        name: 'ProductSalesReport',
        component: ProductSalesReport,
        meta: { title: 'Penjualan Produk — Cloud POS', requiresAuth: true, permission: 'reports.product_sales.view' },
      },
      {
        path: 'tax-report',
        name: 'TaxReport',
        component: TaxReport,
        meta: { title: 'Laporan Pajak — Cloud POS', requiresAuth: true, permission: 'reports.tax.view' },
      },
      {
        path: 'cash-flow-report',
        name: 'CashFlowReport',
        component: CashFlowReport,
        meta: { title: 'Pemasukan & Pengeluaran — Cloud POS', requiresAuth: true, permission: 'reports.cashflow.view' },
      },
      {
        path: 'balance-report',
        name: 'BalanceReport',
        component: BalanceReport,
        meta: { title: 'Laporan Neraca — Cloud POS', requiresAuth: true, permission: 'reports.balance.view' },
      },
      {
        path: 'profit-loss-report',
        name: 'ProfitLossReport',
        component: ProfitLossReport,
        meta: { title: 'Profit & Loss — Cloud POS', requiresAuth: true, permission: 'reports.pnl.view' },
      },
      {
        path: 'general-ledger',
        name: 'GeneralLedger',
        component: GeneralLedger,
        meta: { title: 'Buku Besar — Cloud POS', requiresAuth: true, permission: 'reports.ledger.view' },
      },
      {
        path: 'void-report',
        name: 'VoidReport',
        component: VoidReport,
        meta: { title: 'Void & Titipan — Cloud POS', requiresAuth: true, permission: ['reports.void.view', 'reports.titipan.view'] },
      },
      {
        path: 'discount-report',
        name: 'DiscountReport',
        component: DiscountReport,
        meta: { title: 'Laporan Diskon & Komplimen — Cloud POS', requiresAuth: true, permission: 'reports.discount.view' },
      },
      {
        path: 'business-analysis',
        name: 'BusinessAnalysis',
        component: BusinessAnalysis,
        meta: { title: 'Analisa Bisnis — Cloud POS', requiresAuth: true, permission: 'reports.business_analysis.view' },
      },
      {
        path: 'social-performance',
        name: 'SocialAccounts',
        meta: { title: 'Kinerja Markom — Cloud POS', requiresAuth: true, permission: 'social.view' },
        component: SocialAccounts,
      },
      {
        path: 'procurement-payments',
        name: 'ProcurementPayments',
        component: ProcurementPayments,
        meta: { title: 'Pembayaran — Cloud POS', requiresAuth: true, permission: 'finance.payments.view' },
      },
      {
        path: 'bank-accounts',
        name: 'BankAccounts',
        component: BankAccounts,
        meta: { title: 'Rekening — Cloud POS', requiresAuth: true, permission: 'finance.bank.view' },
      },
      {
        path: 'procurement-dashboard',
        name: 'ProcurementDashboardPage',
        component: ProcurementDashboardPage,
        meta: { title: 'Dashboard Pengadaan — Cloud POS', requiresAuth: true, permission: 'procurement.dashboard.view' },
      },
      {
        path: 'purchase-goods',
        name: 'PurchaseGoods',
        component: PurchaseGoods,
        meta: { title: 'Pengadaan Barang — Cloud POS', requiresAuth: true, permission: 'procurement.requests.view' },
      },
      {
        path: 'purchase-services',
        name: 'PurchaseServices',
        component: PurchaseServices,
        meta: { title: 'Pengadaan Jasa — Cloud POS', requiresAuth: true, permission: 'procurement.requests.view' },
      },
      {
        path: 'work-units',
        name: 'WorkUnits',
        component: WorkUnits,
        meta: { title: 'Unit Kerja — Cloud POS', requiresAuth: true, permission: 'workunits.view' },
      },
      {
        path: 'projects',
        name: 'Projects',
        component: Projects,
        meta: { title: 'Projek — Cloud POS', requiresAuth: true, permission: 'procurement.projects.view' },
      },
      {
        path: 'projects/:id',
        name: 'ProjectDetail',
        component: ProjectDetail,
        meta: { title: 'Detail Projek — Cloud POS', requiresAuth: true, permission: 'procurement.projects.view' },
      },
      {
        path: 'vendors',
        name: 'Vendors',
        component: Vendors,
        meta: { title: 'Vendor — Cloud POS', requiresAuth: true, permission: 'vendors.view' },
      },
      {
        path: 'vendors/:id',
        name: 'VendorDetail',
        component: VendorDetail,
        meta: { title: 'Detail Vendor — Cloud POS', requiresAuth: true, permission: 'vendors.view' },
      },
      {
        path: 'settings/company',
        name: 'CompanyIdentity',
        component: CompanyIdentity,
        meta: { title: 'Identitas Perusahaan — Cloud POS', requiresAuth: true, permission: 'settings.company.view' },
      },
      {
        path: 'settings/timezone',
        name: 'TimezoneSettings',
        component: TimezoneSettings,
        meta: { title: 'Zona Waktu — Cloud POS', requiresAuth: true, permission: 'settings.timezone.view' },
      },
      {
        path: 'settings/tax',
        name: 'TaxSettings',
        component: TaxSettings,
        meta: { title: 'Pengaturan Pajak — Cloud POS', requiresAuth: true, permission: 'settings.tax.view' },
      },
      {
        path: 'settings/devices',
        name: 'DeviceMonitor',
        component: DeviceMonitor,
        meta: { title: 'Perangkat — Cloud POS', requiresAuth: true, permission: 'devices.view' },
      },
      // ── PPIC ────────────────────────────────────────────
      {
        path: 'ppic/dashboard',
        name: 'PpicDashboard',
        component: PpicDashboard,
        meta: { title: 'Dashboard PPIC — Cloud POS', requiresAuth: true, permission: 'ppic.dashboard.view' },
      },
      {
        path: 'ppic/planning-params',
        name: 'PlanningParams',
        component: PlanningParams,
        meta: { title: 'Par Level & ROP — Cloud POS', requiresAuth: true, permission: 'ppic.planning.view' },
      },
      {
        path: 'ppic/expiry',
        name: 'ExpiryMonitor',
        component: ExpiryMonitor,
        meta: { title: 'Monitor Kedaluwarsa — Cloud POS', requiresAuth: true, permission: 'ppic.expiry.view' },
      },
      {
        path: 'ppic/opname',
        name: 'StockOpname',
        component: StockOpname,
        meta: { title: 'Stock Opname — Cloud POS', requiresAuth: true, permission: 'ppic.opname.view' },
      },
      {
        path: 'ppic/forecast',
        name: 'DemandForecast',
        component: DemandForecast,
        meta: { title: 'Demand Forecast — Cloud POS', requiresAuth: true, permission: 'ppic.forecast.view' },
      },
      {
        path: 'ppic/mrp',
        name: 'MrpWorksheet',
        component: MrpWorksheet,
        meta: { title: 'Kebutuhan Bahan (MRP) — Cloud POS', requiresAuth: true, permission: 'ppic.mrp.view' },
      },
      {
        path: 'ppic/production-plans',
        name: 'ProductionPlans',
        component: ProductionPlans,
        meta: { title: 'Rencana Produksi — Cloud POS', requiresAuth: true, permission: 'ppic.production.view' },
      },
      {
        path: 'ppic/work-orders',
        name: 'WorkOrders',
        component: WorkOrders,
        meta: { title: 'Work Order — Cloud POS', requiresAuth: true, permission: 'ppic.workorders.view' },
      },
      {
        path: 'ppic/reports',
        name: 'PpicReports',
        component: PpicReports,
        meta: { title: 'Laporan PPIC — Cloud POS', requiresAuth: true, permission: 'ppic.reports.view' },
      },
      // ── Warehouse / Gudang ──────────────────────────────
      {
        path: 'warehouse-dashboard',
        name: 'WarehouseDashboard',
        component: WarehouseDashboard,
        meta: { title: 'Dashboard Gudang — Cloud POS', requiresAuth: true, permission: 'warehouse_dashboard.view' },
      },
      {
        path: 'warehouses',
        name: 'Warehouses',
        component: Warehouses,
        meta: { title: 'Gudang — Cloud POS', requiresAuth: true, permission: 'warehouses.view' },
      },
      {
        path: 'stock-items',
        name: 'StockItems',
        component: StockItems,
        meta: { title: 'Item Stok — Cloud POS', requiresAuth: true, permission: 'stockitems.view' },
      },
      {
        path: 'stock-transfers',
        name: 'StockTransfers',
        component: StockTransfers,
        meta: { title: 'Transfer Stok — Cloud POS', requiresAuth: true, permission: 'stocktransfers.view' },
      },
      {
        path: 'goods-receipts',
        name: 'GoodsReceipt',
        component: GoodsReceipt,
        meta: { title: 'Penerimaan Barang — Cloud POS', requiresAuth: true, permission: 'stockledger.view' },
      },
      {
        path: 'stock-wastes',
        name: 'StockWastes',
        component: StockWastes,
        meta: { title: 'Stok Rusak/Hilang — Cloud POS', requiresAuth: true, permission: 'stockwastes.view' },
      },
      {
        path: 'stock-ledger',
        name: 'StockLedger',
        component: StockLedger,
        meta: { title: 'Buku Stok — Cloud POS', requiresAuth: true, permission: 'stockledger.view' },
      },
      {
        path: 'perlengkapan',
        name: 'Assets',
        component: Assets,
        meta: { title: 'Manajemen Perlengkapan — Cloud POS', requiresAuth: true, permission: 'assets.view' },
      },
      {
        path: 'reservations',
        name: 'Reservations',
        component: Reservations,
        meta: { title: 'Reservasi — Cloud POS', requiresAuth: true, permission: 'reservations.view' },
      },
      {
        path: 'customers',
        name: 'Customers',
        component: Customers,
        meta: { title: 'Pelanggan — Cloud POS', requiresAuth: true, permission: 'customers.view' },
      },
      {
        path: '403',
        name: 'Forbidden',
        component: Forbidden,
        meta: { title: 'Akses Ditolak — Cloud POS', requiresAuth: true },
      },
      {
        // Catch-all 404 di dalam shell (sidebar tetap tampil untuk user login)
        path: ':pathMatch(.*)*',
        name: 'NotFound',
        component: NotFound,
        meta: { title: 'Tidak Ditemukan — Cloud POS', requiresAuth: true },
      },
      {
        path: 'recipes',
        name: 'Recipes',
        component: Recipes,
        meta: { title: 'Resep — Cloud POS', requiresAuth: true, permission: 'recipes.view' },
      },
    ],
  },

]

// ── Router instance ──────────────────────────────────────────
const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }), // always scroll to top on navigation
})

// ── Navigation guards ────────────────────────────────────────
router.beforeEach(async (to) => {
  // Update page title
  if (to.meta.title) {
    document.title = to.meta.title
  }

  const auth = useAuthStore()

  // Redirect to role's default page if already logged in
  if (to.name === 'Login' && auth.isAuthenticated) {
    return auth.redirectTo || '/'
  }

  // Redirect to login if route requires auth but user is not authenticated.
  // (Login always returns to the role's default page, so no redirect query needed.)
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'Login' }
  }

  // Always refresh permissions from API on first navigation after page load
  if (auth.isAuthenticated && !auth.permsSynced) {
    await auth.fetchPermissions()
  }

  // Superadmin-only routes
  if (to.meta.superadmin && auth.isAuthenticated && !auth.isSuperadmin) {
    return { name: 'Forbidden' }
  }

  // Check permission-based access
  if (to.meta.permission && auth.isAuthenticated && !auth.hasPermission(to.meta.permission)) {
    // Halaman utama '/': jangan tampilkan Forbidden — arahkan ke halaman default
    // role tsb (mis. teknisi → /outlets). Terjadi saat klik logo / buka URL root.
    if (to.path === '/' && auth.redirectTo && auth.redirectTo !== '/') {
      return { path: auth.redirectTo }
    }
    return { name: 'Forbidden' }
  }
})

export default router
