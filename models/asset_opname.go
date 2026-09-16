package models

// Opname aset — audit fisik per outlet (docs/perlengkapan-aset.md §4.5).
//
// Tanpa opname, selisih antara catatan dan lapangan tidak pernah ketahuan:
// barang yang sudah lama hilang tetap tampil sebagai aset bernilai.

type AssetOpnameItem struct {
	ID             string `json:"id"`
	AssetID        string `json:"asset_id,omitempty"`
	AssetName      string `json:"asset_name"`
	AssetNo        string `json:"asset_no"`
	Unit           string `json:"unit"`
	SystemQty      int    `json:"system_qty"`
	CountedQty     *int   `json:"counted_qty"` // nil = belum dihitung
	ConditionNow   string `json:"condition_now"`
	ConditionFound string `json:"condition_found"`
	FoundName      string `json:"found_name"`
	Location       string `json:"location"`
	Notes          string `json:"notes"`
	Diff           int    `json:"diff"`
}

type AssetOpnameSession struct {
	ID           string            `json:"id"`
	OpnameNumber string            `json:"opname_number"`
	OutletID     string            `json:"outlet_id"`
	OutletName   string            `json:"outlet_name"`
	Status       string            `json:"status"` // berjalan|selesai|batal
	Notes        string            `json:"notes"`
	CreatedBy    string            `json:"created_by"`
	ApprovedBy   string            `json:"approved_by"`
	ApprovedAt   string            `json:"approved_at"`
	CreatedAt    string            `json:"created_at"`
	ItemCount    int               `json:"item_count"`
	CountedCount int               `json:"counted_count"`
	DiffCount    int               `json:"diff_count"`
	Accuracy     float64           `json:"accuracy"` // 1 − (baris selisih ÷ baris diperiksa)
	Items        []AssetOpnameItem `json:"items,omitempty"`
}

type AssetOpnameCreateRequest struct {
	OutletID string `json:"outlet_id"`
	Notes    string `json:"notes"`
}

type AssetOpnameCountLine struct {
	ItemID         string `json:"item_id"`
	AssetID        string `json:"asset_id"`
	CountedQty     *int   `json:"counted_qty"`
	ConditionFound string `json:"condition_found"`
	Notes          string `json:"notes"`
	// Temuan barang tak terdaftar: baris baru tanpa asset_id.
	FoundName string `json:"found_name"`
	Location  string `json:"location"`
}

type AssetOpnameCountRequest struct {
	Lines []AssetOpnameCountLine `json:"lines"`
}
