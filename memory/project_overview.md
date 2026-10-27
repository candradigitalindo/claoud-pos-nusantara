---
name: project-overview
description: Ringkasan arsitektur dan modul utama aplikasi Nusantara POS Cloud API
metadata:
  type: project
---

# Nusantara POS — Cloud API

Aplikasi backend cloud server untuk sinkronisasi data multi-outlet POS (Point of Sale).

**Nama proyek:** `cloud-pos` (Go module: `cloud-pos`)  
**Stack backend:** Go 1.24 + Fiber v2 + PostgreSQL + JWT  
**Stack frontend:** Vue 3 + Vite + Tailwind CSS v4 + Pinia + Vue Router + Chart.js  

---

## Arsitektur

```
POS Outlet (pos-app.exe)
    │
    ├─ Push: Orders, Transactions, Products ──→ Cloud API ──→ PostgreSQL
    └─ Pull: GET /updates?since= ←── Cloud API ←── PostgreSQL
```

Backend berjalan di port default **3000**. Frontend (Vue 3) di-serve langsung oleh Fiber dari folder `ui/dist/` (production) atau via Vite dev server (`DEV_UI=true`).

---

## Struktur Direktori

- `main.go` — entry point, setup Fiber, CORS, logger, auto-start Vite dev server
- `config/` — load `.env` (PORT, DB, JWT_SECRET, ADMIN_TOKEN, dll.)
- `database/` — koneksi PostgreSQL, RunMigrations (inline SQL), SeedAdmin
- `routes/routes.go` — semua routing API
- `middleware/auth.go` — AuthOutlet (API key), AdminAuth (JWT/static token), RequirePermission
- `handlers/` — HTTP handler per domain
- `services/` — business logic per domain
- `models/` — struct model
- `ui/src/` — Vue 3 admin dashboard

---

## Domain / Modul

| Modul | Keterangan |
|-------|-----------|
| Outlets | Manajemen outlet POS, regenerate API key, toggle aktif |
| Orders | Sync order dari outlet |
| Transactions | Sync transaksi (pembayaran) dari outlet |
| Products & Categories | Sync produk, kategori, link ke stock item/recipe |
| Analytics | Push daily analytics dari outlet |
| Sync (Batch, Conflicts, Logs) | Sinkronisasi batch, resolve conflict, audit log |
| Dashboard | Stats admin & manager dashboard |
| Admins & Roles | User management, RBAC granular, role scope (all/specific work unit) |
| Procurement | Purchase request (barang/jasa) dengan workflow: submit→approve→purchasing→finance |
| Work Units | Unit kerja yang bisa di-link ke outlet atau berdiri sendiri |
| Vendors | Master data vendor/supplier |
| Bank Accounts | Rekening bank untuk pembayaran procurement |
| Reports | Sales, product sales, unpaid orders, tax, cash flow, balance, profit-loss, general ledger |
| Warehouse | Hanya mengelola bahan baku dan persediaan: katalog, satuan, saldo, penerimaan, mutasi, transfer, waste, dan opname. Tidak membuat atau mengubah resep. Pencatatan pemakaian/hasil pengolahan mengikuti formula F & B |
| F & B Recipes | Pembuatan/perubahan seluruh resep menu/SOP, resep bahan setengah jadi, dan pola pemotongan di `/fnb/recipes`; seluruh pengaturan resep menjadi tugas F & B |
| Settings | Company identity, timezone, tax (PB1) |
| Upload | File upload (foto bukti pembayaran, logo, dll.) |

---

## Autentikasi

- **Outlet API:** Bearer `{api_key}` — API key per outlet di-generate saat create outlet
- **Admin API:** Bearer `{token}` — static ADMIN_TOKEN (superadmin) atau JWT (login via `/api/v1/admin/login`)
- **Permissions:** RBAC granular di tabel `role_permissions`. Format: `module.view`, `module.manage`, `procurement.submit`, dll.
- **Role scope:** Role bisa di-scope ke work unit tertentu (`scope_type = 'specific'`)

---

## Database (tabel utama)

`outlets`, `cloud_orders`, `cloud_transactions`, `cloud_products`, `cloud_categories`, `cloud_analytics`, `sync_logs`, `sync_conflicts`, `cloud_cashier_shifts`, `cloud_cash_movements`, `cloud_admins`, `cloud_printers`, `purchase_requests`, `payment_histories`, `work_units`, `vendors`, `bank_accounts`, `stock_items`, `stock_item_categories`, `warehouses`, `stock_ledger`, `stock_batches`, `stock_movements`, `stock_transfers`, `stock_transfer_items`, `stock_wastes`, `product_recipes`, `recipe_masters`, `recipe_items`, `recipe_outlet_access`, `stock_item_recipes`, `roles`, `role_permissions`, `role_work_unit_scope`, `app_settings`

---

## Frontend Pages (ui/src/pages/)

Dashboard, Login, Outlets, OutletDetail, Products, Admins, Roles, WorkUnits, Vendors, BankAccounts, ProcurementDashboard, ProcurementPayments, PurchaseGoods, PurchaseServices, SalesReport, ProductSalesReport, TaxReport, CashFlowReport, BalanceReport, ProfitLossReport, GeneralLedger, UnpaidOrders, Warehouses, StockItems, StockLedger, StockTransfers, StockWastes, Recipes, CompanyIdentity, TaxSettings, TimezoneSettings, ManagerDashboard

Sub-pages outlet: Categories, Conflicts, Orders, OutletInfo, Printers, Products, SyncLogs, Transactions

**Why:** Cloud sync server untuk jaringan multi-outlet POS Nusantara.  
**How to apply:** Gunakan konteks ini saat mengerjakan fitur, bug, atau refactor di proyek ini.
