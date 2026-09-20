package models

// Projek pembangunan / renovasi — payung di atas pengadaan.
//
// Projek sengaja TIDAK punya alur setujui/bayar/terima sendiri: dia memegang
// RAB (Rencana Anggaran Biaya) yang disusun per baris pekerjaan, lalu
// mengelompokkan pengajuan pengadaan yang dibelanjakan bertahap. Seluruh
// siklus hidup belanja tetap milik purchase_requests; tiap item belanja
// menunjuk baris RAB mana yang ia serap.
//
// Alur: buat projek → susun RAB (draft) → tetapkan RAB → belanja tahap per
// baris RAB → serapan terlihat per baris. RAB yang sudah ditetapkan hanya bisa
// diubah lewat "buka revisi", dan selama revisi belum ditetapkan lagi,
// pengajuan baru untuk projek itu ditolak.

// Project adalah header projek beserta rekap serapan anggarannya.
type Project struct {
	ID            string  `json:"id"`
	ProjectNumber string  `json:"project_number"`
	Name          string  `json:"name"`
	OutletID      *string `json:"outlet_id"`
	OutletName    string  `json:"outlet_name,omitempty"`
	WorkUnitID    *string `json:"work_unit_id"`
	WorkUnitName  string  `json:"work_unit_name,omitempty"`
	PIC           string  `json:"pic"`
	// Budget = total RAB = Σ subtotal baris RAB. Disinkronkan setiap kali baris
	// berubah; tidak pernah diisi tangan lagi.
	Budget float64 `json:"budget"`
	// RabStatus: 'draft' (masih disusun / sedang direvisi) | 'ditetapkan'.
	RabStatus    string `json:"rab_status"`
	RabVersion   int    `json:"rab_version"`
	RabSetAt     string `json:"rab_set_at"`
	RabSetBy     string `json:"rab_set_by"`
	RabItemCount int    `json:"rab_item_count"`
	StartDate     string  `json:"start_date"`
	TargetDate    string  `json:"target_date"`
	Status        string  `json:"status"` // draft | berjalan | selesai | batal
	Notes         string  `json:"notes"`
	CreatedBy     string  `json:"created_by"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`

	// Rekap — dihitung dari purchase_requests milik projek ini.
	ProjectSummary
}

// ProjectSummary adalah angka serapan anggaran sebuah projek.
//
// Committed = nilai yang sudah "dikunci" lewat pengajuan yang masih hidup
// (bukan ditolak/dibatalkan), memakai harga final bila purchasing sudah
// mengisinya dan HPS bila belum — gunanya RAB adalah mencegah over-komit, jadi
// pengajuan yang masih menunggu harga tetap harus membebani anggaran.
// Estimated = bagian dari Committed yang masih berupa HPS, supaya terlihat
// seberapa besar angka itu masih tebakan.
// Paid = yang benar-benar sudah keluar uangnya.
// Outstanding = sisa yang masih harus dibayar ke vendor; ini TETAP berbasis
// harga final — tidak ada hutang atas harga yang belum disepakati.
type ProjectSummary struct {
	RequestCount    int     `json:"request_count"`
	Committed       float64 `json:"committed"`
	Estimated       float64 `json:"estimated"`
	Paid            float64 `json:"paid"`
	Outstanding     float64 `json:"outstanding"`
	RemainingBudget float64 `json:"remaining_budget"`
	// AbsorbedPct = Committed / Budget × 100. 0 bila RAB belum diisi.
	AbsorbedPct float64 `json:"absorbed_pct"`
	// OverBudget true bila komitmen sudah melewati RAB.
	OverBudget bool `json:"over_budget"`
	// OffRabCommitted/OffRabPaid = belanja projek yang itemnya tidak menunjuk
	// baris RAB mana pun (atau menunjuk baris yang sudah tidak ada). Tetap
	// bagian dari Committed/Paid, hanya ditandai supaya terlihat.
	OffRabCommitted float64 `json:"off_rab_committed"`
	OffRabPaid      float64 `json:"off_rab_paid"`
}

