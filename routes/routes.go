package routes

import (
	"cloud-pos/config"
	"cloud-pos/handlers"
	"cloud-pos/middleware"

	"github.com/gofiber/fiber/v2"
)

func Setup(app *fiber.App, cfg *config.Config) {
	// App distribution files: force the correct MIME + download disposition so an
	// APK saves as ".apk" (not ".zip"). Registered before the static handler so it
	// takes precedence for /uploads/apps/<name>.
	app.Get("/uploads/apps/:name", handlers.DownloadAppFile)

	// Serve uploaded files
	app.Static("/uploads", "./uploads")

	api := app.Group("/api/v1")

	// Public
	api.Get("/ping", handlers.Ping)
	api.Get("/timezone", handlers.GetTimezone) // public — used by frontend for time formatting

	// Public reservation (no auth) — per-outlet by slug, dibatasi per IP
	public := api.Group("/public", middleware.PublicRateLimiter())
	public.Get("/outlets/:slug/menu", handlers.PublicGetMenu)
	public.Post("/outlets/:slug/reservations", handlers.PublicCreateReservation)
	// Cek status + unggah bukti DP oleh pelanggan (id reservasi = ULID, tidak bisa ditebak).
	public.Get("/outlets/:slug/reservations/:id", handlers.PublicGetReservation)
	public.Post("/outlets/:slug/reservations/:id/payments", handlers.PublicSubmitReservationPayment)

	// Outlet self-discovery: GET /api/v1/outlet/me — returns outlet info from API key alone
	api.Get("/outlet/me", middleware.AuthOutlet(), func(c *fiber.Ctx) error {
		return handlers.GetOutletInfo(c)
	})

	// Outlet API (authenticated by API key)
	outlet := api.Group("/outlets/:outletId", middleware.AuthOutlet(), middleware.RateLimiter(cfg))

	// Outlet info (self-service)
	outlet.Get("/info", handlers.GetOutletInfo)

	// Reservasi untuk app POS: daftar hari ini + menutup reservasi dari transaksi.
	// Transaksi juga bisa membawa reservation_id di payload (SaveTransaction).
	outlet.Get("/reservations", handlers.OutletListReservations)
	outlet.Post("/reservations/:rid/settle", handlers.OutletSettleReservation)

	// Orders
	outlet.Post("/orders", handlers.PushOrder)
	outlet.Get("/orders", handlers.GetOrders)

	// Transactions
	outlet.Post("/transactions", handlers.PushTransaction)
	outlet.Get("/transactions", handlers.GetTransactions)

	// Products
	outlet.Post("/products", handlers.PushProduct)
	outlet.Get("/products", handlers.GetProducts)

	// Categories
	outlet.Get("/categories", handlers.GetOutletCategories)
	outlet.Put("/categories/:categoryId/printer", handlers.UpdateCategoryPrinter)

	// Printers
	outlet.Get("/printers", handlers.GetOutletPrinters)
	outlet.Post("/printers", handlers.PushPrinter)

	// Analytics
	outlet.Post("/analytics/daily", handlers.PushAnalytics)
	outlet.Get("/analytics", handlers.GetAnalytics)

	// Batch sync
	outlet.Post("/sync/batch", handlers.BatchSync)

	// Updates
	outlet.Get("/updates", handlers.GetUpdates)

	// Restore — ambil state tertinggal setelah device reset database
	outlet.Get("/restore", handlers.GetRestore)

	// Heartbeat perangkat — telemetri tablet kasir (baterai, storage, printer, jaringan)
	outlet.Post("/heartbeat", handlers.PushDeviceHeartbeat)

	// Conflicts
	outlet.Get("/conflicts", handlers.GetConflicts)
	outlet.Post("/conflicts/:conflictId/resolve", handlers.ResolveConflict)

	// Sync logs
	outlet.Get("/sync/logs", handlers.GetSyncLogs)

	// Admin login (public — no auth required)
	api.Post("/admin/login", middleware.LoginRateLimiter(), handlers.AdminLogin(cfg))

	// SSE realtime — auth via ?token= (EventSource tidak bisa kirim header),
	// jadi didaftarkan di luar grup AdminAuth; validasi token di handler.
	api.Get("/admin/events", handlers.Events(cfg))

	// CCTV auth subrequest — dipanggil nginx auth_request untuk melindungi relay
	// /cctv/. Divalidasi via cookie stream token (request media tidak bawa header).
	api.Get("/cctv/authz", handlers.CCTVAuthz(cfg))

	// Admin API (authenticated by admin token or JWT)
	admin := api.Group("/admin", middleware.AdminAuth(cfg))

	// Work Units (Move to top to avoid 404/conflicts)
	admin.Get("/work-units/me", middleware.RequirePermission("procurement.requests.view"), handlers.GetMyWorkUnit)
	admin.Get("/my-work-units", middleware.RequirePermission("procurement.requests.view"), handlers.GetMyWorkUnits)
	admin.Get("/work-units", middleware.RequirePermission("workunits.view"), handlers.ListWorkUnits)
	admin.Get("/work-units/:id", middleware.RequirePermission("workunits.view"), handlers.GetWorkUnit)
	admin.Post("/work-units", middleware.RequirePermission("workunits.create"), handlers.CreateWorkUnit)
	admin.Put("/work-units/:id", middleware.RequirePermission("workunits.update"), handlers.UpdateWorkUnit)

	// App POS distributables (APK upload + download URL) — Master Data
	admin.Get("/app-files", middleware.RequirePermission("appfiles.view"), handlers.ListAppFiles)
	admin.Post("/app-files", middleware.RequirePermission("appfiles.create"), handlers.UploadAppFile)
	admin.Delete("/app-files/:name", middleware.RequirePermission("appfiles.delete"), handlers.DeleteAppFile)
	admin.Delete("/work-units/:id", middleware.RequirePermission("workunits.delete"), handlers.DeleteWorkUnit)

	// Dashboard
	admin.Get("/dashboard", middleware.RequireAnyPermission("dashboard", "dashboard.manager"), handlers.GetDashboard)
	admin.Get("/manager-dashboard", middleware.RequireAnyPermission("dashboard", "dashboard.manager"), handlers.GetManagerDashboard)

	// Me / permissions (all roles)
	admin.Get("/me/permissions", handlers.GetMyPermissions)
	admin.Put("/me/password", handlers.ChangePassword)
	admin.Put("/me/profile", handlers.UpdateProfile)

	// Access logs — login/access history (role permission; superadmin always passes)
	admin.Get("/access-logs", middleware.RequirePermission("access_logs.view"), handlers.GetAccessLogs)

	// Outlets — CRUD granular
	admin.Get("/my-outlets", handlers.GetMyOutlets)
	admin.Get("/outlets", middleware.RequirePermission("outlets.view"), handlers.GetOutlets)
	admin.Get("/outlets/:id", middleware.RequirePermission("outlets.view"), handlers.GetOutlet)
	admin.Post("/outlets", middleware.RequirePermission("outlets.create"), handlers.CreateOutlet)
	admin.Put("/outlets/:id", middleware.RequirePermission("outlets.update"), handlers.UpdateOutlet)
	admin.Post("/outlets/:id/regenerate-key", middleware.RequirePermission("outlets.update"), handlers.RegenerateOutletAPIKey)
	admin.Post("/outlets/:id/toggle", middleware.RequirePermission("outlets.update"), handlers.ToggleOutlet)
	admin.Delete("/outlets/:id", middleware.RequirePermission("outlets.delete"), handlers.DeleteOutlet)

	// Pajak per-outlet (Pengaturan > Pajak)
	admin.Get("/outlet-taxes", middleware.RequirePermission("settings.tax.view"), handlers.ListOutletTax)
	admin.Get("/outlets/:id/tax", middleware.RequirePermission("settings.tax.view"), handlers.GetOutletTax)
	admin.Put("/outlets/:id/tax", middleware.RequirePermission("settings.tax.update"), handlers.UpdateOutletTax)

	// Outlet procurement dashboard
	admin.Get("/outlets/:id/procurement-dashboard", middleware.RequirePermission("procurement.dashboard.view"), handlers.GetProcurementDashboard)

	// Reports (view-only by nature)
	admin.Get("/sales-report", middleware.RequirePermission("reports.sales.view"), handlers.GetSalesReport)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/sales-report/export", middleware.RequirePermission("reports.sales.view"), handlers.ExportSalesReport)
	admin.Get("/unpaid-orders", middleware.RequirePermission("reports.sales.view"), handlers.GetUnpaidOrders)
	admin.Get("/cashier-shifts", middleware.RequirePermission("cashier_shifts.view"), handlers.GetCashierShiftReport)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/cashier-shifts/export", middleware.RequirePermission("cashier_shifts.view"), handlers.ExportCashierShiftReport)
	// Rekonsiliasi shift (tutup kasir vs cloud). Lihat: shift_reconciliation.view (dipisah dari cashier_shifts.view).
	// Penyesuaian data (ikuti versi kasir) = khusus superadmin, tercatat & bisa dibatalkan.
	admin.Get("/shift-reconciliation", middleware.RequirePermission("shift_reconciliation.view"), handlers.GetShiftReconciliation)
	admin.Post("/shift-reconciliation/:shiftId/apply", middleware.RequireSuperadmin(), handlers.ApplyShiftAdjustment)
	admin.Post("/shift-reconciliation/adjustments/:id/revert", middleware.RequireSuperadmin(), handlers.RevertShiftAdjustment)
	admin.Get("/devices", middleware.RequirePermission("devices.view"), handlers.GetDeviceMonitor)
	admin.Get("/devices/:outletId/history", middleware.RequirePermission("devices.view"), handlers.GetDeviceHistory)

	// CCTV — monitoring live/putar ulang (view) + kelola kamera (create/update/delete).
	// Relay-only: cloud tidak menyimpan video; hanya konfigurasi + relay go2rtc.
	admin.Get("/cameras", middleware.RequirePermission("cameras.view"), handlers.GetCameras)
	admin.Get("/outlets/:id/cameras", middleware.RequirePermission("cameras.view"), handlers.GetOutletCameras)
	admin.Post("/outlets/:id/cameras", middleware.RequirePermission("cameras.create"), handlers.CreateCamera)
	admin.Put("/cameras/:camId", middleware.RequirePermission("cameras.update"), handlers.UpdateCamera)
	admin.Post("/cameras/:camId/toggle", middleware.RequirePermission("cameras.update"), handlers.ToggleCamera)
	admin.Delete("/cameras/:camId", middleware.RequirePermission("cameras.delete"), handlers.DeleteCamera)
	admin.Post("/cameras/:camId/stream", middleware.RequirePermission("cameras.view"), handlers.StartCameraStream(cfg))
	admin.Post("/cameras/:camId/playback", middleware.RequirePermission("cameras.view"), handlers.StartCameraPlayback(cfg))
	admin.Post("/cameras/:camId/stop", middleware.RequirePermission("cameras.view"), handlers.StopCameraStream)
	admin.Get("/product-sales-report", middleware.RequirePermission("reports.product_sales.view"), handlers.GetProductSalesReport)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/product-sales-report/export", middleware.RequirePermission("reports.product_sales.view"), handlers.ExportProductSalesReport)
	admin.Get("/tax-report", middleware.RequirePermission("reports.tax.view"), handlers.GetTaxReport)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/tax-report/export", middleware.RequirePermission("reports.tax.view"), handlers.ExportTaxReport)
	admin.Get("/cash-flow-report", middleware.RequirePermission("reports.cashflow.view"), handlers.GetCashFlowReport)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/cash-flow-report/export", middleware.RequirePermission("reports.cashflow.view"), handlers.ExportCashFlowReport)
	admin.Get("/balance-report", middleware.RequirePermission("reports.balance.view"), handlers.GetBalanceReport)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/balance-report/export", middleware.RequirePermission("reports.balance.view"), handlers.ExportBalanceReport)
	admin.Get("/profit-loss-report", middleware.RequirePermission("reports.pnl.view"), handlers.GetProfitLossReport)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/profit-loss-report/export", middleware.RequirePermission("reports.pnl.view"), handlers.ExportProfitLossReport)
	admin.Get("/general-ledger", middleware.RequirePermission("reports.ledger.view"), handlers.GetGeneralLedger)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/general-ledger/export", middleware.RequirePermission("reports.ledger.view"), handlers.ExportGeneralLedger)
	admin.Get("/void-report", middleware.RequirePermission("reports.void.view"), handlers.GetVoidReport)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/void-report/export", middleware.RequirePermission("reports.void.view"), handlers.ExportVoidReport)
	admin.Get("/titipan-report", middleware.RequirePermission("reports.titipan.view"), handlers.GetTitipanReport)
	admin.Get("/discount-report", middleware.RequirePermission("reports.discount.view"), handlers.GetDiscountReport)
	// Export Excel memakai permission & scope outlet yang sama dengan laporan di layar.
	admin.Get("/discount-report/export", middleware.RequirePermission("reports.discount.view"), handlers.ExportDiscountReport)
	// Analisa Bisnis — RGI/anti-alibi lintas outlet (Laporan → Analisa Bisnis)
	admin.Get("/business-analysis", middleware.RequirePermission("reports.business_analysis.view"), handlers.GetBusinessAnalysis)
	admin.Get("/business-analysis/export", middleware.RequirePermission("reports.business_analysis.view"), handlers.ExportBusinessAnalysisExcel)

	// Kinerja Markom — pemasok angka IG/TikTok untuk Analisa Bisnis.
	// Melihat dipisah dari mengubah: mendaftarkan akun, mengetik tambalan
	// mingguan, dan menarik paksa dari halaman publik adalah pekerjaan Markom.
	admin.Get("/social-accounts", middleware.RequirePermission("social.view"), handlers.ListSocialAccounts)
	admin.Get("/social-weekly", middleware.RequirePermission("social.view"), handlers.GetSocialWeekly)
	admin.Get("/social-accounts/:id/manual", middleware.RequirePermission("social.view"), handlers.ListSocialManualWeeks)
	admin.Post("/social-accounts", middleware.RequirePermission("social.manage"), handlers.CreateSocialAccount)
	admin.Post("/social-accounts/scrape", middleware.RequirePermission("social.manage"), handlers.ScrapeSocialAccounts)
	admin.Put("/social-accounts/:id", middleware.RequirePermission("social.manage"), handlers.UpdateSocialAccount)
	admin.Put("/social-accounts/:id/manual", middleware.RequirePermission("social.manage"), handlers.UpsertSocialManualWeek)
	admin.Delete("/social-accounts/:id", middleware.RequirePermission("social.manage"), handlers.DeleteSocialAccount)

	// Products & Categories — CRUD granular
	admin.Get("/products", middleware.RequirePermission("products.view"), handlers.AdminGetProducts)
	admin.Get("/categories", middleware.RequirePermission("products.view"), handlers.AdminGetCategories)
	admin.Post("/products", middleware.RequirePermission("products.create"), handlers.AdminCreateProduct)
	admin.Put("/products/:id", middleware.RequirePermission("products.update"), handlers.AdminUpdateProduct)
	admin.Delete("/products/:id", middleware.RequirePermission("products.delete"), handlers.AdminDeleteProduct)
	admin.Post("/categories", middleware.RequirePermission("products.create"), handlers.AdminCreateCategory)
	admin.Put("/categories/:id", middleware.RequirePermission("products.update"), handlers.AdminUpdateCategory)
	admin.Delete("/categories/:id", middleware.RequirePermission("products.delete"), handlers.AdminDeleteCategory)

	// User management — CRUD granular
	admin.Get("/admins", middleware.RequirePermission("users.view"), handlers.GetAdmins)
	admin.Get("/permissions", middleware.RequirePermission("roles.view"), handlers.GetAllRolePermissions)
	admin.Get("/roles", middleware.RequirePermission("roles.view"), handlers.ListRoles)
	admin.Post("/admins", middleware.RequirePermission("users.create"), handlers.CreateAdmin)
	admin.Put("/admins/:id/role", middleware.RequirePermission("users.update"), handlers.UpdateAdminRole)
	admin.Put("/admins/:id/reset-password", middleware.RequirePermission("users.update"), handlers.ResetAdminPassword)
	admin.Post("/admins/:id/toggle", middleware.RequirePermission("users.update"), handlers.ToggleAdminActive)
	admin.Put("/admins/:id", middleware.RequirePermission("users.update"), handlers.UpdateAdmin)
	admin.Delete("/admins/:id", middleware.RequirePermission("users.delete"), handlers.DeleteAdmin)
	admin.Put("/permissions/:role", middleware.RequirePermission("roles.update"), handlers.UpdateRolePermissions)
	admin.Post("/roles", middleware.RequirePermission("roles.create"), handlers.CreateRole)
	admin.Put("/roles/:name", middleware.RequirePermission("roles.update"), handlers.UpdateRole)
	admin.Delete("/roles/:name", middleware.RequirePermission("roles.delete"), handlers.DeleteRole)
	admin.Put("/roles/:role/scope", middleware.RequirePermission("roles.update"), handlers.UpdateRoleScope)

	// Purchase requests — granular: submit, approve, purchasing (isi harga), finance (bayar)
	admin.Get("/procurement-dashboard", middleware.RequirePermission("procurement.dashboard.view"), handlers.GetProcurementDashboardGlobal)
	admin.Get("/payment-stats", middleware.RequirePermission("finance.payments.view"), handlers.GetPaymentStats)
	// Export Excel memakai permission & scope yang sama dengan halaman Pembayaran.
	admin.Get("/procurement-payments/export", middleware.RequirePermission("finance.payments.view"), handlers.ExportProcurementPayments)
	// Projek (pembangunan/renovasi) — payung RAB di atas pengadaan.
	// Lihat dibuka juga untuk pemegang izin pengadaan: form pengajuan perlu
	// memuat daftar projek untuk dipilih. Ubah/hapus tetap terkunci di manage.
	admin.Get("/projects", middleware.RequireAnyPermission("procurement.projects.view", "procurement.requests.submit", "procurement.requests.view"), handlers.ListProjects)
	admin.Get("/projects/:id", middleware.RequirePermission("procurement.projects.view"), handlers.GetProject)
	admin.Post("/projects", middleware.RequirePermission("procurement.projects.manage"), handlers.CreateProject)
	admin.Put("/projects/:id", middleware.RequirePermission("procurement.projects.manage"), handlers.UpdateProject)
	admin.Delete("/projects/:id", middleware.RequirePermission("procurement.projects.manage"), handlers.DeleteProject)
	// RAB per baris: dilihat oleh siapa pun yang boleh mengajukan/melihat
	// pengadaan (form pengajuan memilih baris RAB), diubah oleh pengelola projek.
	admin.Get("/projects/:id/rab", middleware.RequireAnyPermission("procurement.projects.view", "procurement.requests.submit", "procurement.requests.view"), handlers.GetProjectRab)
	admin.Put("/projects/:id/rab", middleware.RequirePermission("procurement.projects.manage"), handlers.SaveProjectRab)
	admin.Post("/projects/:id/rab/set", middleware.RequirePermission("procurement.projects.manage"), handlers.SetProjectRab)
	admin.Post("/projects/:id/rab/reopen", middleware.RequirePermission("procurement.projects.manage"), handlers.ReopenProjectRab)

	// Material projek — barang habis pakai milik projek (docs §8.8–8.9).
	// Izinnya memakai kunci Projek yang sudah ada (keputusan B, docs §15.1).
	admin.Get("/projects/:id/materials", middleware.RequirePermission("procurement.projects.view"), handlers.ListProjectMaterials)
	admin.Get("/projects/:id/materials/:mid/logs", middleware.RequirePermission("procurement.projects.view"), handlers.ListProjectMaterialLogs)
	admin.Post("/projects/:id/materials/:mid/usage", middleware.RequirePermission("procurement.projects.manage"), handlers.RecordProjectMaterialUsage)
	admin.Post("/projects/:id/materials/:mid/settle", middleware.RequirePermission("procurement.projects.manage"), handlers.SettleProjectMaterial)
	admin.Get("/project-materials/unsettled", middleware.RequirePermission("procurement.projects.view"), handlers.ListUnsettledProjectMaterials)

	admin.Get("/purchase-requests", middleware.RequirePermission("procurement.requests.view"), handlers.ListPurchaseRequests)
	admin.Get("/purchase-requests/:id", middleware.RequirePermission("procurement.requests.view"), handlers.GetPurchaseRequest)
	admin.Get("/purchase-requests/:id/payment-histories", middleware.RequirePermission("finance.payments.view"), handlers.GetPaymentHistories)
	admin.Post("/purchase-requests/:id/split", middleware.RequirePermission("procurement.requests.purchasing"), handlers.SplitPurchaseRequest)
	// Serah Terima pengadaan → aset / stok gudang (docs/perlengkapan-aset.md §8)
	admin.Get("/purchase-requests/:id/receiving-draft", middleware.RequirePermission("procurement.requests.submit"), handlers.GetReceivingDraft)
	admin.Post("/purchase-requests/:id/receive-goods", middleware.RequirePermission("procurement.requests.submit"), handlers.ReceiveGoods)
	admin.Post("/purchase-requests", middleware.RequirePermission("procurement.requests.submit"), handlers.CreatePurchaseRequest)
	admin.Put("/purchase-requests/:id/status", middleware.RequirePermission("procurement.requests.view"), handlers.UpdatePurchaseStatus)
	admin.Put("/purchase-requests/:id/items", middleware.RequirePermission("procurement.requests.view"), handlers.UpdatePurchaseItems)
	admin.Delete("/purchase-requests/:id", middleware.RequirePermission("procurement.requests.submit"), handlers.DeletePurchaseRequest)

	// File upload — dipakai untuk bukti pembayaran pengadaan
	admin.Post("/upload", middleware.RequireAnyPermission("finance.payments.view", "procurement.requests.view", "assets.create", "assets.update", "reservations.update"), handlers.UploadFile)

	// Bank Accounts
	admin.Get("/bank-accounts", middleware.RequirePermission("finance.bank.view"), handlers.ListBankAccounts)
	admin.Post("/bank-accounts", middleware.RequirePermission("finance.bank.create"), handlers.CreateBankAccount)
	admin.Put("/bank-accounts/:id", middleware.RequirePermission("finance.bank.update"), handlers.UpdateBankAccount)
	admin.Delete("/bank-accounts/:id", middleware.RequirePermission("finance.bank.delete"), handlers.DeleteBankAccount)

	// Vendors
	// Vendor list/lookup is also needed by procurement requesters (to pick a vendor).
	admin.Get("/vendors", middleware.RequireAnyPermission("vendors.view", "procurement.requests.view", "procurement.requests.submit", "procurement.requests.purchasing"), handlers.ListVendors)
	admin.Get("/vendors/:id", middleware.RequireAnyPermission("vendors.view", "procurement.requests.view", "procurement.requests.submit", "procurement.requests.purchasing"), handlers.GetVendor)
	admin.Get("/vendors/:id/detail", middleware.RequirePermission("vendors.view"), handlers.GetVendorDetail)
	admin.Get("/vendors/:id/purchases", middleware.RequirePermission("vendors.view"), handlers.ListVendorPurchases)
	// Manajemen Aset + histori perawatan (scoped per outlet)
	// Impor aset dari Excel. Template dan pemeriksaan berkas dipisah dari
	// penyimpanan, supaya berkas ratusan baris tidak langsung masuk.
	admin.Get("/asset-import/template", middleware.RequirePermission("assets.create"), handlers.DownloadAssetImportTemplate)
	admin.Post("/asset-import", middleware.RequirePermission("assets.create"), handlers.ImportAssets)

	// Master kategori aset — dipakai form aset, dialog penerimaan, dan laporan.
	admin.Get("/asset-categories", middleware.RequirePermission("assets.view"), handlers.ListAssetCategories)
	admin.Post("/asset-categories", middleware.RequirePermission("assets.update"), handlers.CreateAssetCategory)
	admin.Put("/asset-categories/:id", middleware.RequirePermission("assets.update"), handlers.UpdateAssetCategory)
	admin.Delete("/asset-categories/:id", middleware.RequirePermission("assets.delete"), handlers.DeleteAssetCategory)

	admin.Get("/assets", middleware.RequirePermission("assets.view"), handlers.ListAssets)
	admin.Get("/assets/:id", middleware.RequirePermission("assets.view"), handlers.GetAsset)
	admin.Post("/assets", middleware.RequirePermission("assets.create"), handlers.CreateAsset)
	admin.Put("/assets/:id", middleware.RequirePermission("assets.update"), handlers.UpdateAsset)
	admin.Delete("/assets/:id", middleware.RequirePermission("assets.delete"), handlers.DeleteAsset)
	admin.Get("/assets/:id/movements", middleware.RequirePermission("assets.view"), handlers.ListAssetMovements)
	// Perawatan dipisah dari assets.update supaya teknisi bisa mencatat pekerjaan
	// tanpa ikut berhak mengubah harga perolehan dan outlet aset.
	admin.Get("/assets/:id/maintenances", middleware.RequirePermission("assets.maintenance.view"), handlers.ListAssetMaintenances)
	admin.Post("/assets/:id/maintenances", middleware.RequirePermission("assets.maintenance.create"), handlers.AddAssetMaintenance)
	admin.Delete("/assets/:id/maintenances/:mid", middleware.RequirePermission("assets.maintenance.create"), handlers.DeleteAssetMaintenance)

	// Work order perawatan lintas aset. "summary" didaftarkan sebelum ":id".
	// Antrean serah terima. Dua meja: "perlengkapan" (bagian Aset) dan "dapur"
	// (Gudang Induk); izinnya sengaja longgar — yang membatasi adalah kind.
	admin.Get("/receiving-queue", middleware.RequireAnyPermission("assets.view", "stockledger.adjust", "procurement.requests.view"), handlers.ListReceivingQueue)
	// Distribusi aset ke PIC pengaju — lanjutan setelah bagian aset menerima.
	admin.Get("/asset-handovers", middleware.RequirePermission("assets.view"), handlers.ListAssetHandovers)
	admin.Get("/asset-handovers/awaiting", middleware.RequirePermission("assets.view"), handlers.ListAssetsAwaitingHandover)
	admin.Get("/asset-handovers/:id", middleware.RequirePermission("assets.view"), handlers.GetAssetHandover)
	admin.Post("/asset-handovers", middleware.RequirePermission("assets.update"), handlers.CreateAssetHandover)

	// Cadangan bukti foto ke email.
	admin.Get("/photo-backup/settings", middleware.RequirePermission("settings.company.view"), handlers.GetPhotoBackupSettings)
	admin.Put("/photo-backup/settings", middleware.RequirePermission("settings.company.update"), handlers.UpdatePhotoBackupSettings)
	admin.Get("/photo-backup/status", middleware.RequirePermission("settings.company.view"), handlers.GetPhotoBackupStatus)
	admin.Post("/photo-backup/retry", middleware.RequirePermission("settings.company.update"), handlers.RetryPhotoBackup)

	admin.Get("/asset-incomplete-receipts", middleware.RequirePermission("assets.view"), handlers.ListIncompleteReceipts)

	// Laporan aset. Rute statis didaftarkan sebelum "/assets/:id" di atas —
	// path terpisah dipakai supaya tidak pernah bentrok dengan id aset.
	admin.Get("/asset-dashboard", middleware.RequirePermission("assets.dashboard.view"), handlers.GetAssetDashboard)
	admin.Get("/asset-summary", middleware.RequirePermission("assets.view"), handlers.GetAssetSummary)
	admin.Get("/asset-reports/:type", middleware.RequirePermission("assets.report.view"), handlers.GetAssetReport)
	admin.Get("/asset-reports/:type/export", middleware.RequirePermission("assets.report.view"), handlers.ExportAssetReport)

	// Penghapusan aset — soft delete biasa hanya untuk salah input (§5.4).
	admin.Get("/asset-disposals", middleware.RequirePermission("assets.disposal.view"), handlers.ListAssetDisposals)
	admin.Post("/asset-disposals", middleware.RequirePermission("assets.disposal.create"), handlers.CreateAssetDisposal)
	admin.Post("/asset-disposals/:id/approve", middleware.RequirePermission("assets.disposal.approve"), handlers.ApproveAssetDisposal)

	// Opname aset — audit fisik per outlet (§4.5).
	admin.Get("/asset-opnames", middleware.RequirePermission("assets.opname.view"), handlers.ListAssetOpnames)
	admin.Get("/asset-opnames/:id", middleware.RequirePermission("assets.opname.view"), handlers.GetAssetOpname)
	admin.Post("/asset-opnames", middleware.RequirePermission("assets.opname.create"), handlers.CreateAssetOpname)
	admin.Put("/asset-opnames/:id/items", middleware.RequirePermission("assets.opname.create"), handlers.SaveAssetOpnameCount)
	admin.Post("/asset-opnames/:id/approve", middleware.RequirePermission("assets.opname.approve"), handlers.ApproveAssetOpname)
	admin.Get("/asset-maintenances", middleware.RequirePermission("assets.maintenance.view"), handlers.ListMaintenances)
	admin.Get("/asset-maintenances/summary", middleware.RequirePermission("assets.maintenance.view"), handlers.GetMaintenanceSummary)
	admin.Get("/asset-maintenances/:id", middleware.RequirePermission("assets.maintenance.view"), handlers.GetMaintenance)
	admin.Post("/asset-maintenances/:id/start", middleware.RequirePermission("assets.maintenance.create"), handlers.StartMaintenance)
	admin.Post("/asset-maintenances/:id/complete", middleware.RequirePermission("assets.maintenance.create"), handlers.CompleteMaintenance)
	admin.Post("/asset-maintenances/:id/cancel", middleware.RequirePermission("assets.maintenance.create"), handlers.CancelMaintenance)
	admin.Post("/asset-maintenances/:id/purchase-request", middleware.RequireAnyPermission("assets.maintenance.create"), handlers.CreateMaintenancePurchaseRequest)

	// Mutasi aset antar outlet. Path terpisah dari /assets supaya rute statis
	// seperti "available" tidak pernah tertangkap sebagai "/assets/:id".
	// "available" HARUS didaftarkan sebelum ":id" — Fiber mencocokkan berurutan.
	admin.Get("/asset-transfers", middleware.RequirePermission("assets.transfer.view"), handlers.ListAssetTransfers)
	admin.Get("/asset-transfers/available", middleware.RequirePermission("assets.transfer.create"), handlers.ListTransferableAssets)
	admin.Get("/asset-transfers/:id", middleware.RequirePermission("assets.transfer.view"), handlers.GetAssetTransfer)
	admin.Post("/asset-transfers", middleware.RequirePermission("assets.transfer.create"), handlers.CreateAssetTransfer)
	admin.Put("/asset-transfers/:id", middleware.RequirePermission("assets.transfer.create"), handlers.UpdateAssetTransfer)
	admin.Delete("/asset-transfers/:id", middleware.RequirePermission("assets.transfer.create"), handlers.DeleteAssetTransfer)
	admin.Post("/asset-transfers/:id/submit", middleware.RequirePermission("assets.transfer.create"), handlers.SubmitAssetTransfer)
	admin.Post("/asset-transfers/:id/approve", middleware.RequirePermission("assets.transfer.approve"), handlers.ApproveAssetTransfer)
	admin.Post("/asset-transfers/:id/reject", middleware.RequirePermission("assets.transfer.approve"), handlers.RejectAssetTransfer)
	admin.Post("/asset-transfers/:id/send", middleware.RequirePermission("assets.transfer.create"), handlers.SendAssetTransfer)
	admin.Post("/asset-transfers/:id/receive", middleware.RequirePermission("assets.transfer.receive"), handlers.ReceiveAssetTransfer)
	admin.Post("/asset-transfers/:id/cancel", middleware.RequirePermission("assets.transfer.create"), handlers.CancelAssetTransfer)

	// Pelanggan (Penjualan) — master otomatis dari order kasir, scoped per outlet
	admin.Get("/customers", middleware.RequirePermission("customers.view"), handlers.ListCustomers)
	admin.Get("/customers/:id", middleware.RequirePermission("customers.view"), handlers.GetCustomer)

	// Reservasi (Penjualan), scoped per outlet. Rute statis /settings didaftarkan
	// sebelum /:id supaya tidak tertangkap sebagai id.
	admin.Get("/reservations/settings", middleware.RequirePermission("reservations.view"), handlers.GetReservationSettings)
	admin.Put("/reservations/settings", middleware.RequirePermission("reservations.update"), handlers.UpdateReservationSettings)
	admin.Get("/reservations", middleware.RequirePermission("reservations.view"), handlers.ListReservations)
	admin.Get("/reservations/:id", middleware.RequirePermission("reservations.view"), handlers.GetReservation)
	admin.Post("/reservations", middleware.RequirePermission("reservations.create"), handlers.CreateReservation)
	admin.Put("/reservations/:id", middleware.RequirePermission("reservations.update"), handlers.UpdateReservation)
	admin.Patch("/reservations/:id/status", middleware.RequirePermission("reservations.update"), handlers.UpdateReservationStatus)
	admin.Delete("/reservations/:id", middleware.RequirePermission("reservations.delete"), handlers.DeleteReservation)
	// Uang muka: catat (admin), validasi/tolak bukti dari pelanggan.
	admin.Get("/reservations/:id/payments", middleware.RequirePermission("reservations.view"), handlers.ListReservationPayments)
	admin.Post("/reservations/:id/payments", middleware.RequirePermission("reservations.update"), handlers.AddReservationPayment)
	admin.Post("/reservations/:id/payments/:pid/validate", middleware.RequirePermission("reservations.update"), handlers.ValidateReservationPayment)
	admin.Post("/reservations/:id/payments/:pid/reject", middleware.RequirePermission("reservations.update"), handlers.RejectReservationPayment)

	// Foto produk
	admin.Post("/products/:id/photo", middleware.RequirePermission("products.update"), handlers.UploadProductPhoto)
	admin.Delete("/products/:id/photo", middleware.RequirePermission("products.update"), handlers.DeleteProductPhoto)

	admin.Post("/vendors", middleware.RequirePermission("vendors.create"), handlers.CreateVendor)
	admin.Put("/vendors/:id", middleware.RequirePermission("vendors.update"), handlers.UpdateVendor)
	admin.Post("/vendors/:id/toggle", middleware.RequirePermission("vendors.update"), handlers.ToggleVendorActive)
	admin.Delete("/vendors/:id", middleware.RequirePermission("vendors.delete"), handlers.DeleteVendor)

	// Warehouse / Inventory — CRUD granular
	// Shared Gudang read lookups (warehouse selector, item picker, ledger) are
	// cross-loaded by every Gudang page, so any warehouse-module viewer may read them.
	whView := []string{"warehouse_dashboard.view", "warehouses.view", "stockitems.view", "stocktransfers.view", "stockwastes.view", "stockledger.view", "recipes.view"}
	admin.Get("/warehouse-dashboard", middleware.RequirePermission("warehouse_dashboard.view"), handlers.GetWarehouseDashboard)
	admin.Get("/warehouses", middleware.RequireAnyPermission(whView...), handlers.ListWarehouses)
	admin.Get("/warehouses/:id", middleware.RequireAnyPermission(whView...), handlers.GetWarehouse)
	admin.Post("/warehouses", middleware.RequirePermission("warehouses.create"), handlers.CreateWarehouse)
	admin.Put("/warehouses/:id", middleware.RequirePermission("warehouses.update"), handlers.UpdateWarehouse)
	admin.Delete("/warehouses/:id", middleware.RequirePermission("warehouses.delete"), handlers.DeleteWarehouse)

	// Stock Items
	admin.Get("/stock-items", middleware.RequireAnyPermission(whView...), handlers.ListStockItems)
	admin.Get("/stock-items/:id", middleware.RequireAnyPermission(whView...), handlers.GetStockItem)
	admin.Post("/stock-items", middleware.RequirePermission("stockitems.create"), handlers.CreateStockItem)
	admin.Put("/stock-items/:id", middleware.RequirePermission("stockitems.update"), handlers.UpdateStockItem)
	admin.Post("/stock-items/:id/toggle", middleware.RequirePermission("stockitems.update"), handlers.ToggleStockItemActive)
	admin.Delete("/stock-items/:id", middleware.RequirePermission("stockitems.delete"), handlers.DeleteStockItem)
	admin.Get("/stock-item-categories", middleware.RequireAnyPermission(whView...), handlers.ListStockItemCategories)
	admin.Post("/stock-item-categories", middleware.RequirePermission("stockitems.create"), handlers.CreateStockItemCategory)
	admin.Put("/stock-item-categories/:id", middleware.RequirePermission("stockitems.update"), handlers.UpdateStockItemCategory)
	admin.Delete("/stock-item-categories/:id", middleware.RequirePermission("stockitems.delete"), handlers.DeleteStockItemCategory)

	// Stock Ledger & Movements
	admin.Get("/stock-ledger", middleware.RequireAnyPermission(whView...), handlers.GetStockLedger)
	admin.Get("/stock-movements", middleware.RequireAnyPermission(whView...), handlers.GetStockMovements)
	admin.Post("/stock-adjustments", middleware.RequirePermission("stockledger.adjust"), handlers.CreateAdjustment)

	// Stock Transfers
	admin.Get("/stock-transfers", middleware.RequirePermission("stocktransfers.view"), handlers.ListStockTransfers)
	admin.Get("/stock-transfers/:id", middleware.RequirePermission("stocktransfers.view"), handlers.GetStockTransfer)
	admin.Post("/stock-transfers", middleware.RequirePermission("stocktransfers.create"), handlers.CreateStockTransfer)
	// Tahap transfer punya izin masing-masing (setujui / kirim / terima);
	// pemeriksaannya per status ada di dalam handler.
	admin.Put("/stock-transfers/:id/status", middleware.RequireAnyPermission("stocktransfers.update", "stocktransfers.approve", "stocktransfers.receive"), handlers.UpdateTransferStatus)
	admin.Put("/stock-transfers/:id/items/:itemId/received", middleware.RequireAnyPermission("stocktransfers.update", "stocktransfers.receive"), handlers.UpdateReceivedQty)

	// Recipes
	admin.Get("/products/:id/recipes", middleware.RequirePermission("recipes.view"), handlers.GetProductRecipes)
	admin.Put("/products/:id/recipes", middleware.RequirePermission("recipes.update"), handlers.SaveProductRecipes)
	admin.Get("/stock-items/:id/recipes", middleware.RequirePermission("recipes.view"), handlers.GetStockItemRecipes)
	admin.Put("/stock-items/:id/recipes", middleware.RequirePermission("recipes.update"), handlers.SaveStockItemRecipes)
	admin.Post("/stock-items/:id/produce", middleware.RequirePermission("recipes.create"), handlers.ProduceStockItem)
	admin.Get("/recipe-masters", middleware.RequirePermission("recipes.view"), handlers.ListRecipeMasters)
	admin.Get("/recipe-masters/:id", middleware.RequirePermission("recipes.view"), handlers.GetRecipeMaster)
	admin.Post("/recipe-masters", middleware.RequirePermission("recipes.create"), handlers.CreateRecipeMaster)
	admin.Put("/recipe-masters/:id", middleware.RequirePermission("recipes.update"), handlers.UpdateRecipeMaster)
	admin.Delete("/recipe-masters/:id", middleware.RequirePermission("recipes.delete"), handlers.DeleteRecipeMaster)

	// Stock Wastes
	admin.Get("/stock-wastes", middleware.RequirePermission("stockwastes.view"), handlers.ListStockWastes)
	admin.Post("/stock-wastes", middleware.RequirePermission("stockwastes.create"), handlers.CreateStockWaste)

	// Penerimaan Barang (Goods Receipt / GRN) — stok masuk ke gudang.
	// Baca: siapa pun pemirsa modul gudang; tulis: pemegang stockledger.adjust.
	admin.Get("/goods-receipts", middleware.RequireAnyPermission(whView...), handlers.ListGoodsReceipts)
	admin.Get("/goods-receipts/:id", middleware.RequireAnyPermission(whView...), handlers.GetGoodsReceipt)
	admin.Post("/goods-receipts", middleware.RequirePermission("stockledger.adjust"), handlers.CreateGoodsReceipt)

	// PPIC — Fase 1: dashboard, par level/ROP, monitor kedaluwarsa, stock opname.
	// Semua respons difilter scope outlet (gudang pusat + gudang outlet dalam scope).
	admin.Get("/ppic/dashboard", middleware.RequirePermission("ppic.dashboard.view"), handlers.GetPpicDashboard)
	admin.Get("/ppic/planning-params", middleware.RequirePermission("ppic.planning.view"), handlers.ListPlanningParams)
	admin.Put("/ppic/planning-params", middleware.RequirePermission("ppic.planning.update"), handlers.UpsertPlanningParams)
	admin.Post("/ppic/planning-params/auto-fill", middleware.RequirePermission("ppic.planning.update"), handlers.AutoFillPlanningParams)
	admin.Get("/ppic/expiry", middleware.RequirePermission("ppic.expiry.view"), handlers.GetExpiryMonitor)
	admin.Post("/ppic/expiry/:batchId/ack", middleware.RequirePermission("ppic.expiry.ack"), handlers.AckExpiryBatch)
	admin.Get("/ppic/opnames", middleware.RequirePermission("ppic.opname.view"), handlers.ListStockOpnames)
	admin.Get("/ppic/opnames/:id", middleware.RequirePermission("ppic.opname.view"), handlers.GetStockOpname)
	admin.Post("/ppic/opnames", middleware.RequirePermission("ppic.opname.create"), handlers.CreateStockOpname)
	admin.Put("/ppic/opnames/:id/items", middleware.RequirePermission("ppic.opname.create"), handlers.UpdateOpnameItems)
	admin.Post("/ppic/opnames/:id/submit", middleware.RequirePermission("ppic.opname.create"), handlers.SubmitStockOpname)
	admin.Post("/ppic/opnames/:id/approve", middleware.RequirePermission("ppic.opname.approve"), handlers.ApproveStockOpname)
	admin.Post("/ppic/opnames/:id/cancel", middleware.RequirePermission("ppic.opname.create"), handlers.CancelStockOpname)
	// PPIC — Fase 2: demand forecast + MRP (saran beli/transfer masuk alur existing).
	admin.Get("/ppic/forecasts", middleware.RequirePermission("ppic.forecast.view"), handlers.ListForecasts)
	admin.Get("/ppic/forecasts/history", middleware.RequirePermission("ppic.forecast.view"), handlers.GetForecastSalesHistory)
	admin.Put("/ppic/forecasts", middleware.RequirePermission("ppic.forecast.manage"), handlers.UpdateForecasts)
	admin.Post("/ppic/forecasts/generate", middleware.RequirePermission("ppic.forecast.manage"), handlers.GenerateForecasts)
	admin.Get("/ppic/mrp/runs", middleware.RequirePermission("ppic.mrp.view"), handlers.ListMrpRuns)
	admin.Get("/ppic/mrp/runs/:id", middleware.RequirePermission("ppic.mrp.view"), handlers.GetMrpRun)
	admin.Post("/ppic/mrp/run", middleware.RequirePermission("ppic.mrp.run"), handlers.RunMrp)
	admin.Post("/ppic/mrp/runs/:id/create-pr", middleware.RequirePermission("ppic.mrp.execute"), handlers.CreateMrpPurchaseRequest)
	admin.Post("/ppic/mrp/runs/:id/create-transfer", middleware.RequirePermission("ppic.mrp.execute"), handlers.CreateMrpTransfer)
	// PPIC — Fase 3: rencana produksi (MPS) + work order + laporan & export.
	admin.Get("/ppic/producible-items", middleware.RequirePermission("ppic.production.view"), handlers.ListProducibleItems)
	admin.Get("/ppic/production-plans", middleware.RequirePermission("ppic.production.view"), handlers.ListProductionPlans)
	admin.Get("/ppic/production-plans/:id", middleware.RequirePermission("ppic.production.view"), handlers.GetProductionPlan)
	admin.Post("/ppic/production-plans", middleware.RequirePermission("ppic.production.create"), handlers.CreateProductionPlan)
	admin.Post("/ppic/production-plans/:id/approve", middleware.RequirePermission("ppic.production.approve"), handlers.ApproveProductionPlan)
	admin.Post("/ppic/production-plans/:id/release", middleware.RequirePermission("ppic.production.approve"), handlers.ReleaseProductionPlan)
	admin.Post("/ppic/production-plans/:id/cancel", middleware.RequirePermission("ppic.production.create"), handlers.CancelProductionPlan)
	admin.Get("/ppic/work-orders", middleware.RequirePermission("ppic.workorders.view"), handlers.ListWorkOrders)
	admin.Get("/ppic/work-orders/:id", middleware.RequirePermission("ppic.workorders.view"), handlers.GetWorkOrder)
	admin.Post("/ppic/work-orders", middleware.RequirePermission("ppic.workorders.create"), handlers.CreateWorkOrder)
	admin.Post("/ppic/work-orders/:id/start", middleware.RequirePermission("ppic.workorders.execute"), handlers.StartWorkOrder)
	admin.Post("/ppic/work-orders/:id/finish", middleware.RequirePermission("ppic.workorders.execute"), handlers.FinishWorkOrder)
	admin.Post("/ppic/work-orders/:id/cancel", middleware.RequirePermission("ppic.workorders.create"), handlers.CancelWorkOrder)
	admin.Get("/ppic/reports/:tab", middleware.RequirePermission("ppic.reports.view"), handlers.GetPpicReport)
	admin.Get("/ppic/reports/:tab/export", middleware.RequirePermission("ppic.reports.export"), handlers.ExportPpicReport)

	// Settings — view + update only (no create/delete for settings)
	admin.Get("/settings", middleware.RequirePermission("settings.company.view"), handlers.GetAllSettings)
	admin.Get("/settings/company", middleware.RequirePermission("settings.company.view"), handlers.GetCompanyIdentity)
	admin.Put("/settings/company", middleware.RequirePermission("settings.company.update"), handlers.UpdateCompanyIdentity)
	admin.Get("/settings/timezone", middleware.RequirePermission("settings.timezone.view"), handlers.GetTimezone)
	admin.Put("/settings/timezone", middleware.RequirePermission("settings.timezone.update"), handlers.UpdateTimezone)
	admin.Get("/settings/tax", middleware.RequirePermission("settings.tax.view"), handlers.GetTaxSettings)
	admin.Put("/settings/tax", middleware.RequirePermission("settings.tax.update"), handlers.UpdateTaxSettings)
}
