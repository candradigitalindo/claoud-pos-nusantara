package models

// PurchaseSubItem represents a single item inside a procurement entry.
type PurchaseSubItem struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
	// PurchasedQty is set by purchasing after approval. Qty remains the
	// approved request; nil keeps legacy requests at the requested quantity.
	PurchasedQty *int    `json:"purchased_qty,omitempty"`
	Unit         string  `json:"unit"`
	HpsPrice     float64 `json:"hps_price"`
	// HpsSource: dari mana angka HPS itu didapat — nama toko yang ditelepon,
	// tautan Shopee/Tokopedia, atau penawaran vendor. Tanpa ini, HPS hanya
	// angka yang harus dipercaya begitu saja saat pengajuan ditinjau.
	HpsSource   string  `json:"hps_source"`
	HpsSubtotal float64 `json:"hps_subtotal"`
	// FinalPrice/FinalSubtotal hanya diisi pembelian setelah pengajuan
	// disetujui; kiriman pengaju saat membuat atau mengubah pengajuan pending
	// diabaikan server (T-39). Subtotal selalu dihitung ulang server.
	FinalPrice    float64 `json:"final_price"`
	FinalSubtotal float64 `json:"final_subtotal"`
	// RabItemID menunjuk baris RAB projek yang dibelanjakan item ini. Kosong =
	// belanja di luar RAB (tetap dihitung ke projek, tetapi ditandai terpisah).
	RabItemID string `json:"rab_item_id,omitempty"`
	// GoodsKind: jenis baris yang DIBEKUKAN server saat pengajuan dibuat dan
	// setiap kali diubah selama pending (T-19): "dapur" (barang katalog stok →
	// diterima Gudang Induk ke stok) atau "perlengkapan" (peralatan → bagian
	// Aset: aset / habis pakai / material projek). Kosong untuk jasa dan data
	// lama. Nilai kiriman klien selalu ditimpa; meja penerimaan dan
	// pemeriksaan tujuan baris membaca nilai ini, bukan katalog yang sedang
	// berlaku — mengganti nama/menonaktifkan item katalog tidak lagi
	// memindahkan pengajuan terbuka antar meja.
	GoodsKind string `json:"goods_kind,omitempty"`
}

// PurchaseQuantity is the quantity to bill and receive, in the purchase unit.
func (s PurchaseSubItem) PurchaseQuantity() int {
	if s.PurchasedQty != nil {
		return *s.PurchasedQty
	}
	return s.Qty
}

// PurchaseRequestItem represents a procurement entry ("Nama Pengadaan") with aggregated totals.
type PurchaseRequestItem struct {
	Name       string            `json:"name"`
	HpsTotal   float64           `json:"hps_total"`
	FinalTotal float64           `json:"final_total"`
	Items      []PurchaseSubItem `json:"items"`
}

// RejectedPurchaseItem: sub-item yang ditolak verifikator (mis. barangnya
// masih tersedia di outlet lain). Baris ini DIPINDAH dari items ke kolom
// rejected_items, jadi tidak ikut total, persetujuan, maupun pembelian — tetapi
// jejaknya tetap terbaca di detail pengajuan.
type RejectedPurchaseItem struct {
	Group        string  `json:"group"` // nama pengadaan asal
	Name         string  `json:"name"`
	Qty          int     `json:"qty"`
	Unit         string  `json:"unit"`
	HpsPrice     float64 `json:"hps_price"`
	HpsSource    string  `json:"hps_source"`
	RabItemID    string  `json:"rab_item_id"`
	Reason       string  `json:"reason"`
	RejectedBy   string  `json:"rejected_by"`
	RejectedByID string  `json:"rejected_by_id"`
	RejectedAt   string  `json:"rejected_at"`
	Stage        string  `json:"stage"` // "verifikasi"
}

