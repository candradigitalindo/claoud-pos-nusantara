package services

import (
	"cloud-pos/database"
	"strings"
)

func ClassifyFinanceCash(id, account, notes string, a FinanceAccess) error {
	if !a.Has("finance.entries.post") {
		return ErrFinanceAccess
	}
	var oid, kind string
	if err := database.DB.QueryRow(`SELECT outlet_id,movement_type FROM cloud_cash_movements WHERE id=$1`, id).Scan(&oid, &kind); err != nil {
		return err
	}
	if !financeInScope(strings.TrimSpace(oid), a.Outlets) {
		return ErrFinanceAccess
	}
	direction := "out"
	if statusIn(strings.ToLower(kind), []string{"masuk", "in", "income", "pemasukan"}) {
		direction = "in"
	}
	meta, ok := financeAccount(account)
	if !ok || meta.Direction != direction && meta.Direction != "both" {
		return Invalid("akun tidak sesuai arah kas")
	}
	notes = strings.TrimSpace(notes)
	if notes == "" || len(notes) > 10000 {
		return Invalid("alasan klasifikasi wajib diisi")
	}
	_, err := database.DB.Exec(`INSERT INTO finance_cash_classifications(id,movement_id,account_code,notes,actor) VALUES($1,$2,$3,$4,$5)`, NewULID(), id, account, notes, a.Actor)
	return err
}
