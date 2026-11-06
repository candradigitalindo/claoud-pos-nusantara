package models

// Model untuk endpoint data terbuka /api/v1/open (tanpa auth). Semua outlet
// dirujuk lewat slug; ID internal (outlet, produk, akun) sengaja tidak pernah
// ikut dalam respons.

type OpenOutletRef struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type OpenProduct struct {
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Kind      string  `json:"kind"`       // food | beverage | activity | other | ""
	KindLabel string  `json:"kind_label"` // Makanan | Minuman | Aktivitas | Lainnya | Belum ditentukan
	Price     float64 `json:"price"`
}

type OpenProductList struct {
	Outlet   OpenOutletRef `json:"outlet"`
	Total    int           `json:"total"`
	Products []OpenProduct `json:"products"`
}

type OpenProductSalesItem struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Qty      int     `json:"qty"`
	Revenue  float64 `json:"revenue"`
}

type OpenProductSales struct {
	Outlet       OpenOutletRef          `json:"outlet"`
	DateFrom     string                 `json:"date_from"`
	DateTo       string                 `json:"date_to"`
	TotalQty     int                    `json:"total_qty"`
	TotalRevenue float64                `json:"total_revenue"`
	Items        []OpenProductSalesItem `json:"items"`
}

type OpenSocialPoint struct {
	Date      string `json:"date"`
	Followers *int64 `json:"followers"`
	Following *int64 `json:"following"`
	Posts     *int64 `json:"posts"`
	Likes     *int64 `json:"likes"`
	Approx    bool   `json:"approx"` // angka dibulatkan oleh platform (mis. "12,3 rb")
}

type OpenSocialAccount struct {
	Platform   string `json:"platform"`
	Username   string `json:"username"`
	ProfileURL string `json:"profile_url"`
	// Latest = pembacaan terakhir yang punya angka pengikut, tidak dibatasi rentang tanggal.
	Latest *OpenSocialPoint `json:"latest"`
	// FollowersChange = pengikut terakhir − pengikut pertama di dalam rentang; null bila < 2 pembacaan.
	FollowersChange *int64            `json:"followers_change"`
	Daily           []OpenSocialPoint `json:"daily"`
}

type OpenSocialOutlet struct {
	Name     string              `json:"name"`
	Slug     string              `json:"slug"`
	Accounts []OpenSocialAccount `json:"accounts"`
}

type OpenSocial struct {
	DateFrom string             `json:"date_from"`
	DateTo   string             `json:"date_to"`
	Outlets  []OpenSocialOutlet `json:"outlets"`
}