// PurchaseRequest represents a purchase request from an outlet.
type PurchaseRequest struct {
	ID            string  `json:"id"`
	RequestNumber string  `json:"request_number"`
	OutletID      *string `json:"outlet_id"`
	OutletName    *string `json:"outlet_name,omitempty"`
	WorkUnitID    *string `json:"work_unit_id"`
	WorkUnitName  string  `json:"work_unit_name,omitempty"`
	RequestType   string  `json:"request_type"`
	RequestedBy   string  `json:"requested_by"`
	// Pelaku tiap tahap diisi dari sesi login, bukan dari isi permintaan
	// (T-7). *_by_id NULL untuk token statis / data lama.
	RequestedByID *string               `json:"requested_by_id"`
	VendorID      *string               `json:"vendor_id"`
	VendorName    string                `json:"vendor_name"`
	Status        string                `json:"status"`
	Items         []PurchaseRequestItem `json:"items"`
	TotalAmount   float64               `json:"total_amount"`
	TotalHps      float64               `json:"total_hps"`
	TotalFinal    float64               `json:"total_final"`
	Notes         string                `json:"notes"`
	InvoiceNumber string                `json:"invoice_number"`
	InvoiceURL    string                `json:"invoice_url"`
	// Verifikasi (pending → verified): pemeriksa ketersediaan sebelum owner.
	VerifiedBy        string  `json:"verified_by"`
	VerifiedByID      *string `json:"verified_by_id"`
	VerifiedAt        *string `json:"verified_at"`
	VerificationNotes string  `json:"verification_notes"`
	ApprovedBy        *string `json:"approved_by"`
	ApprovedByID      *string `json:"approved_by_id"`
	ApprovedAt        *string `json:"approved_at"`
	RejectedReason    *string `json:"rejected_reason"`
	// Penolak (verifikator saat pending, penyetuju saat verified). Dulu
	// penolakan ditulis ke approved_by/approved_at (T-73).
	RejectedBy string  `json:"rejected_by"`
	RejectedAt *string `json:"rejected_at"`
	// RejectedItems: sub-item yang ditolak saat verifikasi. Selalu array.
	RejectedItems        []RejectedPurchaseItem `json:"rejected_items"`
	PaidBy               *string                `json:"paid_by"`
	PaidAt               *string                `json:"paid_at"`
	PaymentProof         *string                `json:"payment_proof"`
	PaymentAccountDest   string                 `json:"payment_account_dest"`
	PaymentAccountSource string                 `json:"payment_account_source"`
	PaymentNotes         string                 `json:"payment_notes"`
	PaidAmount           float64                `json:"paid_amount"`
	// CashPayout: tugas Kas Kasir Outlet terakhir yang masih relevan
	// (menunggu / diproses kasir / dikembalikan) — untuk halaman Pembayaran.
	CashPayout *ProcurementCashPayout `json:"cash_payout,omitempty"`
	// PayTo: tujuan pembayaran yang ditetapkan bagian Pembelian. Keuangan
	// hanya membacanya (lihat PurchasePayTo).
	PayTo PurchasePayTo `json:"pay_to"`
	ReceivedBy           *string                `json:"received_by"`
	ReceivedAt           *string                `json:"received_at"`
	// ReceiptStatus berdiri sendiri dari Status: '' (belum diterima), 'partial',
	// atau 'received'. Barang bisa datang sebelum maupun sesudah dibayar.
	ReceiptStatus string            `json:"receipt_status"`
	ProjectID     *string           `json:"project_id"`
	ProjectName   string            `json:"project_name,omitempty"`
	ParentID      *string           `json:"parent_id"`
	ParentNumber  string            `json:"parent_number,omitempty"`
	SplitStatus   *string           `json:"split_status"` // 'master' or nil
	Children      []PurchaseRequest `json:"children,omitempty"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
	// Version = updated_at (UTC, RFC3339Nano). Dikirim balik pada aksi verify
	// (wajib) dan approve/edit (opsional); bila dokumen sudah berubah server
	// membalas 409 "Pengajuan berubah, muat ulang".
	Version string `json:"version"`
	// Struks & Pickups hanya diisi pada detail (GetPurchaseRequest).
	Struks  []PurchaseStruk  `json:"struks,omitempty"`
	Pickups []PurchasePickup `json:"pickups,omitempty"`
	// Urgency: tingkat kebutuhan — rutin | prioritas | urgent. NeedByDate
	// (YYYY-MM-DD) wajib untuk prioritas & urgent.
	Urgency    string  `json:"urgency"`
	NeedByDate *string `json:"need_by_date"`
	// Emergency: hanya untuk Pembelian Darurat (beli dulu dari kas laci
	// outlet, disahkan penyetuju sesudahnya). nil untuk pengajuan biasa.
	Emergency *PurchaseEmergency `json:"emergency,omitempty"`
	// Pembayaran Manual & Pengajuan / Persetujuan Pembayaran (Mgr Keuangan / GM)
	IsManualPayment        bool    `json:"is_manual_payment"`
	PaymentApprovalStatus  string  `json:"payment_approval_status"`
	PaymentBatchNumber     string  `json:"payment_batch_number"`
	PaymentSubmittedBy     string  `json:"payment_submitted_by"`
	PaymentSubmittedAt     *string `json:"payment_submitted_at"`
	PaymentSubmissionNotes string  `json:"payment_submission_notes"`
	PaymentApprovedBy      string  `json:"payment_approved_by"`
	PaymentApprovedAt      *string `json:"payment_approved_at"`
	PaymentRejectedBy      string  `json:"payment_rejected_by"`
	PaymentRejectedAt      *string `json:"payment_rejected_at"`
	PaymentRejectionReason string  `json:"payment_rejection_reason"`
}

// ManualPaymentItemInput: satu baris item pada Pembayaran Manual (di luar pengajuan).
type ManualPaymentItemInput struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Unit  string  `json:"unit"`
	Price float64 `json:"price"`
}

// CreateManualPaymentInput: payload membuat Pembayaran Manual di halaman Pembayaran Pengadaan.
type CreateManualPaymentInput struct {
	RequestType          string                   `json:"request_type"` // barang | jasa | bahan
	GoodsKind            string                   `json:"goods_kind"`   // perlengkapan | dapur | ""
	WorkUnitID           string                   `json:"work_unit_id"`
	ProjectID            string                   `json:"project_id"`
	Title                string                   `json:"title"`
	VendorID             string                   `json:"vendor_id"`
	VendorName           string                   `json:"vendor_name"`
	PayToMethod          string                   `json:"pay_to_method"` // vendor_account | marketplace_va | manual
	PayToBankName        string                   `json:"pay_to_bank_name"`
	PayToAccountNumber   string                   `json:"pay_to_account_number"`
	PayToAccountHolder   string                   `json:"pay_to_account_holder"`
	PayToMarketplace     string                   `json:"pay_to_marketplace"`
	PayToOrderNumber     string                   `json:"pay_to_order_number"`
	PayToText            string                   `json:"pay_to_text"`
	PayToNote            string                   `json:"pay_to_note"`
	InvoiceNumber        string                   `json:"invoice_number"`
	InvoiceURL           string                   `json:"invoice_url"`
	Notes                string                   `json:"notes"`
	Items                []ManualPaymentItemInput `json:"items"`
	ActorName            string                   `json:"-"`
	ActorID              string                   `json:"-"`
}

// SubmitPaymentApprovalInput: daftar tagihan menunggu pembayaran yang diceklis
// untuk dibuatkan pengajuan pembayaran ke Manager Keuangan / General Manager.
type SubmitPaymentApprovalInput struct {
	IDs       []string `json:"ids"`
	Notes     string   `json:"notes"`
	ActorName string   `json:"-"`
	ActorID   string   `json:"-"`
}

// DecidePaymentApprovalInput: persetujuan atau penolakan pengajuan pembayaran
// oleh Manager Keuangan / General Manager.
type DecidePaymentApprovalInput struct {
	IDs       []string `json:"ids"`
	Decision  string   `json:"decision"` // approve | reject
	Reason    string   `json:"reason"`
	ActorName string   `json:"-"`
	ActorID   string   `json:"-"`
}

// PurchaseEmergency: data Pembelian Darurat — alasan, penerima kas, bukti
// belanja yang diunggah sesudahnya, dan pengesahan penyetuju.
type PurchaseEmergency struct {
	Category      string `json:"category"`
	CategoryLabel string `json:"category_label"`
	Reason        string `json:"reason"`
	// Recipient: staf yang menerima kas dari laci untuk berbelanja.
	Recipient string `json:"recipient"`
	// Bukti belanja: foto struk + total yang tertera di struk.
	ProofURL   string   `json:"proof_url"`
	ProofTotal *float64 `json:"proof_total"`
	ProofNotes string   `json:"proof_notes"`
	ProofBy    string   `json:"proof_by"`
	ProofAt    *string  `json:"proof_at"`
	// Ratification: pending | ratified | rejected.
	Ratification      string  `json:"ratification"`
	RatifiedBy        string  `json:"ratified_by"`
	RatifiedAt        *string `json:"ratified_at"`
	RatificationNotes string  `json:"ratification_notes"`
}

// EmergencyPurchaseItemInput: satu barang/jasa yang dibeli darurat. Price =
// harga satuan yang sudah dikonfirmasi ke toko/teknisi (uang yang dibawa).
type EmergencyPurchaseItemInput struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Unit  string  `json:"unit"`
	Price float64 `json:"price"`
}

// CreateEmergencyPurchaseInput: Pembelian Darurat dari manajer outlet.
type CreateEmergencyPurchaseInput struct {
	WorkUnitID    string                       `json:"work_unit_id"`
	RequestType   string                       `json:"request_type"` // barang | jasa
	Category      string                       `json:"category"`
	Reason        string                       `json:"reason"`
	ShopName      string                       `json:"shop_name"`
	RecipientName string                       `json:"recipient_name"`
	Notes         string                       `json:"notes"`
	Items         []EmergencyPurchaseItemInput `json:"items"`
	// Diisi handler dari sesi login.
	RequestedBy   string `json:"-"`
	RequestedByID string `json:"-"`
}

// EmergencyProofInput: bukti belanja Pembelian Darurat (foto struk wajib).
type EmergencyProofInput struct {
	ProofURL   string  `json:"proof_url"`
	ProofTotal float64 `json:"proof_total"`
	Notes      string  `json:"notes"`
	Version    string  `json:"version"`
	ActorName  string  `json:"-"`
}

// EmergencyDecisionInput: pengesahan (ratify), penolakan (reject), atau
// pembatalan/kirim ulang oleh pemilik pengajuan.
type EmergencyDecisionInput struct {
	Decision  string `json:"decision"` // ratify | reject (aksi ratify)
	Notes     string `json:"notes"`
	Version   string `json:"version"`
	ActorName string `json:"-"`
	ActorID   string `json:"-"`
}

// EmergencySettings: batas Pembelian Darurat. MonthlyCap 0 = tanpa plafon.
type EmergencySettings struct {
	MaxAmount  float64 `json:"max_amount"`
	MonthlyCap float64 `json:"monthly_cap"`
}

// EmergencyRecapRow: satu Pembelian Darurat di rekap audit Keuangan.
type EmergencyRecapRow struct {
	ID            string  `json:"id"`
	RequestNumber string  `json:"request_number"`
	CreatedAt     string  `json:"created_at"`
	OutletID      string  `json:"outlet_id"`
	OutletName    string  `json:"outlet_name"`
	WorkUnitName  string  `json:"work_unit_name"`
	RequestType   string  `json:"request_type"`
	Items         string  `json:"items"`
	CategoryLabel string  `json:"category_label"`
	Reason        string  `json:"reason"`
	RequestedBy   string  `json:"requested_by"`
	Recipient     string  `json:"recipient"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	// Stage: waiting_cash | waiting_proof | waiting_ratification | ratified |
	// rejected | cancelled.
	Stage        string   `json:"stage"`
	PaidAmount   float64  `json:"paid_amount"`
	PaidAt       *string  `json:"paid_at"`
	Cashier      string   `json:"cashier"`
	AuthorizedBy string   `json:"authorized_by"`
	ProofURL     string   `json:"proof_url"`
	ProofTotal   *float64 `json:"proof_total"`
	// Difference = kas diserahkan − total struk (positif: sisa harus kembali
	// ke laci). nil bila belum ada struk atau kas belum diserahkan.
	Difference        *float64 `json:"difference"`
	RatifiedBy        string   `json:"ratified_by"`
	RatifiedAt        *string  `json:"ratified_at"`
	RatificationNotes string   `json:"ratification_notes"`
	CancelReason      string   `json:"cancel_reason"`
}