// ProjectRabItem adalah satu baris RAB beserta serapannya.
//
// Kind: 'barang' | 'jasa' | 'umum'. Pengajuan barang hanya boleh menyerap
// baris barang/umum, pengajuan jasa hanya baris jasa/umum.
type ProjectRabItem struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"project_id"`
	Seq       int     `json:"seq"`
	Section   string  `json:"section"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Unit      string  `json:"unit"`
	Qty       float64 `json:"qty"`
	UnitPrice float64 `json:"unit_price"`
	Subtotal  float64 `json:"subtotal"`
	Notes     string  `json:"notes"`

	// Serapan — dihitung dari item purchase_requests yang menunjuk baris ini.
	// Paid dialokasikan proporsional dari pembayaran dokumen (pembayaran
	// dicatat per dokumen, bukan per item).
	RequestCount int     `json:"request_count"`
	Committed    float64 `json:"committed"`
	Estimated    float64 `json:"estimated"`
	Paid         float64 `json:"paid"`
	Remaining    float64 `json:"remaining"`
	AbsorbedPct  float64 `json:"absorbed_pct"`
	OverBudget   bool    `json:"over_budget"`
}

// ProjectRabItemInput adalah satu baris RAB dari editor. ID kosong = baris baru;
// baris lama yang tidak dikirim lagi akan dihapus (ditolak bila sudah dipakai
// pengajuan).
type ProjectRabItemInput struct {
	ID        string  `json:"id"`
	Section   string  `json:"section"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Unit      string  `json:"unit"`
	Qty       float64 `json:"qty"`
	UnitPrice float64 `json:"unit_price"`
	Notes     string  `json:"notes"`
}

// SaveProjectRabRequest mengganti seluruh susunan RAB sebuah projek.
type SaveProjectRabRequest struct {
	Items []ProjectRabItemInput `json:"items"`
}

// ProjectRabResponse = daftar baris RAB + status RAB projeknya.
type ProjectRabResponse struct {
	ProjectID     string           `json:"project_id"`
	ProjectNumber string           `json:"project_number"`
	ProjectStatus string           `json:"project_status"`
	RabStatus     string           `json:"rab_status"`
	RabVersion    int              `json:"rab_version"`
	Total         float64          `json:"total"`
	Items         []ProjectRabItem `json:"items"`
}

// ProjectDetailResponse = header + rekap + RAB per baris + daftar belanja tahap.
type ProjectDetailResponse struct {
	Project  Project           `json:"project"`
	RabItems []ProjectRabItem  `json:"rab_items"`
	Requests []PurchaseRequest `json:"requests"`
}

// CreateProjectRequest adalah payload pembuatan projek.
//
// Budget opsional dan hanya jalan pintas API: bila diisi, projek lahir dengan
// satu baris RAB gelondongan ("Anggaran projek") yang langsung ditetapkan.
// Alur UI tidak mengirimnya — RAB disusun per baris di halaman detail.
type CreateProjectRequest struct {
	Name       string  `json:"name"`
	OutletID   string  `json:"outlet_id"`
	WorkUnitID string  `json:"work_unit_id"`
	PIC        string  `json:"pic"`
	Budget     float64 `json:"budget"`
	StartDate  string  `json:"start_date"`
	TargetDate string  `json:"target_date"`
	Status     string  `json:"status"`
	Notes      string  `json:"notes"`
}

// UpdateProjectRequest adalah payload perubahan projek. Budget diabaikan —
// total RAB hanya berubah lewat susunan barisnya.
type UpdateProjectRequest struct {
	Name       string  `json:"name"`
	OutletID   string  `json:"outlet_id"`
	WorkUnitID string  `json:"work_unit_id"`
	PIC        string  `json:"pic"`
	Budget     float64 `json:"budget"`
	StartDate  string  `json:"start_date"`
	TargetDate string  `json:"target_date"`
	Status     string  `json:"status"`
	Notes      string  `json:"notes"`
}
