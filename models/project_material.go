package models

// Material projek — barang habis pakai yang dikonsumsi sebuah projek
// pembangunan/renovasi (docs/perlengkapan-aset.md §3.8, §8.8).
//
// Sebelum ini, semen dan cat senilai puluhan juta hanya berupa angka `paid` di
// rekap RAB: tidak ada catatan berapa yang datang, berapa terpakai, dan ke mana
// sisanya pergi saat projek selesai.

type ProjectMaterial struct {
	ID                string  `json:"id"`
	ProjectID         string  `json:"project_id"`
	ProjectName       string  `json:"project_name,omitempty"`
	PurchaseRequestID string  `json:"purchase_request_id,omitempty"`
	RequestNumber     string  `json:"request_number,omitempty"`
	Name              string  `json:"name"`
	Unit              string  `json:"unit"`
	QtyReceived       float64 `json:"qty_received"`
	QtyUsed           float64 `json:"qty_used"`
	QtyReturned       float64 `json:"qty_returned"`
	QtyWasted         float64 `json:"qty_wasted"`
	// Remaining dihitung, tidak disimpan — supaya tidak pernah basi.
	Remaining     float64 `json:"remaining"`
	UnitCost      float64 `json:"unit_cost"`
	ValueReceived float64 `json:"value_received"`
	ValueRemaining float64 `json:"value_remaining"`
	Location      string  `json:"location"`
	ReceivedAt    string  `json:"received_at"`
	ReceivedBy    string  `json:"received_by"`
	Notes         string  `json:"notes"`
}

type ProjectMaterialLog struct {
	ID         string  `json:"id"`
	MaterialID string  `json:"material_id"`
	Type       string  `json:"type"` // penerimaan|pemakaian|pengembalian|susut|koreksi
	Qty        float64 `json:"qty"`
	RefType    string  `json:"ref_type"`
	RefNumber  string  `json:"ref_number"`
	Notes      string  `json:"notes"`
	Actor      string  `json:"actor"`
	CreatedAt  string  `json:"created_at"`
}

// ProjectMaterialUsageRequest — pencatatan pemakaian di lokasi.
type ProjectMaterialUsageRequest struct {
	Qty   float64 `json:"qty"`
	Notes string  `json:"notes"`
}

// ProjectMaterialSettleRequest — menentukan nasib sisa material.
type ProjectMaterialSettleRequest struct {
	// Action: gudang | aset | susut | pindah
	Action string  `json:"action"`
	Qty    float64 `json:"qty"`
	Notes  string  `json:"notes"`

	// action=gudang
	StockItemID string `json:"stock_item_id"`
	WarehouseID string `json:"warehouse_id"`
	// action=aset
	OutletID string `json:"outlet_id"`
	Category string `json:"category"`
	// action=pindah
	TargetProjectID string `json:"target_project_id"`
}

// ProjectMaterialSummary — rekap material satu projek.
type ProjectMaterialSummary struct {
	Lines          int     `json:"lines"`
	ValueReceived  float64 `json:"value_received"`
	ValueUsed      float64 `json:"value_used"`
	ValueWasted    float64 `json:"value_wasted"`
	ValueRemaining float64 `json:"value_remaining"`
	UnsettledLines int     `json:"unsettled_lines"`
}