// EmergencyRecapBucket: jumlah & nilai satu kelompok.
type EmergencyRecapBucket struct {
	Count  int     `json:"count"`
	Amount float64 `json:"amount"`
}

// EmergencyRecapOutlet: ringkasan satu outlet (atau total semua outlet).
type EmergencyRecapOutlet struct {
	OutletID   string `json:"outlet_id"`
	OutletName string `json:"outlet_name"`
	// Recorded: Pembelian Darurat yang tidak dibatalkan.
	Recorded            EmergencyRecapBucket `json:"recorded"`
	WaitingCash         EmergencyRecapBucket `json:"waiting_cash"`
	WaitingProof        EmergencyRecapBucket `json:"waiting_proof"`
	WaitingRatification EmergencyRecapBucket `json:"waiting_ratification"`
	Ratified            EmergencyRecapBucket `json:"ratified"`
	Rejected            EmergencyRecapBucket `json:"rejected"`
	Cancelled           EmergencyRecapBucket `json:"cancelled"`
	ProofDifference     float64              `json:"proof_difference"`
	// OtherCashOut: Kas Keluar tablet yang BUKAN tugas Bayar Pengadaan
	// (gaji harian, BBM, refund, belanja di luar sistem).
	OtherCashOut EmergencyRecapBucket `json:"other_cash_out"`
}

