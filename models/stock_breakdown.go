package models

// ── Pemotongan (satu bahan → beberapa hasil) ──────────────────
//
// Kebalikan dari Produksi (banyak bahan → satu hasil, rasio tetap): satu bahan
// dipotong/diolah menjadi beberapa hasil yang dipakai menu berbeda — mis. ayam
// utuh → paha, dada, sayap; daging sapi → has dalam, has luar, tetelan; ikan
// utuh → fillet, kepala; beras → nasi. Jumlah hasilnya tidak pernah persis
// sama tiap kali, jadi stok dan HPP mengikuti hasil AKTUAL. Satu bahan boleh
// punya beberapa POLA bernama (mis. ayam "Per Bagian" vs "Potong 8"); tiap
// pola memberi hasil standar (pembanding susut/yield) dan bobot biaya.

// StockItemBreakdownLine — satu baris hasil dalam pola: hasil standar per 1
// satuan distribusi bahan (dalam satuan dasar HASIL) dan bobot biayanya.
type StockItemBreakdownLine struct {
	ID             string  `json:"id"`
	OutputItemID   string  `json:"output_item_id"`
	OutputItemCode string  `json:"output_item_code"`
	OutputItemName string  `json:"output_item_name"`
	OutputBaseUnit string  `json:"output_base_unit"`
	OutputIsActive bool    `json:"output_is_active"`
	QtyPerInput    float64 `json:"qty_per_input"`
	CostWeight     float64 `json:"cost_weight"`
	Notes          string  `json:"notes"`
}

// StockBreakdownPattern — satu pola pemotongan beserta baris hasilnya.
type StockBreakdownPattern struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	Notes     string                   `json:"notes"`
	IsActive  bool                     `json:"is_active"`
	Outputs   []StockItemBreakdownLine `json:"outputs"`
	UpdatedBy string                   `json:"updated_by"`
	UpdatedAt string                   `json:"updated_at"`
}

// StockItemBreakdownTemplate — pola-pola pemotongan satu bahan beserta satuan
// bahannya (qty pemakaian diisi dalam satuan distribusi bahan).
type StockItemBreakdownTemplate struct {
	InputItemID        string                  `json:"input_item_id"`
	InputItemCode      string                  `json:"input_item_code"`
	InputItemName      string                  `json:"input_item_name"`
	InputBaseUnit      string                  `json:"input_base_unit"`
	InputDistUnit      string                  `json:"input_dist_unit"`
	InputDistUnitLabel string                  `json:"input_dist_unit_label"`
	InputDistRatio     float64                 `json:"input_dist_ratio"`
	InputIsActive      bool                    `json:"input_is_active"`
	Patterns           []StockBreakdownPattern `json:"patterns"`
}

// StockItemBreakdownRequest — simpan SELURUH pola satu bahan (ganti semua):
// pola ber-id diperbarui, pola tanpa id dibuat, pola yang tidak dikirim
// dihapus. Daftar kosong = semua pola dihapus; riwayat pemotongan tetap
// menyimpan nama polanya.
type StockItemBreakdownRequest struct {
	Patterns []StockBreakdownPatternReq `json:"patterns"`
}

type StockBreakdownPatternReq struct {
	ID    string `json:"id"` // kosong = pola baru
	Name  string `json:"name"`
	Notes string `json:"notes"`
	// IsActive: nil (tidak dikirim) = aktif. Pola nonaktif tidak ditawarkan
	// di form Pemotongan, tetapi baris hasilnya tetap tersimpan.
	IsActive *bool                       `json:"is_active"`
	Outputs  []StockItemBreakdownLineReq `json:"outputs"`
}

type StockItemBreakdownLineReq struct {
	OutputItemID string  `json:"output_item_id"`
	QtyPerInput  float64 `json:"qty_per_input"`
	// CostWeight: nil (tidak dikirim) = 1; harus > 0 bila dikirim.
	CostWeight *float64 `json:"cost_weight"`
	Notes      string   `json:"notes"`
}

