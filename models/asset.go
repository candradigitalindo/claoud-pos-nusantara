package models

import "time"

// Asset — barang inventaris outlet (meja, kursi, elektronik, dll).
//
// Kondisi dan status adalah dua sumbu berbeda: `Condition` menjawab "bagaimana
// barangnya" (baik/rusak), `Status` menjawab "di mana posisinya dalam siklus
// hidup" (aktif/dipinjam/perbaikan/transit/tidak_aktif/dihapus). Sebelum Fase 1
// keduanya bercampur di satu kolom, sehingga aset yang sedang diperbaiki tidak
// bisa dibedakan dari aset yang rusak tapi masih di tempat.
type Asset struct {
	ID               string    `json:"id"`
	AssetNo          string    `json:"asset_no"`
	OutletID         string    `json:"outlet_id"`
	OutletName       string    `json:"outlet_name"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	Quantity         int       `json:"quantity"`
	Unit             string    `json:"unit"`
	TrackingMode     string    `json:"tracking_mode"` // tunggal | massal
	SerialNumber     string    `json:"serial_number"`
	Brand            string    `json:"brand"`
	Model            string    `json:"model"`
	Condition        string    `json:"condition"` // baik | rusak_ringan | rusak_berat
	Status           string    `json:"status"`    // aktif | dipinjam | perbaikan | transit | tidak_aktif | dihapus
	Location         string    `json:"location"`
	PicName          string    `json:"pic_name"`
	AcquisitionSrc   string    `json:"acquisition_src"`
	PurchaseDate     string    `json:"purchase_date"` // YYYY-MM-DD ('' bila kosong)
	PurchasePrice    float64   `json:"purchase_price"`
	WarrantyUntil    string    `json:"warranty_until"` // YYYY-MM-DD ('' bila kosong)
	UsefulLifeMonths int       `json:"useful_life_months"`
	ResidualValue    float64   `json:"residual_value"`
	PhotoURL         string    `json:"photo_url"`
	Notes            string    `json:"notes"`
	MaintenanceCount int       `json:"maintenance_count"`
	LastMaintenance  string    `json:"last_maintenance"` // YYYY-MM-DD ('' bila belum ada)
	MaintenanceCost  float64   `json:"maintenance_cost"` // total biaya perawatan seumur hidup
	// Turunan penyusutan garis lurus, dihitung saat query (bukan kolom).
	MonthsElapsed int     `json:"months_elapsed"`
	Depreciation  float64 `json:"depreciation"` // akumulasi penyusutan
	BookValue     float64 `json:"book_value"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AssetMovement — satu baris buku besar aset. Append-only: koreksi dilakukan
// dengan menambah baris baru, tidak pernah mengubah baris lama.
type AssetMovement struct {
	ID              string    `json:"id"`
	AssetID         string    `json:"asset_id"`
	Type            string    `json:"type"`
	Qty             int       `json:"qty"`
	FromOutletID    string    `json:"from_outlet_id,omitempty"`
	ToOutletID      string    `json:"to_outlet_id,omitempty"`
	FromOutletName  string    `json:"from_outlet_name,omitempty"`
	ToOutletName    string    `json:"to_outlet_name,omitempty"`
	FromLocation    string    `json:"from_location"`
	ToLocation      string    `json:"to_location"`
	ConditionBefore string    `json:"condition_before"`
	ConditionAfter  string    `json:"condition_after"`
	RefType         string    `json:"ref_type"`
	RefID           string    `json:"ref_id,omitempty"`
	RefNumber       string    `json:"ref_number"`
	Amount          float64   `json:"amount"`
	Notes           string    `json:"notes"`
	Actor           string    `json:"actor"`
	CreatedAt       time.Time `json:"created_at"`
}

// AssetMaintenance — satu work order perawatan/perbaikan sebuah aset.
//
// Sebelum Fase 3 ini hanya catatan mundur: pekerjaan yang sudah terjadi.
// Sekarang ia punya siklus hidup (dijadwalkan → berjalan → selesai), sehingga
// jadwal yang tersimpan di NextDueDate benar-benar dibaca dan ditagih.
type AssetMaintenance struct {
	ID              string    `json:"id"`
	WONumber        string    `json:"wo_number"`
	AssetID         string    `json:"asset_id"`
	AssetName       string    `json:"asset_name,omitempty"`
	AssetNo         string    `json:"asset_no,omitempty"`
	OutletID        string    `json:"outlet_id,omitempty"`
	OutletName      string    `json:"outlet_name,omitempty"`
	Status          string    `json:"status"`         // dijadwalkan | berjalan | selesai | batal
	ScheduledDate   string    `json:"scheduled_date"` // YYYY-MM-DD ('' bila kosong)
	MaintenanceDate string    `json:"maintenance_date"` // YYYY-MM-DD
	Type            string    `json:"type"`             // rutin | perbaikan | penggantian | inspeksi
	Description     string    `json:"description"`
	Cost            float64   `json:"cost"`
	PerformedBy     string    `json:"performed_by"`
	VendorID        string    `json:"vendor_id,omitempty"`
	VendorName      string    `json:"vendor_name,omitempty"`
	PurchaseRequestID     string `json:"purchase_request_id,omitempty"`
	PurchaseRequestNumber string `json:"purchase_request_number,omitempty"`
	DowntimeHours   float64   `json:"downtime_hours"`
	AttachmentURL   string    `json:"attachment_url"`
	ConditionAfter  string    `json:"condition_after"`
	NextDueDate     string    `json:"next_due_date"` // YYYY-MM-DD ('' bila kosong)
	CreatedBy       string    `json:"created_by"`
	// DueInDays < 0 berarti terlambat. Hanya terisi untuk WO yang dijadwalkan.
	DueInDays *int      `json:"due_in_days,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type AssetRequest struct {
	OutletID         string  `json:"outlet_id"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	Category         string  `json:"category"`
	Quantity         int     `json:"quantity"`
	Unit             string  `json:"unit"`
	TrackingMode     string  `json:"tracking_mode"`
	SerialNumber     string  `json:"serial_number"`
	Brand            string  `json:"brand"`
	Model            string  `json:"model"`
	Condition        string  `json:"condition"`
	Status           string  `json:"status"`
	Location         string  `json:"location"`
	PicName          string  `json:"pic_name"`
	PurchaseDate     string  `json:"purchase_date"`
	PurchasePrice    float64 `json:"purchase_price"`
	WarrantyUntil    string  `json:"warranty_until"`
	UsefulLifeMonths int     `json:"useful_life_months"`
	ResidualValue    float64 `json:"residual_value"`
	PhotoURL         string  `json:"photo_url"`
	Notes            string  `json:"notes"`
}

type AssetMaintenanceRequest struct {
	// Status menentukan bentuk dokumen: 'dijadwalkan' membuat rencana kerja,
	// 'selesai' (default) mencatat pekerjaan yang sudah terjadi — bentuk yang
	// dipakai form lama, dan sengaja tetap didukung.
	Status          string  `json:"status"`
	ScheduledDate   string  `json:"scheduled_date"`
	MaintenanceDate string  `json:"maintenance_date"`
	Type            string  `json:"type"`
	Description     string  `json:"description"`
	Cost            float64 `json:"cost"`
	PerformedBy     string  `json:"performed_by"`
	VendorID        string  `json:"vendor_id"`
	DowntimeHours   float64 `json:"downtime_hours"`
	AttachmentURL   string  `json:"attachment_url"`
	ConditionAfter  string  `json:"condition_after"`
	NextDueDate     string  `json:"next_due_date"`
}

// AssetMaintenanceCompleteRequest dipakai saat menutup work order: angka yang
// baru diketahui setelah pekerjaan benar-benar selesai.
type AssetMaintenanceCompleteRequest struct {
	MaintenanceDate string  `json:"maintenance_date"`
	Description     string  `json:"description"`
	Cost            float64 `json:"cost"`
	PerformedBy     string  `json:"performed_by"`
	DowntimeHours   float64 `json:"downtime_hours"`
	ConditionAfter  string  `json:"condition_after"`
	NextDueDate     string  `json:"next_due_date"`
	AttachmentURL   string  `json:"attachment_url"`
}
