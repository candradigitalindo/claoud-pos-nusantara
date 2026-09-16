package services

import "testing"

// Interval perawatan harus sama di setiap pemanggilan: sebelum diperbaiki,
// urutan map yang acak membuat AC kadang dijadwalkan 3 bulan, kadang 12.
func TestMaintenanceIntervalDeterministic(t *testing.T) {
	cases := []struct {
		category, name string
		want           int
	}{
		{"Elektronik AC", "AC Daikin 1PK", 3},
		{"Mesin", "Mesin Kopi La Marzocco", 1},
		{"Elektronik", "Kulkas Sharp", 6},
		{"Elektronik", "Printer Kasir", 12},
		{"Mebel", "Rack Penyimpanan", 0}, // 'ac' di dalam "Rack" tidak boleh cocok
		{"Mebel", "Meja Kayu", 0},
	}
	for _, c := range cases {
		for i := 0; i < 50; i++ {
			if got := maintenanceIntervalMonths(c.category, c.name); got != c.want {
				t.Fatalf("%s/%s: dapat %d bulan, harusnya %d (percobaan ke-%d)", c.category, c.name, got, c.want, i)
			}
		}
	}
}
