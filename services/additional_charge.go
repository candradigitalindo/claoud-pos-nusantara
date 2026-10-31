package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
)

// Biaya tambahan outlet (pajak/PB1, service charge) — cerminan tabel yang sama
// di POS. Cloud tidak pernah mengubahnya; POS satu-satunya sumber kebenaran.

// SaveAdditionalCharge menyimpan satu biaya dari sync batch POS.
func SaveAdditionalCharge(outletID string, req models.PushAdditionalChargeRequest) (string, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	cloudID, err := reservePOSID(tx, "additional_charge", outletID, req.LocalID)
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(
		`INSERT INTO cloud_additional_charges (id, local_id, outlet_id, name,
			charge_type, value, is_active, version, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (outlet_id, local_id) DO UPDATE SET
			name = EXCLUDED.name,
			charge_type = EXCLUDED.charge_type,
			value = EXCLUDED.value,
			is_active = EXCLUDED.is_active,
			version = EXCLUDED.version,
			updated_at = NOW()
		RETURNING id`,
		cloudID, req.LocalID, outletID, req.Name, req.ChargeType, req.Value,
		req.IsActive != 0, req.Version,
	).Scan(&cloudID)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	// Sengaja TIDAK dicatat ke sync_logs: POS mengirim ulang daftar ini tiap
	// siklus untuk menutup celah drift, jadi mencatatnya hanya membanjiri log.
	return cloudID, nil
}

// ActiveCharges mengembalikan biaya aktif outlet, urut sesuai POS (nama).
func ActiveCharges(outletID string) ([]models.CloudAdditionalCharge, error) {
	rows, err := database.DB.Query(
		`SELECT local_id, name, charge_type, value
		FROM cloud_additional_charges
		WHERE outlet_id = $1 AND is_active = true
		ORDER BY name ASC`,
		outletID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.CloudAdditionalCharge, 0)
	for rows.Next() {
		var c models.CloudAdditionalCharge
		if err := rows.Scan(&c.LocalID, &c.Name, &c.ChargeType, &c.Value); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CalculateOrderTotal menghitung total tagihan dari subtotal.
//
// SENGAJA mencerminkan _applyAutoCharges di POS: biaya persentase dihitung
// dari subtotal, biaya tetap ditambahkan apa adanya, lalu semuanya dijumlahkan
// ke subtotal. Pesanan online tidak mengenal diskon manual, jadi basisnya
// adalah subtotal penuh.
//
// Bila rumus di POS berubah, rumus di sini HARUS ikut berubah — kalau tidak,
// tamu membayar nominal yang berbeda dari tagihan yang dihitung kasir.
func CalculateOrderTotal(subtotal float64, charges []models.CloudAdditionalCharge) (float64, []models.ChargeLine) {
	lines := make([]models.ChargeLine, 0, len(charges))
	total := subtotal
	for _, c := range charges {
		var applied float64
		if subtotal > 0 {
			if c.ChargeType == "percentage" {
				applied = subtotal * c.Value / 100
			} else {
				applied = c.Value
			}
		}
		lines = append(lines, models.ChargeLine{Name: c.Name, Amount: applied})
		total += applied
	}
	if total < 0 {
		total = 0
	}
	return total, lines
}
