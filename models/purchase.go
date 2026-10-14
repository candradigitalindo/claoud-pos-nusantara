package models

// PurchaseSubItem represents a single item inside a procurement entry.
type PurchaseSubItem struct {
	Name     string  `json:"name"`
	Qty      int     `json:"qty"`
	Unit     string  `json:"unit"`
	HpsPrice float64 `json:"hps_price"`
	// HpsSource: dari mana angka HPS itu didapat — nama toko yang ditelepon,
	// tautan Shopee/Tokopedia, atau penawaran vendor. Tanpa ini, HPS hanya
	// angka yang harus dipercaya begitu saja saat pengajuan ditinjau.
	HpsSource     string  `json:"hps_source"`
	HpsSubtotal   float64 `json:"hps_subtotal"`
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
	ID                   string                `json:"id"`
	RequestNumber        string                `json:"request_number"`
	OutletID             *string               `json:"outlet_id"`
	OutletName           *string               `json:"outlet_name,omitempty"`
	WorkUnitID           *string               `json:"work_unit_id"`
	WorkUnitName         string                `json:"work_unit_name,omitempty"`
	RequestType          string                `json:"request_type"`
	RequestedBy          string                `json:"requested_by"`
	// Pelaku tiap tahap diisi dari sesi login, bukan dari isi permintaan
	// (T-7). *_by_id NULL untuk token statis / data lama.
	RequestedByID        *string               `json:"requested_by_id"`
	VendorID             *string               `json:"vendor_id"`
	VendorName           string                `json:"vendor_name"`
	Status               string                `json:"status"`
	Items                []PurchaseRequestItem `json:"items"`
	TotalAmount          float64               `json:"total_amount"`
	TotalHps             float64               `json:"total_hps"`
	TotalFinal           float64               `json:"total_final"`
	Notes                string                `json:"notes"`
	InvoiceNumber        string                `json:"invoice_number"`
	// Verifikasi (pending → verified): pemeriksa ketersediaan sebelum owner.
	VerifiedBy           string                `json:"verified_by"`
	VerifiedByID         *string               `json:"verified_by_id"`
	VerifiedAt           *string               `json:"verified_at"`
	VerificationNotes    string                `json:"verification_notes"`
	ApprovedBy           *string               `json:"approved_by"`
	ApprovedByID         *string               `json:"approved_by_id"`
	ApprovedAt           *string               `json:"approved_at"`
	RejectedReason       *string               `json:"rejected_reason"`
	// Penolak (verifikator saat pending, penyetuju saat verified). Dulu
	// penolakan ditulis ke approved_by/approved_at (T-73).
	RejectedBy           string                `json:"rejected_by"`
	RejectedAt           *string               `json:"rejected_at"`
	// RejectedItems: sub-item yang ditolak saat verifikasi. Selalu array.
	RejectedItems        []RejectedPurchaseItem `json:"rejected_items"`
	PaidBy               *string               `json:"paid_by"`
	PaidAt               *string               `json:"paid_at"`
	PaymentProof         *string               `json:"payment_proof"`
	PaymentAccountDest   string                `json:"payment_account_dest"`
	PaymentAccountSource string                `json:"payment_account_source"`
	PaymentNotes         string                `json:"payment_notes"`
	PaidAmount           float64               `json:"paid_amount"`
	ReceivedBy           *string               `json:"received_by"`
	ReceivedAt           *string               `json:"received_at"`
	// ReceiptStatus berdiri sendiri dari Status: '' (belum diterima), 'partial',
	// atau 'received'. Barang bisa datang sebelum maupun sesudah dibayar.
	ReceiptStatus string `json:"receipt_status"`
	ProjectID            *string               `json:"project_id"`
	ProjectName          string                `json:"project_name,omitempty"`
	ParentID             *string               `json:"parent_id"`
	ParentNumber         string                `json:"parent_number,omitempty"`
	SplitStatus          *string               `json:"split_status"` // 'master' or nil
	Children             []PurchaseRequest     `json:"children,omitempty"`
	CreatedAt            string                `json:"created_at"`
	UpdatedAt            string                `json:"updated_at"`
	// Version = updated_at (UTC, RFC3339Nano). Dikirim balik pada aksi verify
	// (wajib) dan approve/edit (opsional); bila dokumen sudah berubah server
	// membalas 409 "Pengajuan berubah, muat ulang".
	Version              string                `json:"version"`
	// Struks & Pickups hanya diisi pada detail (GetPurchaseRequest).
	Struks  []PurchaseStruk  `json:"struks,omitempty"`
	Pickups []PurchasePickup `json:"pickups,omitempty"`
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
	OutletID    string                `json:"outlet_id"`
	WorkUnitID  string                `json:"work_unit_id"`
	RequestType string                `json:"request_type" validate:"required"`
	// RequestedBy/RequestedByID selalu ditimpa handler dari sesi login;
	// nilai requested_by di body diabaikan (T-7).
	RequestedBy   string `json:"requested_by"`
	RequestedByID string `json:"-"`
	VendorID    string                `json:"vendor_id"`
	VendorName  string                `json:"vendor_name"`
	// ProjectID opsional: mengikat pengajuan ini sebagai satu belanja tahap
	// dari sebuah projek pembangunan/renovasi.
	ProjectID string                `json:"project_id"`
	Items     []PurchaseRequestItem `json:"items" validate:"required"`
	Notes       string                `json:"notes"`
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
	Items         []PurchaseRequestItem `json:"items" validate:"required"`
	VendorID      *string               `json:"vendor_id,omitempty"`
	VendorName    *string               `json:"vendor_name,omitempty"`
	// InvoiceNumber hanya berlaku pada pengajuan yang sudah disetujui (diisi
	// pembelian); pada pending diabaikan (T-39).
	InvoiceNumber *string               `json:"invoice_number,omitempty"`
	// Version opsional (lihat UpdatePurchaseStatusInput.Version).
	Version string `json:"version,omitempty"`
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
// Vendor & rekening tujuan dipotret server saat pembayaran dicatat, dari data
// vendor saat itu — transaksi lama tidak ikut berubah bila vendor kemudian
// mengganti rekeningnya. DestManual = rekening/cara bayar diketik pembayar
// karena vendornya tidak punya rekening terdaftar (mis. Tunai).
type PaymentHistory struct {
	ID                   string  `json:"id"`
	PurchaseRequestID    string  `json:"purchase_request_id"`
	Amount               float64 `json:"amount"`
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
	// Struk yang dilunasi pembayaran ini (kosong = tanpa struk).
	StrukID     string `json:"struk_id"`
	StrukNumber string `json:"struk_number"`
}
