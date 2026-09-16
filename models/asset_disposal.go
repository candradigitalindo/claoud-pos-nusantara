package models

// Penghapusan aset (docs/perlengkapan-aset.md §5.4).
//
// Soft delete biasa dipakai untuk SALAH INPUT. Barang yang rusak, dijual, atau
// hilang harus lewat dokumen ini — supaya nilainya tercatat dan laporan aset
// tidak diam-diam menyusut tanpa sebab.
type AssetDisposal struct {
	ID             string  `json:"id"`
	DisposalNumber string  `json:"disposal_number"`
	AssetID        string  `json:"asset_id"`
	AssetName      string  `json:"asset_name"`
	AssetNo        string  `json:"asset_no"`
	OutletID       string  `json:"outlet_id"`
	OutletName     string  `json:"outlet_name"`
	Qty            int     `json:"qty"`
	Method         string  `json:"method"` // dijual|dihibahkan|dimusnahkan|hilang|tukar_tambah
	Reason         string  `json:"reason"`
	Proceeds       float64 `json:"proceeds"`
	BookValue      float64 `json:"book_value"`
	Status         string  `json:"status"` // pending|approved|rejected
	RequestedBy    string  `json:"requested_by"`
	ApprovedBy     string  `json:"approved_by"`
	ApprovedAt     string  `json:"approved_at"`
	RejectedReason string  `json:"rejected_reason"`
	AttachmentURL  string  `json:"attachment_url"`
	CreatedAt      string  `json:"created_at"`
}

type AssetDisposalRequest struct {
	AssetID       string  `json:"asset_id"`
	Qty           int     `json:"qty"`
	Method        string  `json:"method"`
	Reason        string  `json:"reason"`
	Proceeds      float64 `json:"proceeds"`
	AttachmentURL string  `json:"attachment_url"`
}

type AssetDisposalApproveRequest struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}
