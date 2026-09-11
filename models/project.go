package models

// Projek pembangunan / renovasi — payung di atas pengadaan.
//
// Projek sengaja TIDAK punya alur setujui/bayar/terima sendiri: dia hanya
// memegang RAB (satu angka total) dan mengelompokkan pengajuan pengadaan yang
// dibelanjakan bertahap. Seluruh siklus hidup belanja tetap milik
// purchase_requests.

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
	Budget        float64 `json:"budget"`
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
}

// ProjectDetailResponse = header + rekap + daftar belanja tahap di dalamnya.
type ProjectDetailResponse struct {
	Project  Project           `json:"project"`
	Requests []PurchaseRequest `json:"requests"`
}

// CreateProjectRequest adalah payload pembuatan projek.
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

// UpdateProjectRequest adalah payload perubahan projek.
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
