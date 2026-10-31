package models

// Pemesanan mandiri oleh tamu (QR dine-in). Order MASUK dari internet, lalu
// DITARIK POS — arah yang berlawanan dengan sinkronisasi lain di sistem ini,
// yang selalu POS → cloud.
//
// Alur hidupnya bertingkat supaya order tidak pernah hilang maupun dobel:
//
//	new       → baru masuk, belum ada perangkat yang mengambil
//	claimed   → satu perangkat POS mengambilnya (atomik, hanya satu yang menang)
//	confirmed → POS berhasil membuatnya jadi order meja; tak akan ditarik lagi
//	rejected  → ditolak staf
//
// Klaim yang tidak pernah dikonfirmasi (POS mati di tengah jalan) kedaluwarsa
// dan kembali menjadi `new`, sehingga perangkat lain bisa mengambilnya.
const (
	// Pesanan baru dibuat dan menunggu tamu membayar QRIS-nya. Pada tahap ini
	// pesanan TIDAK terlihat oleh POS — dapur tidak pernah memasak pesanan yang
	// belum dibayar.
	OnlineOrderAwaitingPayment = "awaiting_payment"
	OnlineOrderNew             = "new"
	OnlineOrderClaimed         = "claimed"
	OnlineOrderConfirmed       = "confirmed"
	OnlineOrderRejected        = "rejected"
)

// OnlineOrderAddon — add-on terpilih pada satu baris pesanan online.
// Nama & harga ikut dikirim, tetapi POS SELALU menghitung ulang dari master
// add-on miliknya sendiri; nilai di sini hanya untuk tampilan dan audit.
type OnlineOrderAddon struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type OnlineOrderItem struct {
	ProductLocalID string             `json:"product_local_id"`
	ProductName    string             `json:"product_name"`
	Qty            int                `json:"qty"`
	Notes          string             `json:"notes"`
	Addons         []OnlineOrderAddon `json:"addons"`
}

// PublicOrderRequest — badan permintaan dari halaman pemesanan tamu.
type PublicOrderRequest struct {
	TableNumber   string            `json:"table_number"`
	CustomerName  string            `json:"customer_name"`
	CustomerPhone string            `json:"customer_phone"`
	Notes         string            `json:"notes"`
	Items         []OnlineOrderItem `json:"items"`
}

// OnlineOrder — bentuk yang ditarik POS sekaligus yang dikembalikan ke tamu
// setelah memesan.
type OnlineOrder struct {
	ID            string            `json:"id"`
	OutletID      string            `json:"outlet_id"`
	TableNumber   string            `json:"table_number"`
	CustomerName  string            `json:"customer_name"`
	CustomerPhone string            `json:"customer_phone"`
	Notes         string            `json:"notes"`
	Items         []OnlineOrderItem `json:"items"`
	Status        string            `json:"status"`
	CreatedAt     string            `json:"created_at"`

	// Rincian tagihan. Dihitung cloud memakai daftar biaya yang dikirim POS
	// supaya sama persis dengan tagihan yang nanti dihitung kasir.
	Subtotal    float64      `json:"subtotal"`
	ChargeLines []ChargeLine `json:"charges"`
	TotalAmount float64      `json:"total_amount"`

	// PaidAmount = nominal yang BENAR-BENAR diterima penyedia. POS mencatat
	// angka ini, bukan TotalAmount, supaya selisih akibat konfigurasi biaya
	// yang sempat berbeda muncul sebagai sisa tagihan alih-alih tertutup diam-diam.
	PaidAmount float64 `json:"paid_amount"`

	// Payment hanya terisi saat pesanan baru dibuat — berisi QR yang harus
	// dipindai tamu.
	Payment *QRISChargeResponse `json:"payment,omitempty"`
}