// EmergencyRecap: rekap bulanan Pembelian Darurat untuk audit Keuangan.
type EmergencyRecap struct {
	Month      string                 `json:"month"` // YYYY-MM (zona aplikasi)
	MonthLabel string                 `json:"month_label"`
	Settings   EmergencySettings      `json:"settings"`
	Totals     EmergencyRecapOutlet   `json:"totals"`
	Outlets    []EmergencyRecapOutlet `json:"outlets"`
	Rows       []EmergencyRecapRow    `json:"rows"`
}

// EmergencyContext: bekal form Pembelian Darurat untuk satu unit kerja.
type EmergencyContext struct {
	EmergencySettings
	OutletID   string `json:"outlet_id"`
	OutletName string `json:"outlet_name"`
	// UsedThisMonth: nilai Pembelian Darurat outlet ini bulan berjalan
	// (yang tidak dibatalkan). Remaining nil bila tanpa plafon.
	UsedThisMonth  float64  `json:"used_this_month"`
	CountThisMonth int      `json:"count_this_month"`
	Remaining      *float64 `json:"remaining"`
	// TabletReady: tablet kasir outlet sudah memakai versi App POS yang
	// punya menu Bayar Pengadaan (tugas kas muncul di sana).
	TabletReady   bool   `json:"tablet_ready"`
	TabletVersion string `json:"tablet_version"`
	TabletNote    string `json:"tablet_note"`
}

