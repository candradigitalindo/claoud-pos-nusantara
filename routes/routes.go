package routes

import (
	"cloud-pos/config"
	"cloud-pos/handlers"
	"cloud-pos/middleware"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// broadcastAfter membungkus handler mutasi supaya memancarkan event SSE
// realtime ke browser admin hanya setelah handler selesai sukses (HTTP 2xx).
func broadcastAfter(h fiber.Handler, events ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := h(c); err != nil {
			return err
		}
		if code := c.Response().StatusCode(); code >= 200 && code < 300 {
			outletID := c.Params("outletId")
			for _, ev := range events {
				services.BroadcastSync(ev, outletID)
			}
		}
		return nil
	}
}

func Setup(app *fiber.App, cfg *config.Config) {
	// Media CCTV harus melewati pemeriksaan akun, izin, outlet, dan sesi HLS.
	app.Get("/cctv/*", handlers.CCTVRelay)
	// App distribution files: force the correct MIME + download disposition so an
	// APK saves as ".apk" (not ".zip"). Registered before the static handler so it
	// takes precedence for /uploads/apps/<name>.
	app.Get("/uploads/apps/:name", handlers.DownloadAppFile)
	// Product photos must reflect deletion immediately, without the generic
	// static handler's cache of open file descriptors. The wildcard also ends
	// malformed/missing photo requests here instead of falling through to it.
	app.Get("/uploads/products/:name", handlers.ServeProductPhoto)
	app.Get("/uploads/products/*", handlers.ServeProductPhoto)

	// Serve uploaded files
	app.Static("/uploads", "./uploads")

	api := app.Group("/api/v1")

	// Public
	api.Get("/ping", handlers.Ping)
	api.Get("/timezone", handlers.GetTimezone) // public — used by frontend for time formatting
	// Pembaruan dalam-aplikasi POS: APK POS terbaru (versionCode, URL, SHA-256).
	// Publik karena APK-nya sendiri memang publik di /uploads/apps; tanpa auth
	// supaya tablet station (tanpa API key outlet) juga bisa memeriksa.
	api.Get("/app-update/pos", handlers.PosAppUpdate)

	// Public reservation (no auth) — per-outlet by slug, dibatasi per IP
	public := api.Group("/public", middleware.PublicRateLimiter())
	public.Get("/outlets/:slug/menu", handlers.PublicGetMenu)
	public.Post("/outlets/:slug/reservations", broadcastAfter(handlers.PublicCreateReservation, "reservation"))
	// Cek status + unggah bukti DP oleh pelanggan (id reservasi = ULID, tidak bisa ditebak).
	public.Get("/outlets/:slug/reservations/:id", handlers.PublicGetReservation)
	public.Post("/outlets/:slug/reservations/:id/payments", broadcastAfter(handlers.PublicSubmitReservationPayment, "reservation"))

	public.Post("/outlets/:slug/orders", broadcastAfter(handlers.PublicCreateOrder, "order"))
	public.Get("/outlets/:slug/orders/:orderId", handlers.PublicOrderStatus)
	public.Post("/outlets/:slug/orders/:orderId/simulate-paid", broadcastAfter(handlers.PublicSimulateOrderPaid, "order", "transaction"))

	// Data terbuka (tanpa auth), outlet dirujuk lewat slug, tanpa ID internal.
	// Sengaja BUKAN di bawah /public: middleware grup Fiber dicocokkan dengan
	// prefiks string, jadi rute di bawah /public (atau /public-...) ikut kena
	// limiter halaman pemesanan pelanggan.
	open := api.Group("/open", middleware.OpenDataRateLimiter())
	open.Get("/outlets", handlers.OpenListOutlets)
	open.Get("/outlets/:slug/products", handlers.OpenOutletProducts)
	open.Get("/outlets/:slug/product-sales", handlers.OpenOutletProductSales)
	open.Get("/social", handlers.OpenSocial)

	// Callback penyedia pembayaran. Tanpa AuthOutlet — keasliannya dibuktikan
	// tanda tangan yang diverifikasi adapter penyedia, bukan API key outlet.
	api.Post("/webhooks/qris/:provider", broadcastAfter(handlers.QRISWebhook, "reservation", "order", "transaction"))

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
	outlet.Post("/reservations/:rid/settle", broadcastAfter(handlers.OutletSettleReservation, "reservation", "transaction"))

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

	// Pemesanan mandiri tamu (QR dine-in) — ditarik POS
	outlet.Get("/online-orders", handlers.GetPendingOnlineOrders)
	outlet.Post("/online-orders/:orderId/claim", broadcastAfter(handlers.ClaimOnlineOrder, "order"))
	outlet.Post("/online-orders/:orderId/confirm", broadcastAfter(handlers.ConfirmOnlineOrder, "order"))
	outlet.Post("/online-orders/:orderId/reject", broadcastAfter(handlers.RejectOnlineOrder, "order"))

	// QRIS terintegrasi
	outlet.Get("/qris/info", handlers.GetPaymentGatewayInfo)
	outlet.Post("/qris/charges", handlers.CreateQRISCharge)
	outlet.Get("/qris/charges/:chargeId", handlers.GetQRISCharge)
	outlet.Post("/qris/charges/:chargeId/simulate-paid", broadcastAfter(handlers.SimulateQRISPaid, "reservation", "order", "transaction"))

	// Printers
	outlet.Get("/printers", handlers.GetOutletPrinters)
	outlet.Post("/printers", handlers.PushPrinter)

	// Analytics
	outlet.Post("/analytics/daily", handlers.PushAnalytics)
	outlet.Get("/analytics", handlers.GetAnalytics)

	// Batch sync
	outlet.Post("/sync/batch", handlers.BatchSync)

	// Bayar Pengadaan dari Kas Kasir Outlet — tablet mengambil tugas langsung
	// (online, tidak lewat sinkron), mengklaim, lalu menyelesaikan/mengembalikan.
	outlet.Get("/procurement-payouts", handlers.ListOutletCashPayouts)
	outlet.Post("/procurement-payouts/:id/claim", broadcastAfter(handlers.ClaimCashPayout, "procurement_payment", "procurement"))
	outlet.Post("/procurement-payouts/:id/complete", broadcastAfter(handlers.CompleteCashPayout, "procurement_payment", "procurement"))
	outlet.Post("/procurement-payouts/:id/return", broadcastAfter(handlers.ReturnCashPayout, "procurement_payment", "procurement"))
	outlet.Post("/procurement-payouts/:id/release", broadcastAfter(handlers.ReleaseCashPayout, "procurement_payment", "procurement"))

	// Updates
	outlet.Get("/updates", handlers.GetUpdates)

	// Restore — ambil state tertinggal setelah device reset database
	outlet.Get("/restore", handlers.GetRestore)

	// Heartbeat perangkat — telemetri tablet kasir (baterai, storage, printer, jaringan)
	outlet.Post("/heartbeat", handlers.PushDeviceHeartbeat)

	// Conflicts
	outlet.Get("/conflicts", handlers.GetConflicts)
	outlet.Post("/conflicts/:conflictId/resolve", broadcastAfter(handlers.ResolveConflict, "transaction"))

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
	api.Post("/cctv/logout", handlers.ClearCCTVSession)
	gateway := api.Group("/cctv/gateway", handlers.CCTVGatewayAuth)
	gateway.Post("/poll", handlers.PollCCTVGateway)
	gateway.Post("/result", handlers.CompleteCCTVGateway)

	// Admin API (authenticated by admin token or JWT)
	admin := api.Group("/admin", middleware.AdminAuth(cfg))

	documents := admin.Group("/documents", middleware.RequirePermission("documents.view"))
	documents.Get("/", handlers.ListDocuments)
	documents.Post("/", middleware.RequirePermission("documents.create"), handlers.UploadDocument)
	// POST avoids counting browser prefetches and HEAD requests as downloads.
	documents.Post("/:id/download", middleware.RequirePermission("documents.download"), handlers.DownloadDocument)

	// Work Units (Move to top to avoid 404/conflicts)
	admin.Get("/work-units/me", middleware.RequireAnyPermission("procurement.requests.view", "procurement.rawmaterials.view", "finance.payments.view"), handlers.GetMyWorkUnit)
	admin.Get("/my-work-units", middleware.RequireAnyPermission("procurement.requests.view", "procurement.rawmaterials.view", "finance.payments.view"), handlers.GetMyWorkUnits)
	admin.Get("/work-units", middleware.RequireAnyPermission("workunits.view", "finance.payments.view"), handlers.ListWorkUnits)
	admin.Get("/work-units/:id", middleware.RequirePermission("workunits.view"), handlers.GetWorkUnit)
	admin.Post("/work-units", middleware.RequirePermission("workunits.create"), handlers.CreateWorkUnit)
	admin.Put("/work-units/:id", middleware.RequirePermission("workunits.update"), handlers.UpdateWorkUnit)

	// App POS distributables (APK upload + download URL) — Master Data
	admin.Get("/app-files", middleware.RequirePermission("appfiles.view"), handlers.ListAppFiles)
	admin.Get("/app-files/devices", middleware.RequirePermission("appfiles.view"), handlers.AppFileDevices)
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
	admin.Post("/shift-reconciliation/:shiftId/apply", middleware.RequireSuperadmin(), broadcastAfter(handlers.ApplyShiftAdjustment, "shift_reconciliation", "cashier_shift"))
	admin.Post("/shift-reconciliation/adjustments/:id/revert", middleware.RequireSuperadmin(), broadcastAfter(handlers.RevertShiftAdjustment, "shift_reconciliation", "cashier_shift"))
	admin.Get("/devices", middleware.RequirePermission("devices.view"), handlers.GetDeviceMonitor)
	admin.Get("/devices/:outletId/history", middleware.RequirePermission("devices.view"), handlers.GetDeviceHistory)

	// CCTV — monitoring live/putar ulang (view) + kelola kamera (create/update/delete).
	// Relay-only: cloud tidak menyimpan video; hanya konfigurasi + relay go2rtc.
	admin.Get("/cameras", middleware.RequirePermission("cameras.view"), handlers.GetCameras)
	admin.Get("/cameras/options", middleware.RequirePermission("cameras.view"), handlers.GetCCTVOptions)
	admin.Get("/cctv/gateways", middleware.RequirePermission("cameras.view"), handlers.GetCCTVGateways)
	admin.Post("/outlets/:id/cctv-gateway", middleware.RequirePermission("cameras.create"), handlers.CreateCCTVGateway(false))
	admin.Post("/outlets/:id/cctv-gateway/rotate", middleware.RequirePermission("cameras.update"), handlers.CreateCCTVGateway(true))
	admin.Delete("/outlets/:id/cctv-gateway", middleware.RequirePermission("cameras.delete"), handlers.DeleteCCTVGateway)
	admin.Post("/outlets/:id/cameras/probe", middleware.RequireAnyPermission("cameras.create", "cameras.update"), handlers.ProbeCameraConfiguration)
	admin.Post("/outlets/:id/cameras/onvif/profiles", middleware.RequireAnyPermission("cameras.create", "cameras.update"), handlers.DiscoverCameraONVIFProfiles)
	admin.Post("/outlets/:id/cameras/tuya/discover", middleware.RequireAnyPermission("cameras.create", "cameras.update"), handlers.DiscoverTuyaCameras)
	admin.Post("/cameras/:camId/test", middleware.RequirePermission("cameras.view"), handlers.TestCameraConnection)
	admin.Post("/cameras/session", middleware.RequirePermission("cameras.view"), handlers.RefreshCCTVSession)
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
	admin.Get("/business-calendar", middleware.RequirePermission("reports.business_analysis.view"), handlers.ListBusinessCalendar)
	admin.Put("/business-calendar", middleware.RequirePermission("reports.business_analysis.manage"), handlers.UpsertBusinessCalendar)
	admin.Delete("/business-calendar/:day", middleware.RequirePermission("reports.business_analysis.manage"), handlers.DeleteBusinessCalendar)

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

	// F&B: menu development and standard cost; warehouse mutations keep their own permissions.
	admin.Get("/fnb/dashboard", middleware.RequirePermission("fnb.dashboard.view"), handlers.FnbOverview)
	admin.Get("/fnb/costing", middleware.RequirePermission("fnb.costing.view"), handlers.FnbCosting)
	admin.Get("/fnb/outlets/:id/policy", middleware.RequirePermission("fnb.costing.view"), handlers.GetFnbOutletPolicy)
	admin.Put("/fnb/outlets/:id/policy", middleware.RequirePermission("fnb.costing.update"), handlers.SaveFnbOutletPolicy)
	admin.Put("/fnb/products/:id/profile", middleware.RequirePermission("fnb.costing.update"), handlers.SaveFnbProfile)
	admin.Get("/fnb/development-options", middleware.RequirePermission("fnb.development.view"), handlers.FnbDevelopmentOptions)
	admin.Get("/fnb/developments", middleware.RequirePermission("fnb.development.view"), handlers.ListFnbDevelopments)
	admin.Post("/fnb/developments", middleware.RequirePermission("fnb.development.create"), handlers.SaveFnbDevelopment)
	admin.Put("/fnb/developments/:id", middleware.RequirePermission("fnb.development.update"), handlers.SaveFnbDevelopment)
	admin.Put("/fnb/developments/:id/status", middleware.RequireAnyPermission("fnb.development.update", "fnb.development.approve"), handlers.TransitionFnbDevelopment)

	// Products & Categories — CRUD granular
	admin.Get("/products", middleware.RequirePermission("products.view"), handlers.AdminGetProducts)
	admin.Get("/products/classification", middleware.RequirePermission("products.view"), handlers.PreviewProductClassification)
	admin.Post("/products/classification", middleware.RequirePermission("products.update"), handlers.ApplyProductClassification)
	admin.Get("/products/recipe-options", middleware.RequireAnyPermission("products.create", "products.update"), handlers.AdminProductRecipeOptions)
	admin.Get("/products/recipe-options/:id", middleware.RequireAnyPermission("products.create", "products.update"), handlers.AdminProductRecipeDetail)
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
	admin.Get("/roles/notification-options", middleware.RequirePermission("roles.view"), handlers.GetRoleNotificationOptions)
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
	admin.Put("/roles/:role/notifications", middleware.RequirePermission("roles.update"), handlers.UpdateRoleNotifications)
	admin.Delete("/roles/:role/notifications", middleware.RequirePermission("roles.update"), handlers.ResetRoleNotifications)

	// Purchase requests — granular: submit, approve, purchasing (isi harga), finance (bayar)
	admin.Get("/procurement-dashboard", middleware.RequirePermission("procurement.dashboard.view"), handlers.GetProcurementDashboardGlobal)
	admin.Get("/payment-stats", middleware.RequirePermission("finance.payments.view"), handlers.GetPaymentStats)
	admin.Get("/finance/accounts", middleware.RequireAnyPermission("finance.entries.view", "reports.ledger.view", "reports.pnl.view", "reports.balance.view"), handlers.FinanceAccounts)
	admin.Get("/finance/lookups", middleware.RequirePermission("finance.entries.view"), handlers.FinanceLookups)
	admin.Get("/finance/operations", middleware.RequirePermission("finance.entries.view"), handlers.ListFinanceOperations)
	admin.Put("/finance/cash-movements/:id/account", middleware.RequirePermission("finance.entries.post"), broadcastAfter(handlers.ClassifyFinanceCash, "finance_entry", "cash_movement"))
	admin.Get("/finance/entries", middleware.RequirePermission("finance.entries.view"), handlers.ListFinanceEntries)
	admin.Get("/finance/entries/:id", middleware.RequirePermission("finance.entries.view"), handlers.GetFinanceEntry)
	admin.Post("/finance/entries", middleware.RequirePermission("finance.entries.create"), broadcastAfter(handlers.SaveFinanceEntry, "finance_entry"))
	admin.Put("/finance/entries/:id", middleware.RequirePermission("finance.entries.create"), broadcastAfter(handlers.SaveFinanceEntry, "finance_entry"))
	admin.Post("/finance/entries/:id/actions", middleware.RequireAnyPermission("finance.entries.post", "finance.entries.void"), broadcastAfter(handlers.ActFinanceEntry, "finance_entry"))
	// Export Excel memakai permission & scope yang sama dengan halaman Pembayaran.
	admin.Get("/procurement-payments/export", middleware.RequirePermission("finance.payments.view"), handlers.ExportProcurementPayments)
	// Projek (pembangunan/renovasi) — payung RAB di atas pengadaan.
	// Lihat dibuka juga untuk pemegang izin pengadaan: form pengajuan perlu
	// memuat daftar projek untuk dipilih — dan untuk keuangan, karena halaman
	// Pembayaran punya filter Projek (dulu role keuangan murni kena 403 sehingga
	// pilihannya hanya "Semua Projek"). Ubah/hapus tetap terkunci di manage.
	admin.Get("/projects", middleware.RequireAnyPermission("procurement.projects.view", "procurement.requests.submit", "procurement.requests.view", "finance.payments.view"), handlers.ListProjects)
	admin.Get("/projects/:id", middleware.RequirePermission("procurement.projects.view"), handlers.GetProject)
	admin.Post("/projects", middleware.RequirePermission("procurement.projects.manage"), broadcastAfter(handlers.CreateProject, "procurement"))
	admin.Put("/projects/:id", middleware.RequirePermission("procurement.projects.manage"), broadcastAfter(handlers.UpdateProject, "procurement"))
	admin.Delete("/projects/:id", middleware.RequirePermission("procurement.projects.manage"), broadcastAfter(handlers.DeleteProject, "procurement"))
	// RAB per baris: dilihat oleh siapa pun yang boleh mengajukan/melihat
	// pengadaan (form pengajuan memilih baris RAB), diubah oleh pengelola projek.
	admin.Get("/projects/:id/rab", middleware.RequireAnyPermission("procurement.projects.view", "procurement.requests.submit", "procurement.requests.view"), handlers.GetProjectRab)
	admin.Put("/projects/:id/rab", middleware.RequirePermission("procurement.projects.manage"), broadcastAfter(handlers.SaveProjectRab, "procurement"))
	admin.Post("/projects/:id/rab/set", middleware.RequirePermission("procurement.projects.manage"), broadcastAfter(handlers.SetProjectRab, "procurement"))
	admin.Post("/projects/:id/rab/reopen", middleware.RequirePermission("procurement.projects.manage"), broadcastAfter(handlers.ReopenProjectRab, "procurement"))

	// Perlengkapan: laporan penggunaan dengan bukti dan pemeriksa terpisah.
	admin.Get("/project-usage-export", middleware.RequirePermission("assets.usage.view"), handlers.ExportProjectUsage)
	admin.Get("/project-usage/projects", middleware.RequirePermission("assets.usage.view"), handlers.ListUsageProjects)
	admin.Get("/project-usage/projects/:id", middleware.RequirePermission("assets.usage.view"), handlers.ProjectUsageOverview)
	admin.Get("/project-usage", middleware.RequirePermission("assets.usage.view"), handlers.ListProjectUsage)
	admin.Get("/project-usage/:id", middleware.RequirePermission("assets.usage.view"), handlers.GetProjectUsage)
	admin.Post("/project-usage", middleware.RequirePermission("assets.usage.create"), broadcastAfter(handlers.SaveProjectUsage, "asset_workflow"))
	admin.Put("/project-usage/:id", middleware.RequirePermission("assets.usage.create"), broadcastAfter(handlers.SaveProjectUsage, "asset_workflow"))
	admin.Post("/project-usage/:id/actions", middleware.RequirePermission("assets.usage.view"), broadcastAfter(handlers.ActProjectUsage, "asset_workflow"))

	// Material projek — barang habis pakai milik projek (docs §8.8–8.9).
	// Izinnya memakai kunci Projek yang sudah ada (keputusan B, docs §15.1).
	admin.Get("/projects/:id/materials", middleware.RequireAnyPermission("procurement.projects.view", "assets.usage.view"), handlers.ListProjectMaterials)
	admin.Get("/projects/:id/materials/:mid/logs", middleware.RequireAnyPermission("procurement.projects.view", "assets.usage.view"), handlers.ListProjectMaterialLogs)
	admin.Post("/projects/:id/materials/:mid/usage", middleware.RequirePermission("assets.usage.create"), broadcastAfter(handlers.RecordProjectMaterialUsage, "asset_workflow"))
	admin.Post("/projects/:id/materials/:mid/settle", middleware.RequirePermission("assets.usage.create"), broadcastAfter(handlers.SettleProjectMaterial, "asset_workflow"))
	admin.Get("/project-materials/unsettled", middleware.RequirePermission("procurement.projects.view"), handlers.ListUnsettledProjectMaterials)

	// Halaman Pembayaran (finance.payments.view) memuat daftar & detail lewat
	// rute yang sama; handler membatasinya ke daftar exclude_masters dan detail
	// (lihat ListPurchaseRequests/GetPurchaseRequest). Tanpa kunci itu di sini,
	// role keuangan murni ditolak sebelum aturan handler sempat berlaku.
	// Pembelian Darurat (keputusan owner 19 Nov 2026): barang/jasa urgent ≤
	// batas dibeli dulu dari kas laci outlet (tugas Bayar Pengadaan di
	// tablet, PIN Manager), struk diunggah, lalu disahkan penyetuju.
	// Didaftarkan sebelum rute ber-:id.
	admin.Get("/purchase-requests/emergency/context", middleware.RequirePermission("procurement.requests.emergency"), handlers.GetEmergencyContext)
	admin.Post("/purchase-requests/emergency", middleware.RequirePermission("procurement.requests.emergency"), broadcastAfter(handlers.CreateEmergencyPurchase, "procurement", "procurement_payment"))
	admin.Post("/purchase-requests/:id/emergency/proof", middleware.RequireAnyPermission("procurement.requests.emergency", "procurement.requests.submit"), broadcastAfter(handlers.SubmitEmergencyProof, "procurement", "procurement_payment"))
	admin.Post("/purchase-requests/:id/emergency/decision", middleware.RequireAnyPermission("procurement.requests.approve", "procurement.requests.approve_services"), broadcastAfter(handlers.DecideEmergencyPurchase, "procurement", "procurement_payment"))
	admin.Post("/purchase-requests/:id/emergency/cancel", middleware.RequireAnyPermission("procurement.requests.emergency", "procurement.requests.submit", "procurement.requests.approve", "procurement.requests.approve_services"), broadcastAfter(handlers.CancelEmergencyPurchase, "procurement", "procurement_payment"))
	admin.Post("/purchase-requests/:id/emergency/resend", middleware.RequireAnyPermission("procurement.requests.emergency", "procurement.requests.submit"), broadcastAfter(handlers.ResendEmergencyPayout, "procurement", "procurement_payment"))
	admin.Get("/procurement/emergency-settings", middleware.RequirePermission("procurement.requests.view"), handlers.GetEmergencySettings)
	admin.Put("/procurement/emergency-settings", middleware.RequireAnyPermission("procurement.requests.approve", "procurement.requests.approve_services"), handlers.UpdateEmergencySettings)
	// Rekap bulanan Pembelian Darurat untuk audit Keuangan (dan penyetuju).
	admin.Get("/procurement/emergency-recap", middleware.RequireAnyPermission("finance.payments.view", "procurement.requests.approve", "procurement.requests.approve_services"), handlers.GetEmergencyRecap)
	admin.Get("/procurement/emergency-recap/export", middleware.RequireAnyPermission("finance.payments.view", "procurement.requests.approve", "procurement.requests.approve_services"), handlers.ExportEmergencyRecap)
	// Pembayaran Manual & Pengajuan / Persetujuan Pembayaran (Mgr Keuangan / GM)
	admin.Post("/purchase-requests/manual-payment", middleware.RequireAnyPermission("finance.payments.view", "finance.payments.pay", "finance.payments.approve"), broadcastAfter(handlers.CreateManualPayment, "procurement_payment", "procurement"))
	admin.Post("/purchase-requests/payment-submissions", middleware.RequireAnyPermission("finance.payments.view", "finance.payments.pay", "finance.payments.approve"), broadcastAfter(handlers.SubmitPaymentApprovalBatch, "procurement_payment", "procurement"))
	admin.Post("/purchase-requests/payment-approvals", middleware.RequireAnyPermission("finance.payments.view", "finance.payments.pay", "finance.payments.approve"), broadcastAfter(handlers.DecidePaymentApprovalBatch, "procurement_payment", "procurement"))
	admin.Get("/purchase-requests", middleware.RequireAnyPermission("procurement.requests.view", "procurement.rawmaterials.view", "finance.payments.view"), handlers.ListPurchaseRequests)
	admin.Get("/purchasing/export", middleware.RequireAnyPermission("procurement.requests.view", "procurement.rawmaterials.view"), handlers.ExportPurchasing)
	admin.Get("/purchase-requests/:id", middleware.RequireAnyPermission("procurement.requests.view", "procurement.rawmaterials.view", "finance.payments.view"), handlers.GetPurchaseRequest)
	admin.Get("/purchase-requests/:id/payment-histories", middleware.RequirePermission("finance.payments.view"), handlers.GetPaymentHistories)
	admin.Post("/purchase-requests/:id/split", middleware.RequireAnyPermission("procurement.requests.purchasing", "procurement.rawmaterials.purchasing"), broadcastAfter(handlers.SplitPurchaseRequest, "procurement"))
	// Struk pembelian (dibayar per struk) dicatat bagian pembelian atau
	// petugas logistik; pengambilan barang dari vendor oleh petugas logistik.
	// Izin per jenis diperiksa handler.
	strukPerms := []string{"procurement.requests.purchasing", "procurement.rawmaterials.purchasing", "procurement.requests.pickup", "procurement.rawmaterials.pickup"}
	admin.Post("/purchase-requests/:id/struks", middleware.RequireAnyPermission(strukPerms...), broadcastAfter(handlers.AddPurchaseStruk, "procurement", "procurement_payment"))
	admin.Delete("/purchase-requests/:id/struks/:strukId", middleware.RequireAnyPermission(strukPerms...), broadcastAfter(handlers.DeletePurchaseStruk, "procurement", "procurement_payment"))
	admin.Post("/purchase-requests/:id/pickups", middleware.RequireAnyPermission("procurement.requests.pickup", "procurement.rawmaterials.pickup"), broadcastAfter(handlers.AddPurchasePickup, "procurement", "procurement_receiving"))
	// Serah Terima pengadaan → aset / stok gudang (docs/perlengkapan-aset.md §8).
	// Penerima: pengaju jenisnya (semua baris), bagian Aset (assets.create —
	// baris peralatan), atau petugas gudang (stockledger.adjust — baris
	// dapur/stok). Wewenang per baris diputus handler & service (T-10, T-17).
	receivePerms := []string{"procurement.requests.submit", "procurement.rawmaterials.submit", "assets.create", "stockledger.adjust"}
	admin.Get("/purchase-requests/:id/receiving-draft", middleware.RequireAnyPermission(receivePerms...), handlers.GetReceivingDraft)
	admin.Post("/purchase-requests/:id/receive-goods", middleware.RequireAnyPermission(receivePerms...), broadcastAfter(handlers.ReceiveGoods, "procurement", "procurement_receiving", "goods_receipt", "asset_workflow"))
	admin.Post("/purchase-requests", middleware.RequireAnyPermission("procurement.requests.submit", "procurement.rawmaterials.submit"), broadcastAfter(handlers.CreatePurchaseRequest, "procurement"))
	// finance.payments.pay: keuangan membayar (aksi pay); izin per aksi diputus handler.
	admin.Put("/purchase-requests/:id/status", middleware.RequireAnyPermission("procurement.requests.view", "procurement.rawmaterials.view", "finance.payments.pay"), broadcastAfter(handlers.UpdatePurchaseStatus, "procurement", "procurement_payment", "procurement_receiving"))
	// Sumber dana Kas Kasir Outlet: tugas penyerahan kas untuk tablet kasir.
	admin.Post("/procurement-payouts", middleware.RequirePermission("finance.payments.pay"), broadcastAfter(handlers.AssignCashPayout, "procurement_payment", "procurement"))
	admin.Post("/procurement-payouts/:id/cancel", middleware.RequirePermission("finance.payments.pay"), broadcastAfter(handlers.CancelCashPayout, "procurement_payment", "procurement"))
	admin.Put("/purchase-requests/:id/items", middleware.RequireAnyPermission("procurement.requests.view", "procurement.rawmaterials.view"), broadcastAfter(handlers.UpdatePurchaseItems, "procurement"))
	// Tujuan pembayaran: diisi/diubah Pembelian (izin per jenis diputus
	// handler), dikembalikan Keuangan bila salah.
	admin.Put("/purchase-requests/:id/payment-details", middleware.RequireAnyPermission("procurement.requests.purchasing", "procurement.rawmaterials.purchasing"), broadcastAfter(handlers.UpdatePurchasePaymentDetails, "procurement", "procurement_payment"))
	admin.Post("/purchase-requests/:id/payment-details/return", middleware.RequirePermission("finance.payments.pay"), broadcastAfter(handlers.ReturnPurchasePaymentDetails, "procurement", "procurement_payment"))
	admin.Delete("/purchase-requests/:id", middleware.RequireAnyPermission("procurement.requests.submit", "procurement.rawmaterials.submit"), broadcastAfter(handlers.DeletePurchaseRequest, "procurement", "procurement_payment", "procurement_receiving"))

	// File upload — dipakai untuk bukti pembayaran pengadaan
	// stockledger.adjust: petugas gudang mengunggah foto serah terima barang dapur.
	admin.Post("/upload", middleware.RequireAnyPermission("finance.entries.create", "finance.payments.view", "procurement.requests.view", "procurement.rawmaterials.view", "assets.create", "assets.update", "assets.usage.create", "reservations.update", "stockledger.adjust"), handlers.UploadFile)

	// Bank Accounts
	// finance.payments.pay ikut boleh membaca daftar: form bayar di halaman
	// Pembayaran wajib memilih rekening asal, jadi role yang hanya diberi hak
	// bayar (tanpa kelola rekening) dulu tidak bisa mencatat pembayaran sama
	// sekali. Isinya hanya nama bank, nomor, dan pemilik — tanpa saldo.
	admin.Get("/bank-accounts", middleware.RequireAnyPermission("finance.bank.view", "finance.payments.pay"), handlers.ListBankAccounts)
	admin.Post("/bank-accounts", middleware.RequirePermission("finance.bank.create"), handlers.CreateBankAccount)
	admin.Put("/bank-accounts/:id", middleware.RequirePermission("finance.bank.update"), handlers.UpdateBankAccount)
	admin.Delete("/bank-accounts/:id", middleware.RequirePermission("finance.bank.delete"), handlers.DeleteBankAccount)

	// Vendors
	// Vendor list/lookup is also needed by procurement requesters and finance (to pick a vendor).
	admin.Get("/vendors", middleware.RequireAnyPermission("vendors.view", "procurement.requests.view", "procurement.requests.submit", "procurement.requests.purchasing", "procurement.rawmaterials.view", "procurement.rawmaterials.submit", "procurement.rawmaterials.purchasing", "finance.payments.view"), handlers.ListVendors)
	// finance.payments.view: modal Bayar memuat rekening vendor tujuan dari sini.
	admin.Get("/vendors/:id", middleware.RequireAnyPermission("vendors.view", "procurement.requests.view", "procurement.requests.submit", "procurement.requests.purchasing", "procurement.rawmaterials.view", "procurement.rawmaterials.submit", "procurement.rawmaterials.purchasing", "finance.payments.view"), handlers.GetVendor)
	admin.Get("/vendors/:id/detail", middleware.RequirePermission("vendors.view"), handlers.GetVendorDetail)
	admin.Get("/vendors/:id/purchases", middleware.RequirePermission("vendors.view"), handlers.ListVendorPurchases)
	// Manajemen Aset + histori perawatan (scoped per outlet)
	// Impor aset dari Excel. Template dan pemeriksaan berkas dipisah dari
	// penyimpanan, supaya berkas ratusan baris tidak langsung masuk.
	admin.Get("/asset-import/template", middleware.RequirePermission("assets.create"), handlers.DownloadAssetImportTemplate)
	admin.Post("/asset-import", middleware.RequirePermission("assets.create"), broadcastAfter(handlers.ImportAssets, "asset_workflow"))

	// Master kategori aset — dipakai form aset, dialog penerimaan, dan laporan.
	admin.Get("/asset-categories", middleware.RequirePermission("assets.view"), handlers.ListAssetCategories)
	admin.Post("/asset-categories", middleware.RequirePermission("assets.update"), handlers.CreateAssetCategory)
	admin.Put("/asset-categories/:id", middleware.RequirePermission("assets.update"), handlers.UpdateAssetCategory)
	admin.Delete("/asset-categories/:id", middleware.RequirePermission("assets.delete"), handlers.DeleteAssetCategory)

	admin.Get("/assets", middleware.RequirePermission("assets.view"), handlers.ListAssets)
	// Ekspor Excel daftar Perlengkapan: saringan sama dengan layar, tanpa
	// paginasi. Path terpisah supaya tidak pernah bentrok dengan "/assets/:id".
	admin.Get("/asset-export", middleware.RequirePermission("assets.view"), handlers.ExportAssets)
	admin.Get("/assets/:id", middleware.RequirePermission("assets.view"), handlers.GetAsset)
	admin.Post("/assets", middleware.RequirePermission("assets.create"), broadcastAfter(handlers.CreateAsset, "asset_workflow"))
	admin.Put("/assets/:id", middleware.RequirePermission("assets.update"), broadcastAfter(handlers.UpdateAsset, "asset_workflow"))
	admin.Delete("/assets/:id", middleware.RequirePermission("assets.delete"), broadcastAfter(handlers.DeleteAsset, "asset_workflow"))
	admin.Get("/assets/:id/movements", middleware.RequirePermission("assets.view"), handlers.ListAssetMovements)
	// Perawatan dipisah dari assets.update supaya teknisi bisa mencatat pekerjaan
	// tanpa ikut berhak mengubah harga perolehan dan outlet aset.
	admin.Get("/assets/:id/maintenances", middleware.RequirePermission("assets.maintenance.view"), handlers.ListAssetMaintenances)
	admin.Post("/assets/:id/maintenances", middleware.RequirePermission("assets.maintenance.create"), broadcastAfter(handlers.AddAssetMaintenance, "asset_workflow"))
	admin.Delete("/assets/:id/maintenances/:mid", middleware.RequirePermission("assets.maintenance.create"), broadcastAfter(handlers.DeleteAssetMaintenance, "asset_workflow"))

	// Work order perawatan lintas aset. "summary" didaftarkan sebelum ":id".
	// Antrean serah terima. Dua meja: "perlengkapan" (bagian Aset) dan "dapur"
	// (Gudang Induk); izinnya sengaja longgar — yang membatasi adalah kind.
	admin.Get("/receiving-queue", middleware.RequireAnyPermission("assets.view", "stockledger.adjust", "procurement.requests.view", "procurement.rawmaterials.view"), handlers.ListReceivingQueue)
	// Distribusi aset ke PIC pengaju — lanjutan setelah bagian aset menerima.
	admin.Get("/asset-handovers", middleware.RequirePermission("assets.view"), handlers.ListAssetHandovers)
	admin.Get("/asset-handovers/awaiting", middleware.RequirePermission("assets.view"), handlers.ListAssetsAwaitingHandover)
	admin.Get("/asset-handovers/:id", middleware.RequirePermission("assets.view"), handlers.GetAssetHandover)
	admin.Post("/asset-handovers", middleware.RequirePermission("assets.update"), broadcastAfter(handlers.CreateAssetHandover, "asset_workflow", "procurement_receiving"))

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
	admin.Post("/asset-disposals", middleware.RequirePermission("assets.disposal.create"), broadcastAfter(handlers.CreateAssetDisposal, "asset_workflow"))
	admin.Post("/asset-disposals/:id/approve", middleware.RequirePermission("assets.disposal.approve"), broadcastAfter(handlers.ApproveAssetDisposal, "asset_workflow"))

	// Opname aset — audit fisik per outlet (§4.5).
	admin.Get("/asset-opnames", middleware.RequirePermission("assets.opname.view"), handlers.ListAssetOpnames)
	admin.Get("/asset-opnames/:id", middleware.RequirePermission("assets.opname.view"), handlers.GetAssetOpname)
	admin.Post("/asset-opnames", middleware.RequirePermission("assets.opname.create"), broadcastAfter(handlers.CreateAssetOpname, "asset_workflow"))
	admin.Put("/asset-opnames/:id/items", middleware.RequirePermission("assets.opname.create"), broadcastAfter(handlers.SaveAssetOpnameCount, "asset_workflow"))
	admin.Post("/asset-opnames/:id/approve", middleware.RequirePermission("assets.opname.approve"), broadcastAfter(handlers.ApproveAssetOpname, "asset_workflow"))
	admin.Get("/asset-maintenances", middleware.RequirePermission("assets.maintenance.view"), handlers.ListMaintenances)
	admin.Get("/asset-maintenances/summary", middleware.RequirePermission("assets.maintenance.view"), handlers.GetMaintenanceSummary)
	admin.Get("/asset-maintenances/:id", middleware.RequirePermission("assets.maintenance.view"), handlers.GetMaintenance)
	admin.Post("/asset-maintenances/:id/start", middleware.RequirePermission("assets.maintenance.create"), broadcastAfter(handlers.StartMaintenance, "asset_workflow"))
	admin.Post("/asset-maintenances/:id/complete", middleware.RequirePermission("assets.maintenance.create"), broadcastAfter(handlers.CompleteMaintenance, "asset_workflow"))
	admin.Post("/asset-maintenances/:id/cancel", middleware.RequirePermission("assets.maintenance.create"), broadcastAfter(handlers.CancelMaintenance, "asset_workflow"))
	admin.Post("/asset-maintenances/:id/purchase-request", middleware.RequireAnyPermission("assets.maintenance.create"), broadcastAfter(handlers.CreateMaintenancePurchaseRequest, "asset_workflow", "procurement"))

	// Mutasi aset antar outlet. Path terpisah dari /assets supaya rute statis
	// seperti "available" tidak pernah tertangkap sebagai "/assets/:id".
	// "available" HARUS didaftarkan sebelum ":id" — Fiber mencocokkan berurutan.
	admin.Get("/asset-transfers", middleware.RequirePermission("assets.transfer.view"), handlers.ListAssetTransfers)
	admin.Get("/asset-transfers/available", middleware.RequirePermission("assets.transfer.create"), handlers.ListTransferableAssets)
	admin.Get("/asset-transfers/:id", middleware.RequirePermission("assets.transfer.view"), handlers.GetAssetTransfer)
	admin.Post("/asset-transfers", middleware.RequirePermission("assets.transfer.create"), broadcastAfter(handlers.CreateAssetTransfer, "asset_workflow"))
	admin.Put("/asset-transfers/:id", middleware.RequirePermission("assets.transfer.create"), broadcastAfter(handlers.UpdateAssetTransfer, "asset_workflow"))
	admin.Delete("/asset-transfers/:id", middleware.RequirePermission("assets.transfer.create"), broadcastAfter(handlers.DeleteAssetTransfer, "asset_workflow"))
	admin.Post("/asset-transfers/:id/submit", middleware.RequirePermission("assets.transfer.create"), broadcastAfter(handlers.SubmitAssetTransfer, "asset_workflow"))
	admin.Post("/asset-transfers/:id/approve", middleware.RequirePermission("assets.transfer.approve"), broadcastAfter(handlers.ApproveAssetTransfer, "asset_workflow"))
	admin.Post("/asset-transfers/:id/reject", middleware.RequirePermission("assets.transfer.approve"), broadcastAfter(handlers.RejectAssetTransfer, "asset_workflow"))
	admin.Post("/asset-transfers/:id/send", middleware.RequirePermission("assets.transfer.create"), broadcastAfter(handlers.SendAssetTransfer, "asset_workflow"))
	admin.Post("/asset-transfers/:id/receive", middleware.RequirePermission("assets.transfer.receive"), broadcastAfter(handlers.ReceiveAssetTransfer, "asset_workflow"))
	admin.Post("/asset-transfers/:id/cancel", middleware.RequirePermission("assets.transfer.create"), broadcastAfter(handlers.CancelAssetTransfer, "asset_workflow"))

	// Pelanggan (Penjualan) — master otomatis dari order kasir, scoped per outlet
	admin.Get("/customers", middleware.RequirePermission("customers.view"), handlers.ListCustomers)
	admin.Get("/customers/:id", middleware.RequirePermission("customers.view"), handlers.GetCustomer)

	// Reservasi (Penjualan), scoped per outlet. Rute statis /settings didaftarkan
	// sebelum /:id supaya tidak tertangkap sebagai id.
	admin.Get("/reservations/settings", middleware.RequirePermission("reservations.view"), handlers.GetReservationSettings)
	admin.Put("/reservations/settings", middleware.RequirePermission("reservations.update"), handlers.UpdateReservationSettings)
	admin.Get("/reservations", middleware.RequirePermission("reservations.view"), handlers.ListReservations)
	admin.Get("/reservations/:id", middleware.RequirePermission("reservations.view"), handlers.GetReservation)
	admin.Post("/reservations", middleware.RequirePermission("reservations.create"), broadcastAfter(handlers.CreateReservation, "reservation"))
	admin.Put("/reservations/:id", middleware.RequirePermission("reservations.update"), broadcastAfter(handlers.UpdateReservation, "reservation"))
	admin.Patch("/reservations/:id/status", middleware.RequirePermission("reservations.update"), broadcastAfter(handlers.UpdateReservationStatus, "reservation"))
	admin.Delete("/reservations/:id", middleware.RequirePermission("reservations.delete"), broadcastAfter(handlers.DeleteReservation, "reservation"))
	// Uang muka: catat (admin), validasi/tolak bukti dari pelanggan.
	admin.Get("/reservations/:id/payments", middleware.RequirePermission("reservations.view"), handlers.ListReservationPayments)
	admin.Post("/reservations/:id/payments", middleware.RequirePermission("reservations.update"), broadcastAfter(handlers.AddReservationPayment, "reservation"))
	admin.Post("/reservations/:id/payments/:pid/validate", middleware.RequirePermission("reservations.update"), broadcastAfter(handlers.ValidateReservationPayment, "reservation"))
	admin.Post("/reservations/:id/payments/:pid/reject", middleware.RequirePermission("reservations.update"), broadcastAfter(handlers.RejectReservationPayment, "reservation"))

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
	whView := []string{"warehouse_dashboard.view", "warehouses.view", "stockitems.view", "stocktransfers.view", "stockwastes.view", "stockledger.view", "recipes.view", "stockquality.view"}
	admin.Get("/warehouse-dashboard", middleware.RequirePermission("warehouse_dashboard.view"), handlers.GetWarehouseDashboard)
	admin.Get("/warehouses", middleware.RequireAnyPermission(whView...), handlers.ListWarehouses)
	admin.Get("/warehouses/:id", middleware.RequireAnyPermission(whView...), handlers.GetWarehouse)
	admin.Post("/warehouses", middleware.RequirePermission("warehouses.create"), handlers.CreateWarehouse)
	admin.Put("/warehouses/:id", middleware.RequirePermission("warehouses.update"), handlers.UpdateWarehouse)
	admin.Delete("/warehouses/:id", middleware.RequirePermission("warehouses.delete"), handlers.DeleteWarehouse)

	// Stock Items
	admin.Get("/stock-items", middleware.RequireAnyPermission(whView...), handlers.ListStockItems)
	// Saran nama katalog di form pengadaan bahan baku (nama + satuan saja).
	admin.Get("/stock-item-names", middleware.RequireAnyPermission(append(append([]string{}, whView...), "procurement.rawmaterials.view")...), handlers.ListStockCatalogNames)
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
	admin.Post("/stock-adjustments", middleware.RequirePermission("stockledger.adjust"), broadcastAfter(handlers.CreateAdjustment, "goods_receipt", "ppic_alert"))

	// Stock Transfers
	admin.Get("/stock-transfers", middleware.RequirePermission("stocktransfers.view"), handlers.ListStockTransfers)
	admin.Get("/stock-transfers/:id", middleware.RequirePermission("stocktransfers.view"), handlers.GetStockTransfer)
	admin.Post("/stock-transfers/:id/receive", middleware.RequireAnyPermission("stocktransfers.receive", "stockquality.central.receive", "stockquality.outlet.receive"), broadcastAfter(handlers.ReceiveStockTransfer, "stock_transfer", "goods_receipt", "stock_quality"))
	admin.Post("/stock-transfers", middleware.RequirePermission("stocktransfers.create"), broadcastAfter(handlers.CreateStockTransfer, "stock_transfer"))
	// Tahap transfer punya izin masing-masing (setujui / kirim / terima);
	// pemeriksaannya per status ada di dalam handler.
	admin.Put("/stock-transfers/:id/status", middleware.RequireAnyPermission("stocktransfers.update", "stocktransfers.approve", "stocktransfers.receive"), broadcastAfter(handlers.UpdateTransferStatus, "stock_transfer"))
	admin.Put("/stock-transfers/:id/items/:itemId/received", middleware.RequireAnyPermission("stocktransfers.update", "stocktransfers.receive"), broadcastAfter(handlers.UpdateReceivedQty, "stock_transfer"))

	// Recipes
	admin.Get("/products/:id/recipes", middleware.RequirePermission("recipes.view"), handlers.GetProductRecipes)
	admin.Put("/products/:id/recipes", middleware.RequirePermission("recipes.update"), handlers.SaveProductRecipes)
	admin.Get("/stock-items/:id/recipes", middleware.RequirePermission("recipes.view"), handlers.GetStockItemRecipes)
	admin.Put("/stock-items/:id/recipes", middleware.RequirePermission("recipes.update"), handlers.SaveStockItemRecipes)
	admin.Post("/stock-items/:id/produce", middleware.RequirePermission("stockledger.adjust"), broadcastAfter(handlers.ProduceStockItem, "goods_receipt", "ppic_alert"))
	// Pemotongan (satu bahan → beberapa hasil, mis. ayam utuh → paha/dada/sayap).
	// Template dibaca juga oleh pemegang stockledger.adjust (form Pemotongan
	// butuh hasil standar & bobotnya); diubah hanya pengelola katalog (handler).
	admin.Get("/stock-breakdown-templates", middleware.RequireAnyPermission("recipes.view", "stockledger.adjust"), handlers.ListStockItemBreakdowns)
	admin.Get("/stock-items/:id/breakdown", middleware.RequireAnyPermission("recipes.view", "stockledger.adjust"), handlers.GetStockItemBreakdown)
	admin.Put("/stock-items/:id/breakdown", middleware.RequirePermission("recipes.update"), handlers.SaveStockItemBreakdown)
	admin.Get("/stock-breakdowns", middleware.RequireAnyPermission(whView...), handlers.ListStockBreakdowns)
	admin.Post("/stock-breakdowns", middleware.RequirePermission("stockledger.adjust"), broadcastAfter(handlers.CreateStockBreakdown, "goods_receipt", "ppic_alert"))
	admin.Get("/recipe-masters", middleware.RequirePermission("recipes.view"), handlers.ListRecipeMasters)
	admin.Get("/recipe-masters/:id", middleware.RequirePermission("recipes.view"), handlers.GetRecipeMaster)
	admin.Post("/recipe-masters", middleware.RequirePermission("recipes.create"), handlers.CreateRecipeMaster)
	admin.Put("/recipe-masters/:id", middleware.RequirePermission("recipes.update"), handlers.UpdateRecipeMaster)
	admin.Delete("/recipe-masters/:id", middleware.RequirePermission("recipes.delete"), handlers.DeleteRecipeMaster)

	// Stock Wastes
	admin.Get("/stock-wastes", middleware.RequirePermission("stockwastes.view"), handlers.ListStockWastes)
	admin.Post("/stock-wastes", middleware.RequirePermission("stockwastes.create"), handlers.LegacyStockWasteDisabled)

	admin.Post("/stock-evidence", middleware.RequireAnyPermission("stockquality.central.report", "stockquality.central.approve", "stockquality.central.execute", "stockquality.central.receive", "stockquality.refund", "stockquality.outlet.report", "stockquality.outlet.approve", "stockquality.outlet.execute", "stockquality.outlet.receive", "stocktransfers.receive"), handlers.UploadStockEvidence)
	admin.Get("/stock-quality/lookups", middleware.RequirePermission("stockquality.view"), handlers.StockQualityLookups)
	admin.Get("/stock-quality/batches", middleware.RequireAnyPermission("stockquality.central.report", "stockquality.outlet.report"), handlers.StockQualityBatches)
	admin.Get("/stock-quality", middleware.RequirePermission("stockquality.view"), handlers.ListStockQuality)
	admin.Get("/stock-quality/:id", middleware.RequirePermission("stockquality.view"), handlers.GetStockQuality)
	admin.Post("/stock-quality", middleware.RequireAnyPermission("stockquality.central.report", "stockquality.outlet.report"), broadcastAfter(handlers.CreateStockQuality, "stock_quality", "stock_transfer"))
	admin.Post("/stock-quality/:id/actions", middleware.RequireAnyPermission("stockquality.central.approve", "stockquality.central.execute", "stockquality.central.receive", "stockquality.refund", "stockquality.outlet.approve", "stockquality.outlet.execute"), broadcastAfter(handlers.ActStockQuality, "stock_quality", "stock_transfer"))

	// Penerimaan Barang (Goods Receipt / GRN) — stok masuk ke gudang.
	// Baca: siapa pun pemirsa modul gudang; tulis: pemegang stockledger.adjust.
	admin.Get("/goods-receipts", middleware.RequireAnyPermission(whView...), handlers.ListGoodsReceipts)
	admin.Get("/goods-receipts/:id", middleware.RequireAnyPermission(whView...), handlers.GetGoodsReceipt)
	admin.Post("/goods-receipts", middleware.RequirePermission("stockledger.adjust"), broadcastAfter(handlers.CreateGoodsReceipt, "goods_receipt", "ppic_alert"))

	// PPIC — kontrol pemakaian: lookup minimal dan kebijakan terpisah dari mutasi Gudang/resep F&B.
	admin.Get("/ppic/usage-control/lookups", middleware.RequirePermission("ppic.control.view"), handlers.GetPpicUsageLookups)
	admin.Put("/ppic/usage-control/policy", middleware.RequirePermission("ppic.control.update"), broadcastAfter(handlers.SavePpicUsagePolicy, "ppic_alert"))
	admin.Get("/ppic/usage-control/policy/history", middleware.RequirePermission("ppic.control.view"), handlers.GetPpicUsagePolicyHistory)
	admin.Get("/ppic/usage-control/export", middleware.RequirePermission("ppic.control.export"), handlers.ExportPpicUsageControl)
	admin.Get("/ppic/usage-control", middleware.RequirePermission("ppic.control.view"), handlers.GetPpicUsageControl)

	// PPIC — lookup minimal, opname, dan pemantauan kedaluwarsa.
	admin.Get("/ppic/lookups", middleware.RequireAnyPermission("ppic.control.view", "ppic.opname.view", "ppic.expiry.view"), handlers.GetPpicUsageLookups)
	admin.Get("/ppic/expiry", middleware.RequirePermission("ppic.expiry.view"), handlers.GetExpiryMonitor)
	admin.Post("/ppic/expiry/:batchId/ack", middleware.RequirePermission("ppic.expiry.ack"), broadcastAfter(handlers.AckExpiryBatch, "ppic_alert"))
	admin.Get("/ppic/opnames", middleware.RequirePermission("ppic.opname.view"), handlers.ListStockOpnames)
	admin.Get("/ppic/opnames/:id", middleware.RequirePermission("ppic.opname.view"), handlers.GetStockOpname)
	admin.Post("/ppic/opnames", middleware.RequirePermission("ppic.opname.create"), broadcastAfter(handlers.CreateStockOpname, "ppic_alert"))
	admin.Put("/ppic/opnames/:id/items", middleware.RequirePermission("ppic.opname.create"), broadcastAfter(handlers.UpdateOpnameItems, "ppic_alert"))
	admin.Post("/ppic/opnames/:id/submit", middleware.RequirePermission("ppic.opname.create"), broadcastAfter(handlers.SubmitStockOpname, "ppic_alert"))
	admin.Post("/ppic/opnames/:id/approve", middleware.RequirePermission("ppic.opname.approve"), broadcastAfter(handlers.ApproveStockOpname, "ppic_alert"))
	admin.Post("/ppic/opnames/:id/cancel", middleware.RequirePermission("ppic.opname.create"), broadcastAfter(handlers.CancelStockOpname, "ppic_alert"))

	// Settings — view + update only (no create/delete for settings)
	// ── WhatsApp: koneksi gateway, notifikasi, penerima, log, broadcast ──
	admin.Get("/whatsapp/status", middleware.RequirePermission("whatsapp.view"), handlers.WAStatus)
	admin.Post("/whatsapp/login", middleware.RequirePermission("whatsapp.manage"), handlers.WALogin)
	admin.Post("/whatsapp/logout", middleware.RequirePermission("whatsapp.manage"), handlers.WALogout)
	admin.Get("/whatsapp/groups", middleware.RequirePermission("whatsapp.manage"), handlers.WAGroups)
	admin.Post("/whatsapp/test", middleware.RequirePermission("whatsapp.manage"), handlers.WASendTest)
	admin.Get("/whatsapp/settings", middleware.RequirePermission("whatsapp.view"), handlers.WAGetSettings)
	admin.Put("/whatsapp/settings", middleware.RequirePermission("whatsapp.manage"), handlers.WAUpdateSettings)
	admin.Post("/whatsapp/preview", middleware.RequirePermission("whatsapp.view"), handlers.WAPreviewTemplate)
	admin.Get("/whatsapp/recipients/users", middleware.RequirePermission("whatsapp.view"), handlers.WARecipientUsers)
	admin.Get("/whatsapp/roles", middleware.RequirePermission("whatsapp.view"), handlers.WARoleOptions)
	admin.Put("/whatsapp/roles/:role/notifications", middleware.RequirePermission("whatsapp.manage"), handlers.WAUpdateRoleNotifications)
	admin.Delete("/whatsapp/roles/:role/notifications", middleware.RequirePermission("whatsapp.manage"), handlers.WAResetRoleNotifications)
	admin.Get("/whatsapp/recipients", middleware.RequirePermission("whatsapp.view"), handlers.WAListRecipients)
	admin.Post("/whatsapp/recipients", middleware.RequirePermission("whatsapp.manage"), handlers.WACreateRecipient)
	admin.Put("/whatsapp/recipients/:id", middleware.RequirePermission("whatsapp.manage"), handlers.WAUpdateRecipient)
	admin.Delete("/whatsapp/recipients/:id", middleware.RequirePermission("whatsapp.manage"), handlers.WADeleteRecipient)
	admin.Get("/whatsapp/messages", middleware.RequirePermission("whatsapp.view"), handlers.WAListMessages)
	admin.Post("/whatsapp/messages/retry-failed", middleware.RequirePermission("whatsapp.manage"), handlers.WARetryFailed)
	admin.Post("/whatsapp/messages/:id/retry", middleware.RequirePermission("whatsapp.manage"), handlers.WARetryMessage)
	admin.Post("/whatsapp/messages/:id/cancel", middleware.RequirePermission("whatsapp.manage"), handlers.WACancelMessage)
	admin.Get("/whatsapp/broadcasts", middleware.RequirePermission("whatsapp.broadcast"), handlers.WAListBroadcasts)
	admin.Post("/whatsapp/broadcasts/preview", middleware.RequirePermission("whatsapp.broadcast"), handlers.WAPreviewBroadcast)
	admin.Post("/whatsapp/broadcasts", middleware.RequirePermission("whatsapp.broadcast"), handlers.WACreateBroadcast)
	admin.Get("/whatsapp/broadcasts/:id", middleware.RequirePermission("whatsapp.broadcast"), handlers.WAGetBroadcast)
	admin.Post("/whatsapp/broadcasts/:id/cancel", middleware.RequirePermission("whatsapp.broadcast"), handlers.WACancelBroadcast)

	admin.Get("/settings", middleware.RequirePermission("settings.company.view"), handlers.GetAllSettings)
	admin.Get("/settings/company", middleware.RequirePermission("settings.company.view"), handlers.GetCompanyIdentity)
	admin.Put("/settings/company", middleware.RequirePermission("settings.company.update"), handlers.UpdateCompanyIdentity)
	admin.Get("/settings/timezone", middleware.RequirePermission("settings.timezone.view"), handlers.GetTimezone)
	admin.Put("/settings/timezone", middleware.RequirePermission("settings.timezone.update"), handlers.UpdateTimezone)
	admin.Get("/settings/tax", middleware.RequirePermission("settings.tax.view"), handlers.GetTaxSettings)
	admin.Put("/settings/tax", middleware.RequirePermission("settings.tax.update"), handlers.UpdateTaxSettings)
}
