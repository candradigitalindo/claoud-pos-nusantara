package services

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestScrapeLive menembak halaman profil SUNGGUHAN. Dilewati kecuali dijalankan
// dengan sengaja, karena test yang menyentuh jaringan tidak boleh ikut menahan
// build: ia gagal karena hal-hal di luar kode ini — blokir platform, jaringan
// server, bentuk halaman yang berubah semalam.
//
// Justru karena sebab-sebab itulah alat ini disimpan di repo dan bukan dibuang
// setelah dipakai sekali: ketika suatu hari angkanya berhenti masuk, inilah cara
// tercepat memisahkan "penariknya rusak" dari "platformnya berubah".
//
//	SOCIAL_LIVE_TEST=tiktok:nusantara_healing_chill,instagram:nama.akun \
//	  go test ./services/ -run TestScrapeLive -v
func TestScrapeLive(t *testing.T) {
	spec := strings.TrimSpace(os.Getenv("SOCIAL_LIVE_TEST"))
	if spec == "" {
		t.Skip("lewati: setel SOCIAL_LIVE_TEST=platform:username[,platform:username]")
	}

	for _, bagian := range strings.Split(spec, ",") {
		platform, username, ok := strings.Cut(strings.TrimSpace(bagian), ":")
		if !ok {
			t.Fatalf("format salah: %q (mau platform:username)", bagian)
		}

		p, err := scrapeProfile(platform, username)
		if err != nil {
			t.Errorf("%s @%s GAGAL: %v", platform, username, err)
			continue
		}

		ringkas := map[string]any{
			"via": p.Via, "approx": p.Approx,
			"followers": p.Followers, "following": p.Following,
			"posts": p.PostsCount, "likes_total": p.LikesTotal,
			"konten_terbaca": len(p.Posts),
		}
		b, _ := json.Marshal(ringkas)
		fmt.Printf("  %s @%s -> %s\n", platform, username, b)

		if p.Followers == nil {
			t.Errorf("%s @%s: jumlah pengikut tidak terbaca", platform, username)
		}
	}
}
