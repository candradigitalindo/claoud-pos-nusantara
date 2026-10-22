package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"github.com/lib/pq"
)

func GetFnbOutletPolicy(outlet string, scope []string) (models.FnbOutletPolicy, error) {
	r := models.FnbOutletPolicy{OutletID: outlet}
	if _, err := fnbScope(outlet, scope); err != nil {
		return r, err
	}
	if outlet == "" {
		return r, Invalid("pilih outlet terlebih dahulu")
	}
	err := database.DB.QueryRow(`SELECT o.name,COALESCE(p.target_cost_pct,35),p.outlet_id IS NOT NULL
		FROM outlets o LEFT JOIN fnb_outlet_policies p ON p.outlet_id=o.id WHERE o.id=$1 AND o.is_active`, outlet).Scan(&r.OutletName, &r.TargetCostPct, &r.Configured)
	return r, err
}

func SaveFnbOutletPolicy(outlet string, req models.FnbOutletPolicy, scope []string, actor string) error {
	if outlet == "" || !fnbFinite(req.TargetCostPct) || req.TargetCostPct <= 0 || req.TargetCostPct > 100 {
		return Invalid("outlet dan batas HPP (lebih dari 0 hingga 100%%) harus valid")
	}
	if _, err := fnbScope(outlet, scope); err != nil {
		return err
	}
	res, err := database.DB.Exec(`INSERT INTO fnb_outlet_policies(outlet_id,target_cost_pct,updated_by)
		SELECT id,$2,$3 FROM outlets WHERE id=$1 AND is_active AND ($4 OR id=ANY($5))
		ON CONFLICT(outlet_id) DO UPDATE SET target_cost_pct=$2,updated_by=$3,updated_at=NOW()`, outlet, req.TargetCostPct, actor, scope == nil, pq.Array(scope))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrFnbScope
	}
	return nil
}
