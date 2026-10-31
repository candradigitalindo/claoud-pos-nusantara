package services

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Alur tagihan QRIS, dari sisi kasir:
//
//  1. POS memanggil CreateQRISCharge saat kasir memilih metode QRIS. Baris
//     tagihan tersimpan berstatus pending dan QR-nya dikirim balik.
//  2. Tamu memindai lalu membayar. Penyedia memanggil webhook kita, yang
//     memanggil MarkQRISChargePaid.
//  3. POS mem-polling GetQRISCharge sampai statusnya paid, lalu barulah
//     mencatat pembayaran seperti biasa (tunai/kartu) ke shift berjalan.
//
// POS tetap yang mencatat pembayarannya sendiri — cloud hanya memastikan uang
// benar-benar masuk. Dengan begitu tagihan QRIS tidak pernah membuat pembayaran
// muncul di cloud tanpa pasangannya di laporan shift kasir.

// CreateQRISCharge membuat tagihan baru lewat penyedia aktif dan menyimpannya.
//
// Bila untuk order yang sama masih ada tagihan pending yang belum kedaluwarsa,
// tagihan ITU yang dikembalikan alih-alih membuat yang baru — kasir yang tidak
// sengaja menekan "QRIS" dua kali tidak menagih tamu dua kali.
func CreateQRISCharge(outletID, orderLocalID string, amount float64, description string) (*models.QRISChargeResponse, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("nominal tagihan harus lebih dari 0")
	}

	gw, err := ActiveGateway()
	if err != nil {
		return nil, err
	}
	if existing, err := findReusableCharge(outletID, orderLocalID, amount, gw.Name()); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	chargeID := newChargeID()
	res, err := gw.CreateQRIS(QRISCharge{
		ChargeID:    chargeID,
		OutletID:    outletID,
		Amount:      amount,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("penyedia menolak tagihan: %w", err)
	}

	_, err = database.DB.Exec(
		`INSERT INTO qris_charges (id, outlet_id, order_local_id, amount, provider,
			provider_ref, qr_string, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8, (now() AT TIME ZONE 'UTC'), (now() AT TIME ZONE 'UTC'))`,
		chargeID, outletID, nullStr(orderLocalID), amount, gw.Name(),
		res.ProviderRef, res.QRString, res.ExpiresAt.UTC(),
	)
	if err != nil {
		return nil, err
	}

	go logSync(outletID, "qris_charge_created", "qris_charge", 1, "success", "")

	return &models.QRISChargeResponse{
		ChargeID:  chargeID,
		Provider:  gw.Name(),
		QRString:  res.QRString,
		Amount:    amount,
		Status:    QRISPending,
		ExpiresAt: res.ExpiresAt.UTC().Format(time.RFC3339),
	}, nil
}

// findReusableCharge mencari tagihan pending yang masih berlaku untuk order &
// nominal yang sama. Nominal ikut dicocokkan supaya perubahan tagihan (item
// ditambah setelah QR terbit) tetap menerbitkan QR baru.
func findReusableCharge(outletID, orderLocalID string, amount float64, provider string) (*models.QRISChargeResponse, error) {
	if orderLocalID == "" {
		return nil, nil
	}
	var r models.QRISChargeResponse
	var expires time.Time
	err := database.DB.QueryRow(
		`SELECT id, provider, qr_string, amount, status, expires_at
		FROM qris_charges
		WHERE outlet_id = $1 AND order_local_id = $2 AND status = 'pending'
			AND amount = $3 AND provider = $4 AND expires_at > (now() AT TIME ZONE 'UTC')
		ORDER BY created_at DESC LIMIT 1`,
		outletID, orderLocalID, amount, provider,
	).Scan(&r.ChargeID, &r.Provider, &r.QRString, &r.Amount, &r.Status, &expires)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.ExpiresAt = expires.UTC().Format(time.RFC3339)
	return &r, nil
}