// StockBreakdownRequest — catat satu pemotongan di sebuah gudang.
type StockBreakdownRequest struct {
	WarehouseID string `json:"warehouse_id"`
	InputItemID string `json:"input_item_id"`
	// PatternID: pola yang dipakai — wajib bila bahan punya lebih dari satu
	// pola aktif; kosong = satu-satunya pola aktif bahan itu.
	PatternID string `json:"pattern_id"`
	// InputQtyDist: bahan yang benar-benar dipakai, dalam satuan DISTRIBUSI
	// bahan (mis. ekor, kg); dikonversi ke satuan dasar dengan dist_ratio.
	InputQtyDist float64                   `json:"input_qty_dist"`
	Outputs      []StockBreakdownOutputReq `json:"outputs"`
	Notes        string                    `json:"notes"`
}

type StockBreakdownOutputReq struct {
	ItemID  string  `json:"item_id"`
	QtyBase float64 `json:"qty_base"` // hasil aktual, satuan dasar hasil
}

// StockBreakdown — dokumen pemotongan.
type StockBreakdown struct {
	ID              string `json:"id"`
	BreakdownNumber string `json:"breakdown_number"`
	WarehouseID     string `json:"warehouse_id"`
	WarehouseName   string `json:"warehouse_name"`
	WarehouseType   string `json:"warehouse_type"`
	InputItemID     string `json:"input_item_id"`
	InputItemCode   string `json:"input_item_code"`
	InputItemName   string `json:"input_item_name"`
	InputBaseUnit   string `json:"input_base_unit"`
	// PatternID kosong bila polanya sudah dihapus; PatternName = salinan nama
	// pola saat dicatat (tidak ikut berubah bila pola diganti namanya).
	PatternID   string `json:"pattern_id"`
	PatternName string `json:"pattern_name"`
	// InputUnit: satuan distribusi saat pemotongan dicatat (snapshot) —
	// satuan InputQtyDist, tetap benar walau satuan katalog diubah kemudian.
	InputUnit    string  `json:"input_unit"`
	InputQtyDist float64 `json:"input_qty_dist"`
	InputQtyBase float64 `json:"input_qty_base"`
	// InputCostTotal: biaya FIFO bahan yang benar-benar terpotong.
	InputCostTotal   float64 `json:"input_cost_total"`
	InputCostPerBase float64 `json:"input_cost_per_base"`
	// ExpiryDate: kedaluwarsa yang diwarisi batch hasil = kedaluwarsa
	// terdekat dari batch bahan yang terpotong ("" = bahan tanpa kedaluwarsa).
	ExpiryDate string `json:"expiry_date"`
	// YieldPct: hasil aktual dibanding standar pola, ditimbang bobot biaya
	// (Σ aktual×bobot/standar ÷ Σ standar×bobot/standar × 100). Boleh > 100
	// (mis. beras → nasi yang mengembang lebih dari standar).
	YieldPct float64 `json:"yield_pct"`
	// ShrinkCost: bagian biaya bahan yang "hilang" karena hasil di bawah
	// standar — biaya × (1 − yield). Negatif bila hasil melebihi standar.
	ShrinkCost float64                `json:"shrink_cost"`
	Notes      string                 `json:"notes"`
	CreatedBy  string                 `json:"created_by"`
	CreatedAt  string                 `json:"created_at"`
	Outputs    []StockBreakdownOutput `json:"outputs"`
}

type StockBreakdownOutput struct {
	ID         string  `json:"id"`
	ItemID     string  `json:"item_id"`
	ItemCode   string  `json:"item_code"`
	ItemName   string  `json:"item_name"`
	BaseUnit   string  `json:"base_unit"`
	QtyBase    float64 `json:"qty_base"`     // aktual
	StdQtyBase float64 `json:"std_qty_base"` // qty bahan (satuan distribusi) × hasil standar
	// YieldPct: aktual ÷ standar × 100 (0 bila standar 0).
	YieldPct    float64 `json:"yield_pct"`
	QtyPerInput float64 `json:"qty_per_input"`
	CostWeight  float64 `json:"cost_weight"`
	CostPerBase float64 `json:"cost_per_base"`
	TotalCost   float64 `json:"total_cost"`
}
