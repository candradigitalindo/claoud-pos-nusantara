package services

import (
	"cloud-pos/database"
	"github.com/lib/pq"
)

func RecipeMasterInScope(id string, scope []string) bool {
	var ok bool
	err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM recipe_masters r WHERE id=$1 AND ($2 OR visibility='public'
		OR EXISTS(SELECT 1 FROM recipe_outlet_access a WHERE a.recipe_master_id=r.id AND a.outlet_id=ANY($3))))`, id, scope == nil, pq.Array(scope)).Scan(&ok)
	return err == nil && ok
}