// GetQRISCharge mengembalikan status terkini sebuah tagihan — endpoint yang
// di-polling POS sambil menunggu tamu membayar.
//
// Tagihan pending yang sudah lewat waktu ditandai expired di sini, sehingga
// polling berhenti walau penyedia tidak pernah mengirim apa pun.
func GetQRISCharge(outletID, chargeID string) (*models.QRISChargeResponse, error) {
	var r models.QRISChargeResponse
	var expires time.Time
	var paidAt sql.NullTime
	err := database.DB.QueryRow(
		`SELECT id, provider, qr_string, amount, status, expires_at, paid_at
		FROM qris_charges WHERE outlet_id = $1 AND id = $2`,
		outletID, chargeID,
	).Scan(&r.ChargeID, &r.Provider, &r.QRString, &r.Amount, &r.Status, &expires, &paidAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tagihan tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	if r.Status == QRISPending && time.Now().UTC().After(expires.UTC()) {
		if _, uerr := database.DB.Exec(
			`UPDATE qris_charges SET status = 'expired', updated_at = (now() AT TIME ZONE 'UTC')
			WHERE id = $1 AND status = 'pending'`, chargeID,
		); uerr != nil {
			log.Printf("tandai qris_charge %s expired: %v", chargeID, uerr)
		} else {
			r.Status = QRISExpired
		}
	}

	r.ExpiresAt = expires.UTC().Format(time.RFC3339)
	if paidAt.Valid {
		r.PaidAt = paidAt.Time.UTC().Format(time.RFC3339)
	}
	return &r, nil
}

// MarkQRISChargePaid mencatat hasil dari webhook penyedia.
//
// Idempoten dan sekali-jalan: hanya baris yang MASIH pending yang berubah, jadi
// callback ganda (penyedia mengulang kiriman) tidak pernah menghasilkan
// pembayaran dobel, dan tagihan yang sudah lunas tidak bisa diturunkan lagi
// menjadi gagal oleh callback yang datang terlambat.
func MarkQRISChargePaid(chargeID, status string) error {
	return markQRISChargeStatus(chargeID, status, "", "")
}

// MarkQRISChargePaidForProvider binds a verified callback to its own provider.
func MarkQRISChargePaidForProvider(provider, chargeID, status string) error {
	if provider == "" {
		return fmt.Errorf("penyedia pembayaran wajib diisi")
	}
	return markQRISChargeStatus(chargeID, status, provider, "")
}

// SimulateQRISChargePaid cannot settle a real charge or another outlet's charge.
func SimulateQRISChargePaid(outletID, chargeID string) error {
	if !MockSimulationEnabled() {
		return fmt.Errorf("simulasi QRIS tidak aktif")
	}
	if outletID == "" {
		return fmt.Errorf("outlet wajib diisi")
	}
	return markQRISChargeStatus(chargeID, QRISPaid, "mock", outletID)
}

// The status and online-order promotion commit together. A failed promotion is
// retried by the provider instead of acknowledging a payment with a lost order.
func markQRISChargeStatus(chargeID, status, provider, outletID string) error {
	if status != QRISPaid && status != QRISExpired && status != QRISFailed {
		return nil
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current, chargeOutlet string
	var amount float64
	err = tx.QueryRow(`SELECT status, outlet_id, amount FROM qris_charges
		WHERE id=$1 AND ($2='' OR provider=$2) AND ($3='' OR outlet_id=$3)
		FOR UPDATE`, chargeID, provider, outletID).Scan(&current, &chargeOutlet, &amount)
	if err == sql.ErrNoRows {
		return fmt.Errorf("tagihan tidak ditemukan untuk outlet/penyedia ini")
	}
	if err != nil {
		return err
	}
	if current == QRISPending {
		var paidAt interface{}
		if status == QRISPaid {
			paidAt = time.Now().UTC()
		}
		if _, err = tx.Exec(`UPDATE qris_charges SET status=$1, paid_at=$2,
			updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$3`, status, paidAt, chargeID); err != nil {
			return err
		}
		current = status
	}
	if current == QRISPaid {
		if _, err = tx.Exec(`UPDATE online_orders SET status='new', paid_amount=$2,
			updated_at=(now() AT TIME ZONE 'UTC')
			WHERE qris_charge_id=$1 AND outlet_id=$3 AND status='awaiting_payment'`,
			chargeID, amount, chargeOutlet); err != nil {
			return err
		}
	}
	return tx.Commit()
}