// PurchaseStruk: satu struk/nota pembelian dari vendor. Satu pengajuan bisa
// dibeli dalam beberapa transaksi (mis. struk A tanggal 1 senilai 19 jt,
// struk B senilai 20 jt) dan dibayar per struk — PaidAmount/Remaining
// dihitung dari pembayaran yang menunjuk struk ini.
type PurchaseStruk struct {
	ID          string  `json:"id"`
	StrukNumber string  `json:"struk_number"`
	StrukDate   string  `json:"struk_date"` // YYYY-MM-DD
	Total       float64 `json:"total"`
	PaidAmount  float64 `json:"paid_amount"`
	Remaining   float64 `json:"remaining"`
	PhotoURL    string  `json:"photo_url"`
	Notes       string  `json:"notes"`
	CreatedBy   string  `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
}

// AddPurchaseStrukInput: struk baru dari bagian pembelian / petugas logistik.
type AddPurchaseStrukInput struct {
	StrukNumber string  `json:"struk_number"`
	StrukDate   string  `json:"struk_date"`
	Total       float64 `json:"total"`
	PhotoURL    string  `json:"photo_url"`
	Notes       string  `json:"notes"`
}

// PurchasePickup: pengambilan barang dari vendor oleh petugas logistik.
// ReceivedAt kosong = masih dalam pengantaran ke tempat penerimaan.
type PurchasePickup struct {
	ID         string `json:"id"`
	PickedBy   string `json:"picked_by"`
	PickedAt   string `json:"picked_at"`
	PhotoURL   string `json:"photo_url"`
	Notes      string `json:"notes"`
	ReceivedBy string `json:"received_by"`
	ReceivedAt string `json:"received_at,omitempty"`
}

// AddPurchasePickupInput: catatan pengambilan (foto wajib).
type AddPurchasePickupInput struct {
	PhotoURL string `json:"photo_url"`
	Notes    string `json:"notes"`
}

// SplitPurchaseRequestInput is the payload for splitting a PR into a new vendor request.
type SplitPurchaseRequestInput struct {
	VendorID   string                `json:"vendor_id" validate:"required"`
	VendorName string                `json:"vendor_name" validate:"required"`
	Items      []PurchaseRequestItem `json:"items" validate:"required"`
}

// CreatePurchaseRequestInput is the payload for creating a purchase request.
type CreatePurchaseRequestInput struct {
	OutletID    string `json:"outlet_id"`
	WorkUnitID  string `json:"work_unit_id"`
	RequestType string `json:"request_type" validate:"required"`
	// RequestedBy/RequestedByID selalu ditimpa handler dari sesi login;
	// nilai requested_by di body diabaikan (T-7).
	RequestedBy   string `json:"requested_by"`
	RequestedByID string `json:"-"`
	VendorID      string `json:"vendor_id"`
	VendorName    string `json:"vendor_name"`
	// ProjectID opsional: mengikat pengajuan ini sebagai satu belanja tahap
	// dari sebuah projek pembangunan/renovasi.
	ProjectID string                `json:"project_id"`
	Items     []PurchaseRequestItem `json:"items" validate:"required"`
	Notes     string                `json:"notes"`
	// Urgency: rutin (bawaan) | prioritas | urgent. NeedByDate YYYY-MM-DD,
	// wajib untuk prioritas & urgent.
	Urgency    string `json:"urgency"`
	NeedByDate string `json:"need_by_date"`
}

// UpdatePurchaseStatusInput is the payload for advancing purchase request status.
type UpdatePurchaseStatusInput struct {
	Action string `json:"action" validate:"required"` // verify, approve, reject, request_payment, pay, receive, cancel
	// ActorName boleh tetap dikirim UI lama, tetapi selalu DITIMPA handler
	// dengan nama akun yang login (T-7). ActorID tidak bisa diisi dari body.
	ActorName            string  `json:"actor_name"`
	ActorID              string  `json:"-"`
	RejectedReason       string  `json:"rejected_reason,omitempty"`
	PaymentProof         string  `json:"payment_proof,omitempty"`
	PaymentAccountDest   string  `json:"payment_account_dest,omitempty"`
	PaymentAccountSource string  `json:"payment_account_source,omitempty"`
	PaymentNotes         string  `json:"payment_notes,omitempty"`
	PaymentAmount        float64 `json:"payment_amount,omitempty"`
	BankFee              float64 `json:"bank_fee,omitempty"`
	// Marketplace virtual accounts belong to this payment/order, never the
	// vendor master. An omitted method keeps the registered-account behavior.
	PaymentMethod        string `json:"payment_method,omitempty"`
	PaymentMarketplace   string `json:"payment_marketplace,omitempty"`
	PaymentOrderNumber   string `json:"payment_order_number,omitempty"`
	PaymentBankName      string `json:"payment_bank_name,omitempty"`
	PaymentAccountNumber string `json:"payment_account_number,omitempty"`
	PaymentAccountHolder string `json:"payment_account_holder,omitempty"`
	// Version = PurchaseRequest.version dari GET. Wajib untuk verify; untuk
	// aksi lain diperiksa bila dikirim. Beda → 409.
	Version string `json:"version,omitempty"`
	// Notes: catatan verifikator (aksi verify).
	Notes string `json:"notes,omitempty"`
	// RejectedItems: sub-item yang ditolak verifikator (aksi verify).
	RejectedItems []VerifyRejectedItemInput `json:"rejected_items,omitempty"`
	// StrukID (aksi pay, opsional): struk yang dilunasi pembayaran ini. Bila
	// kosong pada pengajuan yang punya struk, server membagi pembayaran ke
	// struk yang belum lunas, terlama dulu.
	StrukID string `json:"struk_id,omitempty"`
	// ExpectedStatus: status yang dipakai handler saat memeriksa izin. Bila
	// dokumen sudah pindah status sebelum dikunci, aksi ditolak 409 — izin
	// tidak boleh diputus atas tahap yang sudah lewat.
	ExpectedStatus string `json:"-"`
	// CashPayoutID (internal, aksi pay): pembayaran ini berasal dari tugas
	// Kas Kasir Outlet yang sudah diproses tablet. Tidak bisa diisi dari body.
	CashPayoutID string `json:"-"`
	// PayTo (aksi request_payment): tujuan pembayaran dari bagian Pembelian.
	// Kosong = rekening vendor terdaftar (bila vendor punya rekening).
	PayTo *PurchasePaymentDetailsInput `json:"pay_to,omitempty"`
	// DirectCashPayout (aksi approve): penyetuju langsung mengarahkan pembayaran
	// ke Kas Kasir Outlet (menggunakan HPS sebagai harga final) tanpa melalui
	// tahap Purchasing.
	DirectCashPayout bool   `json:"direct_cash_payout,omitempty"`
	PayoutRecipient  string `json:"payout_recipient,omitempty"`
	PayoutNotes      string `json:"payout_notes,omitempty"`
}

// PurchasePayTo: tujuan pembayaran sebuah pengajuan. Diisi bagian Pembelian
// saat mengajukan pembayaran dan boleh diubah selama tagihan belum lunas;
// Keuangan hanya membaca, lalu memilih sumber dana.
//
// Method = isian Pembelian ('' bila belum pernah diisi — data lama).
// Effective = metode yang berlaku sekarang: data lama tanpa isian memakai
// rekening vendor terdaftar bila ada. Untuk vendor_account, BankName,
// AccountNumber, dan AccountHolder adalah rekening vendor SAAT INI (dipotret
// ke riwayat saat dibayar). Ready=false → belum bisa dibayar; Problem
// menjelaskan sebabnya (dikembalikan, belum diisi, rekening kosong).
type PurchasePayTo struct {
	Method        string  `json:"method"`
	Effective     string  `json:"effective"`
	BankName      string  `json:"bank_name"`
	AccountNumber string  `json:"account_number"`
	AccountHolder string  `json:"account_holder"`
	Marketplace   string  `json:"marketplace"`
	OrderNumber   string  `json:"order_number"`
	Text          string  `json:"text"`
	Note          string  `json:"note"`
	SetBy         string  `json:"set_by"`
	SetAt         *string `json:"set_at"`
	ReturnedAt    *string `json:"returned_at"`
	ReturnedBy    string  `json:"returned_by"`
	ReturnReason  string  `json:"return_reason"`
	Ready         bool    `json:"ready"`
	Problem       string  `json:"problem"`
	// Rekening terdaftar vendor saat ini — acuan form Pembelian (opsi
	// "Rekening Vendor Terdaftar" hanya bila nomornya ada).
	VendorBankName      string `json:"vendor_bank_name"`
	VendorAccountNumber string `json:"vendor_account_number"`
	VendorAccountHolder string `json:"vendor_account_holder"`
}

// PurchasePaymentDetailsInput: tujuan pembayaran dari bagian Pembelian.
// Method: vendor_account (rekening vendor terdaftar — tidak bisa diketik),
// marketplace_va (VA dari checkout), manual (tunai / cara bayar lain — hanya
// untuk vendor yang belum punya rekening terdaftar).
type PurchasePaymentDetailsInput struct {
	Method        string `json:"method"`
	Marketplace   string `json:"marketplace"`
	OrderNumber   string `json:"order_number"`
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
	Text          string `json:"text"`
	Note          string `json:"note"`
	// Version wajib saat mengubah detail (PUT payment-details); diabaikan
	// di dalam aksi request_payment (versi dibawa input status).
	Version   string `json:"version,omitempty"`
	ActorName string `json:"-"`
	ActorID   string `json:"-"`
}

// PurchasePaymentReturnInput: Keuangan mengembalikan detail pembayaran yang
// salah/kurang ke bagian Pembelian.
type PurchasePaymentReturnInput struct {
	Reason    string `json:"reason"`
	Version   string `json:"version"`
	ActorName string `json:"-"`
	ActorID   string `json:"-"`
}

// VerifyRejectedItemInput menunjuk satu sub-item menurut isi tersimpan:
// Group = indeks di items (Nama Pengadaan), Item = indeks sub-item di group itu.
type VerifyRejectedItemInput struct {
	Group  int    `json:"group"`
	Item   int    `json:"item"`
	Reason string `json:"reason"`
}

// UpdatePurchaseItemsInput is the payload for updating items (e.g. setting final prices).
//
// Vendor dan nomor invoice sengaja berupa pointer: field yang tidak dikirim
// (nil) berarti "biarkan apa adanya", sedangkan string kosong berarti pengguna
// memang mengosongkannya. Sebelumnya bertipe string biasa, sehingga pengaju
// yang sekadar mengubah qty diam-diam menghapus vendor dan nomor invoice.
type UpdatePurchaseItemsInput struct {
	Items      []PurchaseRequestItem `json:"items" validate:"required"`
	VendorID   *string               `json:"vendor_id,omitempty"`
	VendorName *string               `json:"vendor_name,omitempty"`
	// InvoiceNumber & InvoiceURL hanya berlaku pada pengajuan yang sudah disetujui (diisi
	// pembelian); pada pending diabaikan (T-39).
	InvoiceNumber *string `json:"invoice_number,omitempty"`
	InvoiceURL    *string `json:"invoice_url,omitempty"`
	// Version opsional (lihat UpdatePurchaseStatusInput.Version).
	Version string `json:"version,omitempty"`
	// ActorName: nama akun yang login — diisi handler, bukan body — untuk
	// notifikasi "harga & vendor ditetapkan".
	ActorName string `json:"-"`
	// ExpectedStatus: status yang dipakai handler memilih izin (pending →
	// submit, approved → purchasing). Beda saat dikunci → 409.
	ExpectedStatus string `json:"-"`
}

// PurchaseRequestListResponse wraps paginated purchase requests.
type PurchaseRequestListResponse struct {
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	Total      int               `json:"total"`
	TotalPages int               `json:"total_pages"`
	Requests   []PurchaseRequest `json:"requests"`
}

// PaymentHistory represents a single payment entry for a purchase request.
//
// Vendor and payment destination are snapshotted for each payment. A marketplace
// VA belongs to that order, while standard transfers use the vendor's registered
// account. DestManual marks payer-entered destinations, including marketplace VAs.
type PaymentHistory struct {
	ID                   string  `json:"id"`
	PurchaseRequestID    string  `json:"purchase_request_id"`
	Amount               float64 `json:"amount"`
	BankFee              float64 `json:"bank_fee"`
	PaymentProof         string  `json:"payment_proof"`
	PaymentAccountDest   string  `json:"payment_account_dest"`
	PaymentAccountSource string  `json:"payment_account_source"`
	PaymentNotes         string  `json:"payment_notes"`
	PaidBy               string  `json:"paid_by"`
	CreatedAt            string  `json:"created_at"`
	VendorID             string  `json:"vendor_id"`
	VendorName           string  `json:"vendor_name"`
	DestBankName         string  `json:"dest_bank_name"`
	DestAccountNumber    string  `json:"dest_account_number"`
	DestAccountHolder    string  `json:"dest_account_holder"`
	DestManual           bool    `json:"dest_manual"`
	PaymentMethod        string  `json:"payment_method"`
	PaymentMarketplace   string  `json:"payment_marketplace"`
	PaymentOrderNumber   string  `json:"payment_order_number"`
	// Struk yang dilunasi pembayaran ini (kosong = tanpa struk).
	StrukID     string `json:"struk_id"`
	StrukNumber string `json:"struk_number"`
}
