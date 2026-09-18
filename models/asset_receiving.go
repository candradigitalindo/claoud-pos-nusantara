package models

// Penerimaan barang dari Pengadaan (docs/perlengkapan-aset.md §8).
//
// Sebelum ini, aksi "Serah Terima" hanya mengubah status pengajuan menjadi
// 'received'. AC dan kulkas yang baru dibeli tidak pernah menjadi baris aset —
// petugas harus mengetik ulang seluruhnya, atau tidak mengetiknya sama sekali.

// ReceivingLine adalah satu sub-item pengajuan beserta usulan tujuannya.
type ReceivingLine struct {
	PRItemKey   string  `json:"pr_item_key"`
	// SourcePRID = dokumen yang MEMEGANG item. Untuk pengajuan yang dipecah per
	// vendor, ini id anaknya — bukan masternya yang tinggal cangkang.
	SourcePRID  string  `json:"source_pr_id"`
	EntryName   string  `json:"entry_name"`
	Name        string  `json:"name"`
	Qty         int     `json:"qty"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	Subtotal    float64 `json:"subtotal"`
	// Destination: aset | material | stok | habis
	Destination string `json:"destination"`
	// Kind memisahkan tanggung jawab di layar: "perlengkapan" adalah urusan tim
	// aset, "dapur" urusan gudang (barang yang sudah ada di katalog stok),
	// "habis" barang pakai-buang. Tim aset tidak menerima data barang dapur.
	Kind string `json:"kind"`
	// Recorded = qty yang sudah pernah dicatat (aset + GRN) untuk baris ini,
	// Remaining = sisanya. Inilah dasar penerimaan bertahap dan menu
	// "Lengkapi Penerimaan".
	Recorded  int `json:"recorded"`
	Remaining int `json:"remaining"`
	// SuggestedStockItemID terisi bila ada item stok bernama mirip.
	SuggestedStockItemID   string `json:"suggested_stock_item_id,omitempty"`
	SuggestedStockItemName string `json:"suggested_stock_item_name,omitempty"`
}

// ReceivingDraft adalah isi dialog Serah Terima.
type ReceivingDraft struct {
	PurchaseRequestID string          `json:"purchase_request_id"`
	RequestNumber     string          `json:"request_number"`
	Status            string          `json:"status"`
	OutletID          string          `json:"outlet_id"`
	OutletName        string          `json:"outlet_name"`
	ProjectID         string          `json:"project_id,omitempty"`
	VendorName        string          `json:"vendor_name"`
	InvoiceNumber     string          `json:"invoice_number"`
	CanReceive        bool            `json:"can_receive"`
	// PaidAmount & TotalFinal dipakai layar untuk memberi tahu petugas bahwa
	// barang ini diterima SEBELUM dibayar — bukan untuk menghalangi.
	PaidAmount    float64 `json:"paid_amount"`
	TotalFinal    float64 `json:"total_final"`
	AlreadyReceived   bool            `json:"already_received"`
	CapitalizationMin float64         `json:"capitalization_min"`
	// TargetWarehouse* = gudang yang MEMBUTUHKAN barang (gudang run MRP, atau
	// gudang outlet pengaju) — ditampilkan supaya penerima tahu barang ini
	// diteruskan ke mana. ReceivingWarehouseID = gudang induk, bawaan pilihan
	// untuk baris stok: barang dapur diterima di sana dulu, lalu diteruskan
	// lewat Transfer Stok yang berfoto.
	TargetWarehouseID    string `json:"target_warehouse_id,omitempty"`
	TargetWarehouseName  string `json:"target_warehouse_name,omitempty"`
	TargetWarehouseType  string `json:"target_warehouse_type,omitempty"`
	ReceivingWarehouseID string `json:"receiving_warehouse_id,omitempty"`
	Lines             []ReceivingLine `json:"lines"`
}

// ReceiveGoodsLine adalah keputusan petugas atas satu baris.
type ReceiveGoodsLine struct {
	PRItemKey   string `json:"pr_item_key"`
	Destination string `json:"destination"` // aset | material | stok | habis
	Qty         int    `json:"qty"`
	Reason      string `json:"reason"` // wajib untuk 'habis' bernilai besar

	// Tujuan aset
	AssetName        string  `json:"asset_name"`
	Category         string  `json:"category"`
	TrackingMode     string  `json:"tracking_mode"`
	SerialNumber     string  `json:"serial_number"`
	Brand            string  `json:"brand"`
	Model            string  `json:"model"`
	Location         string  `json:"location"`
	UsefulLifeMonths int     `json:"useful_life_months"`
	ResidualValue    float64 `json:"residual_value"`
	UnitPrice        float64 `json:"unit_price"`

	// Tujuan stok gudang
	StockItemID string `json:"stock_item_id"`
	WarehouseID string `json:"warehouse_id"`
	ExpiryDate  string `json:"expiry_date"`
}

type ReceiveGoodsRequest struct {
	OutletID string             `json:"outlet_id"` // bila pengajuan tidak terikat outlet
	Notes    string             `json:"notes"`
	// PhotoURL adalah bukti barang diterima dari tim purchasing. Wajib: tanpa
	// foto, "barang sudah saya serahkan" dan "saya belum menerima apa pun"
	// tidak bisa dibedakan saat keduanya saling mengklaim.
	PhotoURL string `json:"photo_url"`
	Desk     string `json:"desk"` // perlengkapan | dapur — meja yang menerima
	Lines    []ReceiveGoodsLine `json:"lines"`
}

// ReceiveGoodsResult merangkum apa yang benar-benar terbentuk.
type ReceiveGoodsResult struct {
	PurchaseRequestID string   `json:"purchase_request_id"`
	Status            string   `json:"status"`
	AssetsCreated     int      `json:"assets_created"`
	AssetIDs          []string `json:"asset_ids"`
	GRNNumber         string   `json:"grn_number,omitempty"`
	StockLinesQueued  int      `json:"stock_lines_queued"`
	MaterialLines     int      `json:"material_lines"`
	ReceiptStatus     string   `json:"receipt_status"`
	// OutstandingLines = baris yang masih menunggu meja lain (mis. bagian aset
	// sudah mencatat peralatan, gudang belum menerima barang dapurnya).
	OutstandingLines int `json:"outstanding_lines"`
	SkippedLines      int      `json:"skipped_lines"`
	Message           string   `json:"message"`
}
