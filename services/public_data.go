package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Data terbuka (/api/v1/open, tanpa auth). Outlet hanya dirujuk lewat slug dan
// tidak satu pun ID internal yang dikembalikan.

// ErrOpenOutletNotFound = slug tidak dikenal atau outletnya nonaktif.
var ErrOpenOutletNotFound = errors.New("outlet tidak ditemukan")

var openKindLabels = map[string]string{
	"food": "Makanan", "beverage": "Minuman", "activity": "Aktivitas", "other": "Lainnya",
}

func openKindLabel(kind string) string {
	if l, ok := openKindLabels[kind]; ok {
		return l
	}
	return "Belum ditentukan"
}

// openOutlet menerjemahkan slug ke outlet aktif. ID hanya dipakai di dalam.
func openOutlet(slug string) (id string, ref models.OpenOutletRef, err error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return "", ref, ErrOpenOutletNotFound
	}
	err = database.DB.QueryRow(`SELECT id, name, slug FROM outlets WHERE slug = $1 AND is_active = true`, slug).
		Scan(&id, &ref.Name, &ref.Slug)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ref, ErrOpenOutletNotFound
	}
	return id, ref, err
}

// ListOpenOutlets = semua outlet aktif yang punya slug, urut nama.
func ListOpenOutlets() ([]models.OpenOutletRef, error) {
	rows, err := database.DB.Query(`SELECT name, slug FROM outlets
		WHERE is_active = true AND COALESCE(slug,'') <> '' ORDER BY lower(name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.OpenOutletRef{}
	for rows.Next() {
		var o models.OpenOutletRef
		if err := rows.Scan(&o.Name, &o.Slug); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetOpenProducts = produk aktif satu outlet: nama, kategori, jenis, harga.
// Kategori dirapikan spasinya; kosong menjadi "Tidak Berkategori" (sama dengan
// laporan penjualan produk). Jenis memakai sumber yang sama dengan halaman
// Produk (product_menu_kinds).
func GetOpenProducts(slug string) (*models.OpenProductList, error) {
	outletID, ref, err := openOutlet(slug)
	if err != nil {
		return nil, err
	}
	rows, err := database.DB.Query(`
		SELECT name, cat, kind, price FROM (
			SELECT btrim(p.name) AS name,
			       COALESCE(NULLIF(regexp_replace(btrim(COALESCE(p.category_name,'')),'\s+',' ','g'),''),'Tidak Berkategori') AS cat,
			       COALESCE(mk.menu_kind,'') AS kind, p.price
			FROM cloud_products p
			LEFT JOIN product_menu_kinds mk ON mk.product_id = p.id
			WHERE p.outlet_id = $1 AND COALESCE(p.is_deleted,false) = false
		) x
		ORDER BY lower(cat), lower(name), price`, outletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &models.OpenProductList{Outlet: ref, Products: []models.OpenProduct{}}
	for rows.Next() {
		var p models.OpenProduct
		if err := rows.Scan(&p.Name, &p.Category, &p.Kind, &p.Price); err != nil {
			return nil, err
		}
		p.KindLabel = openKindLabel(p.Kind)
		res.Products = append(res.Products, p)
	}
	res.Total = len(res.Products)
	return res, rows.Err()
}

// GetOpenProductSales memakai GetProductSalesReport apa adanya supaya angkanya
// identik dengan menu Laporan Penjualan Produk di aplikasi (basis tanggal order
// zona aplikasi, tanpa order void/tahan; nilai = harga × qty sebelum diskon,
// kompliment, dan pajak).
func GetOpenProductSales(slug, dateFrom, dateTo string) (*models.OpenProductSales, error) {
	outletID, ref, err := openOutlet(slug)
	if err != nil {
		return nil, err
	}
	rep, err := GetProductSalesReport(dateFrom, dateTo, outletID, "revenue", "desc", nil, 1, 100000)
	if err != nil {
		return nil, err
	}
	res := &models.OpenProductSales{Outlet: ref, DateFrom: dateFrom, DateTo: dateTo,
		TotalQty: rep.TotalQty, TotalRevenue: rep.TotalRevenue, Items: []models.OpenProductSalesItem{}}
	// Spasi kategori dirapikan seperti endpoint produk; baris yang jadi kembar
	// ("JAPANESE FOOD" vs "JAPANESE FOOD ") digabung.
	at := map[[2]string]int{}
	for _, r := range rep.Items {
		cat := strings.Join(strings.Fields(r.CategoryName), " ")
		if cat == "" {
			cat = "Tidak Berkategori"
		}
		key := [2]string{r.ProductName, cat}
		if i, ok := at[key]; ok {
			res.Items[i].Qty += r.TotalQty
			res.Items[i].Revenue += r.TotalRevenue
			continue
		}
		at[key] = len(res.Items)
		res.Items = append(res.Items, models.OpenProductSalesItem{
			Name: r.ProductName, Category: cat, Qty: r.TotalQty, Revenue: r.TotalRevenue,
		})
	}
	sort.SliceStable(res.Items, func(i, j int) bool { return res.Items[i].Revenue > res.Items[j].Revenue })
	return res, nil
}

// GetOpenSocial = akun Instagram & TikTok aktif semua outlet aktif: pembacaan
// terakhir + deret harian dalam rentang [dateFrom, dateTo] (captured_date).
// Outlet tanpa akun tetap tampil dengan accounts kosong.
func GetOpenSocial(dateFrom, dateTo string) (*models.OpenSocial, error) {
	res := &models.OpenSocial{DateFrom: dateFrom, DateTo: dateTo, Outlets: []models.OpenSocialOutlet{}}
	outlets, err := ListOpenOutlets()
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for _, o := range outlets {
		idx[o.Slug] = len(res.Outlets)
		res.Outlets = append(res.Outlets, models.OpenSocialOutlet{Name: o.Name, Slug: o.Slug, Accounts: []models.OpenSocialAccount{}})
	}

	rows, err := database.DB.Query(`
		SELECT a.id, o.slug, a.platform, a.username, a.profile_url,
		       s.captured_date, s.followers, s.following, s.posts_count, s.likes_total, COALESCE(s.approx,false)
		FROM social_accounts a
		JOIN outlets o ON o.id = a.outlet_id AND o.is_active = true AND COALESCE(o.slug,'') <> ''
		LEFT JOIN LATERAL (
			SELECT captured_date, followers, following, posts_count, likes_total, approx
			FROM social_snapshots WHERE account_id = a.id AND followers IS NOT NULL
			ORDER BY captured_date DESC LIMIT 1
		) s ON true
		WHERE a.is_active = true AND a.platform IN ('instagram','tiktok')
		ORDER BY lower(o.name), a.platform, lower(a.username)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type pos struct{ outlet, account int }
	byAccount := map[string]pos{} // ID akun hanya kunci internal
	var accountIDs []string
	for rows.Next() {
		var id, slug string
		var acc models.OpenSocialAccount
		var captured sql.NullTime
		var p models.OpenSocialPoint
		if err := rows.Scan(&id, &slug, &acc.Platform, &acc.Username, &acc.ProfileURL,
			&captured, &p.Followers, &p.Following, &p.Posts, &p.Likes, &p.Approx); err != nil {
			return nil, err
		}
		oi, ok := idx[slug]
		if !ok {
			continue
		}
		if captured.Valid {
			p.Date = captured.Time.Format("2006-01-02")
			acc.Latest = &p
		}
		acc.Daily = []models.OpenSocialPoint{}
		id = strings.TrimSpace(id)
		byAccount[id] = pos{oi, len(res.Outlets[oi].Accounts)}
		accountIDs = append(accountIDs, id)
		res.Outlets[oi].Accounts = append(res.Outlets[oi].Accounts, acc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(accountIDs) == 0 {
		return res, nil
	}

	snaps, err := database.DB.Query(`
		SELECT account_id, captured_date, followers, following, posts_count, likes_total, approx
		FROM social_snapshots
		WHERE account_id = ANY($1::text[]) AND captured_date BETWEEN $2::date AND $3::date
		ORDER BY account_id, captured_date`, pq.Array(accountIDs), dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	defer snaps.Close()
	for snaps.Next() {
		var id string
		var d time.Time
		var p models.OpenSocialPoint
		if err := snaps.Scan(&id, &d, &p.Followers, &p.Following, &p.Posts, &p.Likes, &p.Approx); err != nil {
			return nil, err
		}
		at, ok := byAccount[strings.TrimSpace(id)]
		if !ok {
			continue
		}
		p.Date = d.Format("2006-01-02")
		acc := &res.Outlets[at.outlet].Accounts[at.account]
		acc.Daily = append(acc.Daily, p)
	}
	if err := snaps.Err(); err != nil {
		return nil, err
	}
	for oi := range res.Outlets {
		for ai := range res.Outlets[oi].Accounts {
			acc := &res.Outlets[oi].Accounts[ai]
			var first, last int64
			n := 0
			for _, p := range acc.Daily {
				if p.Followers == nil {
					continue
				}
				if n == 0 {
					first = *p.Followers
				}
				last = *p.Followers
				n++
			}
			if n >= 2 {
				diff := last - first
				acc.FollowersChange = &diff
			}
		}
	}
	return res, nil
}
