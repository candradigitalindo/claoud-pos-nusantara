package models

// Mutasi aset antar outlet.
//
// Alurnya sengaja meniru stock_transfers (draft → pending → approved → sent →
// received) supaya tim tidak perlu mempelajari alur kedua. Bedanya ada pada
// barangnya: aset bernomor tunggal berpindah utuh beserta riwayatnya, sedangkan
// aset massal dipecah/digabung berdasarkan jumlah. Lihat docs/perlengkapan-aset.md §6.

type AssetTransferItem struct {
	ID             string `json:"id"`
	AssetID        string `json:"asset_id"`
	AssetNo        string `json:"asset_no"`
	AssetName      string `json:"asset_name"`
	Unit           string `json:"unit"`
	TrackingMode   string `json:"tracking_mode"`
	Qty            int    `json:"qty"`
	ReceivedQty    *int   `json:"received_qty"` // nil sebelum diterima
	ConditionSent  string `json:"condition_sent"`
	ConditionRecv  string `json:"condition_recv"`
	TargetAssetID  string `json:"target_asset_id,omitempty"`
	AvailableQty   int    `json:"available_qty,omitempty"` // sisa di outlet asal, untuk layar
	Notes          string `json:"notes"`
}

type AssetTransfer struct {
	ID             string              `json:"id"`
	TransferNumber string              `json:"transfer_number"`
	FromOutletID   string              `json:"from_outlet_id"`
	FromOutletName string              `json:"from_outlet_name"`
	ToOutletID     string              `json:"to_outlet_id"`
	ToOutletName   string              `json:"to_outlet_name"`
	Status         string              `json:"status"` // draft|pending|approved|sent|received|rejected|cancelled
	Reason         string              `json:"reason"`
	ExpectedReturn string              `json:"expected_return"`
	Notes          string              `json:"notes"`
	// PhotoURL: bukti barang saat dikirim dari outlet asal (wajib saat 'sent'),
	// sama seperti transfer stok dan serah terima ke PIC.
	PhotoURL       string              `json:"photo_url"`
	RejectedReason string              `json:"rejected_reason"`
	CreatedBy      string              `json:"created_by"`
	ApprovedBy     string              `json:"approved_by"`
	ApprovedAt     string              `json:"approved_at"`
	SentBy         string              `json:"sent_by"`
	SentAt         string              `json:"sent_at"`
	ReceivedBy     string              `json:"received_by"`
	ReceivedAt     string              `json:"received_at"`
	ItemCount      int                 `json:"item_count"`
	TotalQty       int                 `json:"total_qty"`
	// HasShortfall menandai dokumen yang diterima lebih sedikit dari yang dikirim.
	HasShortfall bool                `json:"has_shortfall"`
	Items        []AssetTransferItem `json:"items,omitempty"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    string              `json:"updated_at"`
}

type AssetTransferItemRequest struct {
	AssetID string `json:"asset_id"`
	Qty     int    `json:"qty"`
	Notes   string `json:"notes"`
}

type AssetTransferRequest struct {
	FromOutletID   string                     `json:"from_outlet_id"`
	ToOutletID     string                     `json:"to_outlet_id"`
	Reason         string                     `json:"reason"`
	ExpectedReturn string                     `json:"expected_return"`
	Notes          string                     `json:"notes"`
	Items          []AssetTransferItemRequest `json:"items"`
}

// AssetTransferActionRequest dipakai semua aksi status. `Items` hanya dibaca
// saat menerima: qty yang benar-benar sampai, per baris.
type AssetTransferActionRequest struct {
	Reason   string                     `json:"reason"`    // alasan penolakan
	PhotoURL string                     `json:"photo_url"` // bukti kirim, wajib saat send
	Items    []AssetTransferReceiptLine `json:"items"`
}

type AssetTransferReceiptLine struct {
	ItemID        string `json:"item_id"`
	ReceivedQty   int    `json:"received_qty"`
	ConditionRecv string `json:"condition_recv"`
}
