package models

// Bukti foto serah terima.
//
// Dua momen wajib berfoto: saat barang diterima dari tim purchasing, dan saat
// barang diteruskan ke tujuan akhirnya — gudang outlet untuk barang dapur, PIC
// pengaju untuk peralatan. Tanpa foto, "barang sudah saya serahkan" dan "saya
// belum menerima apa pun" tidak bisa dibedakan.
type HandoverPhoto struct {
	ID           string `json:"id"`
	Moment       string `json:"moment"` // terima | distribusi
	Desk         string `json:"desk"`   // perlengkapan | dapur
	RefType      string `json:"ref_type"`
	RefID        string `json:"ref_id"`
	RefNumber    string `json:"ref_number"`
	PhotoURL     string `json:"photo_url"`
	Notes        string `json:"notes"`
	Actor        string `json:"actor"`
	BackupStatus string `json:"backup_status"` // pending|sent|failed|skipped
	DriveFileID  string `json:"drive_file_id"`
	DriveURL     string `json:"drive_url"`
	BackupAt     string `json:"backup_at"`
	BackupError  string `json:"backup_error"`
	CreatedAt    string `json:"created_at"`
}

// AssetHandover — serah terima aset dari bagian Aset ke PIC yang mengajukan.
type AssetHandover struct {
	ID                string              `json:"id"`
	HandoverNumber    string              `json:"handover_number"`
	PurchaseRequestID string              `json:"purchase_request_id,omitempty"`
	RequestNumber     string              `json:"request_number,omitempty"`
	OutletID          string              `json:"outlet_id"`
	OutletName        string              `json:"outlet_name"`
	PicName           string              `json:"pic_name"`
	PicPosition       string              `json:"pic_position"`
	Location          string              `json:"location"`
	Notes             string              `json:"notes"`
	HandedBy          string              `json:"handed_by"`
	CreatedAt         string              `json:"created_at"`
	Items             []AssetHandoverItem `json:"items,omitempty"`
	Photos            []HandoverPhoto     `json:"photos,omitempty"`
}

type AssetHandoverItem struct {
	ID        string `json:"id"`
	AssetID   string `json:"asset_id"`
	AssetNo   string `json:"asset_no"`
	AssetName string `json:"asset_name"`
	Unit      string `json:"unit"`
	Qty       int    `json:"qty"`
}

type AssetHandoverItemRequest struct {
	AssetID string `json:"asset_id"`
	Qty     int    `json:"qty"`
}

type AssetHandoverRequest struct {
	PurchaseRequestID string                     `json:"purchase_request_id"`
	OutletID          string                     `json:"outlet_id"`
	PicName           string                     `json:"pic_name"`
	PicPosition       string                     `json:"pic_position"`
	Location          string                     `json:"location"`
	Notes             string                     `json:"notes"`
	PhotoURL          string                     `json:"photo_url"`
	Items             []AssetHandoverItemRequest `json:"items"`
}
