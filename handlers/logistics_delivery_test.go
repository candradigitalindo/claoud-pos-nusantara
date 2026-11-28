package handlers

import (
	"reflect"
	"testing"

	"cloud-pos/services"
)

func TestLogisticsViewGroupsOutletIsolation(t *testing.T) {
	cases := []struct {
		name         string
		acc          purchaseAccess
		outletScoped bool
		want         []string
	}{
		{
			name:         "outlet manager with stockledger.adjust + logistics.goods.view stays locked to barang_jasa",
			acc:          purchaseAccess{perms: []string{"stockledger.adjust", "logistics.goods.view", "procurement.requests.submit"}},
			outletScoped: true,
			want:         []string{services.PurchasingGroupBarangJasa},
		},
		{
			name:         "central warehouse with stockledger.adjust sees bahan_baku",
			acc:          purchaseAccess{perms: []string{"stockledger.adjust", "procurement.rawmaterials.submit"}},
			outletScoped: false,
			want:         []string{services.PurchasingGroupBahanBaku},
		},
		{
			name:         "specific-scoped role with explicit logistics.rawmaterials.view still sees bahan_baku",
			acc:          purchaseAccess{perms: []string{"stockledger.adjust", "logistics.rawmaterials.view"}},
			outletScoped: true,
			want:         []string{services.PurchasingGroupBahanBaku},
		},
		{
			name:         "superadmin sees all groups",
			acc:          purchaseAccess{superadmin: true},
			outletScoped: true,
			want:         nil,
		},
		{
			name:         "combined logistics.deliveries.view sees all groups",
			acc:          purchaseAccess{perms: []string{"logistics.deliveries.view"}},
			outletScoped: false,
			want:         nil,
		},
		{
			name:         "finance without logistics perms sees no groups",
			acc:          purchaseAccess{perms: []string{"finance.payments.view", "finance.payments.pay"}},
			outletScoped: false,
			want:         []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := logisticsViewGroups(tc.acc, tc.outletScoped)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("logisticsViewGroups() = %v, want %v", got, tc.want)
			}
		})
	}
}
