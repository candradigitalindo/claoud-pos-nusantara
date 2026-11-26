package services

import (
	"errors"
	"testing"

	"cloud-pos/models"
)

func TestApplyApproverQtyEditUnit(t *testing.T) {
	stored := []models.PurchaseRequestItem{{
		Name: "Kebutuhan Outlet",
		Items: []models.PurchaseSubItem{
			{Name: "Piring Keramik", Qty: 10, Unit: "pcs", HpsPrice: 25000, HpsSource: "Toko A", GoodsKind: "perlengkapan"},
			{Name: "Sendok Makan", Qty: 20, Unit: "pcs", HpsPrice: 5000, HpsSource: "Toko A", GoodsKind: "perlengkapan"},
		},
	}}

	// Penyetuju menurunkan qty item pertama 10 -> 6, item kedua tetap 20.
	inc := clonePurchaseItems(stored)
	inc[0].Items[0].Qty = 6
	inc[0].Items[0].FinalPrice = 999999 // harga final harus diabaikan
	out, err := applyApproverQtyEdit(stored, inc)
	if err != nil {
		t.Fatalf("applyApproverQtyEdit valid: %v", err)
	}
	if out[0].Items[0].Qty != 6 || out[0].Items[0].RequestedQty == nil || *out[0].Items[0].RequestedQty != 10 {
		t.Fatalf("item 0 qty/requested_qty = %d/%v, mau 6/10", out[0].Items[0].Qty, out[0].Items[0].RequestedQty)
	}
	if out[0].Items[0].FinalPrice != 0 {
		t.Errorf("final price harus dikosongkan: %v", out[0].Items[0].FinalPrice)
	}
	if out[0].Items[1].RequestedQty != nil {
		t.Errorf("item tidak diubah tidak boleh punya requested_qty: %v", out[0].Items[1].RequestedQty)
	}

	// Perubahan kedua (6 -> 8): requested_qty tetap 10 (qty awal pengaju).
	inc2 := clonePurchaseItems(out)
	inc2[0].Items[0].Qty = 8
	out2, err := applyApproverQtyEdit(out, inc2)
	if err != nil {
		t.Fatalf("edit kedua: %v", err)
	}
	if out2[0].Items[0].Qty != 8 || out2[0].Items[0].RequestedQty == nil || *out2[0].Items[0].RequestedQty != 10 {
		t.Fatalf("edit kedua qty/requested_qty = %d/%v, mau 8/10", out2[0].Items[0].Qty, out2[0].Items[0].RequestedQty)
	}

	// Kembalikan ke qty asli (10): requested_qty bersih kembali ke nil.
	inc3 := clonePurchaseItems(out2)
	inc3[0].Items[0].Qty = 10
	out3, err := applyApproverQtyEdit(out2, inc3)
	if err != nil {
		t.Fatalf("kembalikan qty asli: %v", err)
	}
	if out3[0].Items[0].Qty != 10 || out3[0].Items[0].RequestedQty != nil {
		t.Fatalf("kembalikan asli qty/requested_qty = %d/%v, mau 10/nil", out3[0].Items[0].Qty, out3[0].Items[0].RequestedQty)
	}

	// Mengubah HPS / nama / satuan / menghapus baris harus ditolak.
	var ve ValidationError
	badHps := clonePurchaseItems(stored)
	badHps[0].Items[0].HpsPrice = 30000
	if _, err := applyApproverQtyEdit(stored, badHps); !errors.As(err, &ve) {
		t.Errorf("ubah HPS oleh penyetuju harus ditolak: %v", err)
	}
	badName := clonePurchaseItems(stored)
	badName[0].Items[0].Name = "Piring Lain"
	if _, err := applyApproverQtyEdit(stored, badName); !errors.As(err, &ve) {
		t.Errorf("ubah nama oleh penyetuju harus ditolak: %v", err)
	}
	badDel := clonePurchaseItems(stored)
	badDel[0].Items = badDel[0].Items[:1]
	if _, err := applyApproverQtyEdit(stored, badDel); !errors.As(err, &ve) {
		t.Errorf("hapus baris oleh penyetuju harus ditolak: %v", err)
	}
}

func TestApproverCanAdjustQtyOnVerifiedBarangAndApprove(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, verifItems())
	ver, err := UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Tanpa flag ApproverQtyOnly (jalur pengaju), barang terverifikasi tetap terkunci.
	var ve ValidationError
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: ver.Items, Version: ver.Version}); !errors.As(err, &ve) {
		t.Fatalf("jalur non-approver harus tetap ditolak: %v", err)
	}

	// Jalur penyetuju (ApproverQtyOnly=true): boleh ubah qty, HPS terhitung ulang,
	// requested_qty menyimpan angka awal pengaju.
	origQty := ver.Items[0].Items[0].Qty
	newQty := origQty + 2
	adj := clonePurchaseItems(ver.Items)
	adj[0].Items[0].Qty = newQty
	updated, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{
		Items: adj, Version: ver.Version, ApproverQtyOnly: true, ActorName: "Direktur",
	})
	if err != nil {
		t.Fatalf("approver adjust qty: %v", err)
	}
	if updated.Items[0].Items[0].Qty != newQty {
		t.Fatalf("qty=%d, mau %d", updated.Items[0].Items[0].Qty, newQty)
	}
	if updated.Items[0].Items[0].RequestedQty == nil || *updated.Items[0].Items[0].RequestedQty != origQty {
		t.Fatalf("requested_qty=%v, mau %d", updated.Items[0].Items[0].RequestedQty, origQty)
	}
	wantHps, _ := recalcItems(clonePurchaseItems(adj))
	if updated.TotalHps != wantHps || updated.TotalAmount != wantHps {
		t.Fatalf("total_hps/amount = %.0f/%.0f, mau %.0f", updated.TotalHps, updated.TotalAmount, wantHps)
	}

	// Setelah disetujui, qty yang disetujui tetap berlaku untuk tahap Purchasing.
	approved, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "approve", ActorName: "Direktur", Version: updated.Version,
	})
	if err != nil {
		t.Fatalf("approve setelah ubah qty: %v", err)
	}
	if approved.Status != "approved" || approved.Items[0].Items[0].Qty != newQty || approved.TotalHps != wantHps {
		t.Fatalf("setelah approve: status=%s qty=%d hps=%.0f", approved.Status, approved.Items[0].Items[0].Qty, approved.TotalHps)
	}
}
