package models

// Penerimaan barang dari Pengadaan (docs/perlengkapan-aset.md §8).
//
// Sebelum ini, aksi "Serah Terima" hanya mengubah status pengajuan menjadi
// 'received'. AC dan kulkas yang baru dibeli tidak pernah menjadi baris aset —
// petugas harus mengetik ulang seluruhnya, atau tidak mengetiknya sama sekali.

// ReceivingLine adalah satu sub-item pengajuan beserta usulan tujuannya.
type ReceivingLine struct {
	// PRItemKey = identitas baris DI DRAF INI, unik per draf — kirim balik apa
	// adanya di receive-goods. Untuk baris milik dokumen yang dibuka nilainya
	// sama dengan ItemKey; untuk baris pecahan yang dibuka lewat induknya
	// diberi awalan id pecahan ("<id pecahan>:<kunci>"), supaya baris bernama
	// kembar di induk dan pecahan tidak saling menimpa (T-29).
	PRItemKey string `json:"pr_item_key"`
	// ItemKey = kunci tersimpan baris itu di dokumen sumbernya. Aset, GRN,
	// material projek, dan keputusan penerimaan ditautkan lewat pasangan
	// (SourcePRID, ItemKey) — identitas baris yang stabil.
	ItemKey string `json:"item_key"`
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
	// RequestType: jenis pengajuan. Bahan baku hanya boleh diterima ke stok
	// gudang — layar menyembunyikan tujuan lain, server menolaknya.
	RequestType       string          `json:"request_type"`
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
	// Pickups: pengambilan oleh petugas logistik yang masih dalam pengantaran
	// (pengajuan ini beserta induk/pecahannya), terbaru dulu.
	Pickups []PurchasePickup `json:"pickups"`
}

// ReceiveGoodsLine adalah keputusan petugas atas satu baris.
type ReceiveGoodsLine struct {
	// PRItemKey = ReceivingLine.pr_item_key dari draf (identitas baris di draf).
	PRItemKey string `json:"pr_item_key"`
	// SourcePRID opsional: bila dikirim, baris dicocokkan lewat pasangan
	// (source_pr_id, item_key) — PRItemKey boleh berisi item_key maupun
	// pr_item_key draf.
	SourcePRID  string `json:"source_pr_id,omitempty"`
	Destination string `json:"destination"` // aset | material | stok | habis
	Qty         int    `json:"qty"`
	// Reason wajib untuk 'habis' (minimal 10 huruf, T-17): barang tidak boleh
	// keluar dari catatan tanpa sebab tertulis.
	Reason string `json:"reason"`

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
	// UnitPrice DIABAIKAN server (T-38): nilai aset/stok/material selalu
	// harga final dokumen (atau HPS bila harga final belum ada). Field
	// dipertahankan supaya klien lama tidak ditolak.
	UnitPrice float64 `json:"unit_price"`

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
	// PickupID: pengambilan oleh petugas logistik yang barangnya diterima di
	// sini (kosong = diantar vendor). Pengambilan itu ditandai sudah diterima.
	PickupID string `json:"pickup_id,omitempty"`
	Lines    []ReceiveGoodsLine `json:"lines"`
}

// ReceiveGoodsResult merangkum apa yang benar-benar terbentuk.
type ReceiveGoodsResult struct {
	PurchaseRequestID string   `json:"purchase_request_id"`
	Status            string   `json:"status"`
	AssetsCreated     int      `json:"assets_created"`
	AssetIDs          []string `json:"asset_ids"`
	GRNNumber         string   `json:"grn_number,omitempty"`
	// GRNNumbers: semua GRN yang terbit — satu per (gudang, dokumen sumber):
	// baris pecahan yang diterima lewat induknya ditautkan ke pecahannya
	// sendiri, supaya sisa per baris dihitung sama dari jalur mana pun (T-5).
	GRNNumbers []string `json:"grn_numbers,omitempty"`
	StockLinesQueued  int      `json:"stock_lines_queued"`
	MaterialLines     int      `json:"material_lines"`
	// ExpenseLines = baris yang dicatat sebagai habis pakai (tanpa aset/stok).
	ExpenseLines int `json:"expense_lines"`
	ReceiptStatus     string   `json:"receipt_status"`
	// OutstandingLines = baris yang masih menunggu meja lain (mis. bagian aset
	// sudah mencatat peralatan, gudang belum menerima barang dapurnya).
	OutstandingLines int `json:"outstanding_lines"`
	SkippedLines      int      `json:"skipped_lines"`
	Message           string   `json:"message"`
}
