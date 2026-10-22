package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"math"
	"strings"

	"github.com/lib/pq"
)

var ErrFnbScope = errors.New("data di luar cakupan outlet Anda")

func fnbScope(outlet string, scope []string) ([]string, error) {
	if outlet == "" {
		return scope, nil
	}
	if scope != nil {
		found := false
		for _, id := range scope {
			if id == outlet {
				found = true
			}
		}
		if !found {
			return nil, ErrFnbScope
		}
	}
	return []string{outlet}, nil
}

func validMenuKind(kind string, optional bool) bool {
	return kind == "food" || kind == "beverage" || (optional && kind == "")
}

func fnbFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func SaveFnbProfile(productID string, req models.FnbProfile, scope []string, actor string) error {
	if !validMenuKind(req.MenuKind, true) || !fnbFinite(req.TargetCostPct) || req.TargetCostPct <= 0 || req.TargetCostPct > 100 ||
		!fnbFinite(req.PackagingCost) || req.PackagingCost < 0 || !fnbFinite(req.OverheadCost) || req.OverheadCost < 0 {
		return Invalid("kategori, target HPP (0–100%%), dan biaya tambahan harus valid")
	}
	res, err := database.DB.Exec(`INSERT INTO fnb_product_profiles(product_id,menu_kind,target_cost_pct,packaging_cost,overhead_cost,updated_by)
		SELECT id,$2,$3,$4,$5,$6 FROM cloud_products WHERE id=$1 AND NOT is_deleted AND ($7 OR outlet_id=ANY($8))
		ON CONFLICT(product_id) DO UPDATE SET menu_kind=$2,target_cost_pct=$3,packaging_cost=$4,overhead_cost=$5,updated_by=$6,updated_at=NOW()`,
		productID, req.MenuKind, req.TargetCostPct, req.PackagingCost, req.OverheadCost, actor, scope == nil, pq.Array(scope))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrFnbScope
	}
	return nil
}

// GetFnbCosting calculates current standard menu cost, not historical accounting COGS.
// Prices use the product outlet's inventory valuation, with central as fallback.
func GetFnbCosting(outlet string, scope []string) ([]models.FnbCostRow, error) {
	filter, err := fnbScope(outlet, scope)
	if err != nil {
		return nil, err
	}
	type product struct {
		row               models.FnbCostRow
		stockType, linked string
		allowed           bool
	}
	products := []product{}
	rows, err := database.DB.Query(`SELECT p.id,p.name,p.outlet_id,o.name,COALESCE(p.category_name,''),p.price,
		COALESCE(p.stock_type,'none'),COALESCE(p.linked_stock_item_id,''),COALESCE(p.recipe_master_id,''),
		COALESCE(f.menu_kind,r.menu_kind,''),COALESCE(op.target_cost_pct,f.target_cost_pct,35),COALESCE(f.packaging_cost,0),COALESCE(f.overhead_cost,0),
		op.target_cost_pct,CASE WHEN op.outlet_id IS NOT NULL THEN 'outlet' WHEN f.product_id IS NOT NULL THEN 'menu' ELSE 'default' END,
		($3 OR (NOT EXISTS(SELECT 1 FROM product_recipe_sources ps JOIN recipe_masters rm ON rm.id=ps.recipe_master_id
			WHERE ps.product_id=p.id AND rm.visibility<>'public' AND NOT EXISTS(SELECT 1 FROM recipe_outlet_access a WHERE a.recipe_master_id=rm.id AND a.outlet_id=p.outlet_id))
			AND NOT EXISTS(SELECT 1 FROM product_recipes pr WHERE pr.product_id=p.id AND pr.visibility<>'public'
				AND p.stock_type='recipe' AND NOT EXISTS(SELECT 1 FROM product_recipe_sources ps WHERE ps.product_id=p.id))))
		FROM cloud_products p JOIN outlets o ON o.id=p.outlet_id
		LEFT JOIN fnb_product_profiles f ON f.product_id=p.id LEFT JOIN fnb_outlet_policies op ON op.outlet_id=p.outlet_id
		LEFT JOIN LATERAL (SELECT rm.menu_kind FROM product_recipe_sources ps JOIN recipe_masters rm ON rm.id=ps.recipe_master_id
			WHERE ps.product_id=p.id ORDER BY ps.position,rm.id LIMIT 1) r ON true
		WHERE NOT p.is_deleted AND o.is_active AND ($1 OR p.outlet_id=ANY($2)) ORDER BY o.name,p.name`, filter == nil, pq.Array(filter), scope == nil)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var p product
		r := &p.row
		if err := rows.Scan(&r.ProductID, &r.ProductName, &r.OutletID, &r.OutletName, &r.Category, &r.Price, &p.stockType, &p.linked, &r.RecipeID,
			&r.MenuKind, &r.TargetCostPct, &r.PackagingCost, &r.OverheadCost, &r.OutletTargetCostPct, &r.TargetSource, &p.allowed); err != nil {
			rows.Close()
			return nil, err
		}
		products = append(products, p)
		ids = append(ids, r.ProductID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := []models.FnbCostRow{}
	if len(ids) == 0 {
		return result, nil
	}
	type material struct {
		id  string
		qty float64
	}
	mats := map[string][]material{}
	rows, err = database.DB.Query(`SELECT product_id,item_id,qty_base FROM product_recipe_materials WHERE product_id=ANY($1) ORDER BY product_id,item_id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var m material
		if err := rows.Scan(&id, &m.id, &m.qty); err != nil {
			rows.Close()
			return nil, err
		}
		mats[id] = append(mats[id], m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	catalog, err := loadRecipeCostCatalog(database.DB, filter)
	if err != nil {
		return nil, err
	}
	infos, wip, unitCost := catalog.infos, catalog.wip, catalog.unitCost
	links, err := loadProductRecipeLinks(ids)
	if err != nil {
		return nil, err
	}
	recipeIDs := []string{}
	for _, ls := range links {
		for _, l := range ls {
			recipeIDs = append(recipeIDs, l.RecipeMasterID)
		}
	}
	definitions, err := loadRecipeCostDefinitions(database.DB, recipeIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range products {
		r := p.row
		r.CostStatus = "no_recipe"
		if !p.allowed {
			r.CostStatus = "restricted"
			result = append(result, r)
			continue
		}
		referenceTotal, referenceReady := 0.0, len(links[r.ProductID]) > 0
		for _, l := range links[r.ProductID] {
			d := definitions[l.RecipeMasterID]
			cost := catalog.recipeCost(d, r.OutletID)
			r.Recipes = append(r.Recipes, models.FnbRecipeCost{ProductRecipeLink: l, RecipeCost: cost})
			if cost.InitialCost == nil || cost.InitialCost.Total == nil {
				referenceReady = false
			} else {
				referenceTotal += *cost.InitialCost.Total * l.Multiplier
			}
		}
		if referenceReady {
			v := round4(referenceTotal)
			r.ReferenceMaterialCost = &v
		}
		ms := mats[r.ProductID]
		if p.stockType == "single" && p.linked != "" {
			ms = []material{{p.linked, 1}}
		}
		if len(ms) > 0 {
			r.CostStatus = "ready"
			r.CostBasis = "outlet_stock"
			projectedTotal, projectedComplete := 0.0, true
			for _, m := range ms {
				v, ok := unitCost(m.id, r.OutletID, map[string]bool{})
				inf := infos[m.id]
				if catalog.quantities[m.id+"|"+r.OutletID] <= 0 || catalog.costs[m.id+"|"+r.OutletID] <= 0 {
					r.CostBasis = "estimate"
				}
				projected, projectionOK, source, asOf := catalog.projectedUnitCost(m.id, map[string]bool{})
				comparison := models.FnbPriceComparison{ItemID: m.id, ItemName: inf.name, Unit: inf.unit, QtyBase: m.qty, CurrentUnitCost: round4(v), Source: source, AsOf: asOf}
				if projectionOK && m.qty > 0 && fnbFinite(m.qty) {
					value := round4(projected)
					comparison.ProjectedUnitCost = &value
					projectedTotal += round4(projected * m.qty)
				} else {
					projectedComplete = false
				}
				r.PriceComparisons = append(r.PriceComparisons, comparison)
				if !ok || m.qty <= 0 || !fnbFinite(m.qty) {
					r.CostStatus = "missing_cost"
				}
				line := models.PpicHppIngredient{ItemID: m.id, ItemName: inf.name, Unit: inf.unit, QtyBase: m.qty, CostPerBase: round4(v), Cost: round4(v * m.qty), IsWip: len(wip[m.id]) > 0}
				r.Ingredients = append(r.Ingredients, line)
				r.MaterialCost += line.Cost
			}
			for _, recipe := range r.Recipes {
				if recipe.CurrentCost == nil {
					r.CostStatus = "missing_cost"
				}
			}
			r.MaterialCost = round4(r.MaterialCost)
			r.TotalCost = round4(r.MaterialCost + r.PackagingCost + r.OverheadCost)
			r.MaxMaterialCost = round4(math.Max(0, r.Price*r.TargetCostPct/100-r.PackagingCost-r.OverheadCost))
			if projectedComplete {
				material := round4(projectedTotal)
				total := round4(material + r.PackagingCost + r.OverheadCost)
				r.ProjectedMaterialCost = &material
				r.ProjectedTotalCost = &total
				suggested := math.Ceil(total / (r.TargetCostPct / 100))
				r.ProjectedSuggestedPrice = &suggested
				if r.Price > 0 {
					pct := round4(total / r.Price * 100)
					r.ProjectedCostPct = &pct
					r.ProjectedIsOver = pct >= r.TargetCostPct
					reduction := round4(math.Max(0, material-r.MaxMaterialCost))
					r.ProjectedMaterialReduction = &reduction
				}
			}
			if r.CostStatus == "ready" {
				r.Margin = round4(r.Price - r.TotalCost)
				r.SuggestedPrice = math.Ceil(r.TotalCost / (r.TargetCostPct / 100))
				if r.Price > 0 {
					r.CostPct = round4(r.TotalCost / r.Price * 100)
					r.IsOver = r.CostPct >= r.TargetCostPct
					r.MaterialReduction = round4(math.Max(0, r.MaterialCost-r.MaxMaterialCost))
				}
			}
		}
		result = append(result, r)
	}
	return result, nil
}

func GetFnbOverview(outlet string, scope []string) (*models.FnbOverview, error) {
	costs, err := GetFnbCosting(outlet, scope)
	if err != nil {
		return nil, err
	}
	devs, err := ListFnbDevelopments(outlet, scope)
	if err != nil {
		return nil, err
	}
	recipes, err := ListRecipeMastersScoped(outlet, "", scope)
	if err != nil {
		return nil, err
	}
	r := &models.FnbOverview{ProductCount: len(costs), RecipeCount: len(recipes), DevelopmentStages: map[string]int{}, Attention: []models.FnbCostRow{}}
	for _, d := range devs {
		r.DevelopmentStages[d.Status]++
	}
	for _, c := range costs {
		switch c.MenuKind {
		case "food":
			r.FoodCount++
		case "beverage":
			r.BeverageCount++
		default:
			r.Unclassified++
		}
		switch c.CostStatus {
		case "ready":
			r.CostReady++
		case "no_recipe":
			r.NoRecipe++
		default:
			r.MissingCost++
		}
		if c.IsOver {
			r.OverTarget++
		}
		if c.ProjectedIsOver {
			r.ProjectedOverTarget++
		}
		if len(r.Attention) < 12 && (c.IsOver || c.ProjectedIsOver || c.CostStatus != "ready" || c.MenuKind == "" || c.Price <= 0) {
			c.Ingredients = nil
			c.Recipes = nil
			c.PriceComparisons = nil
			r.Attention = append(r.Attention, c)
		}
	}
	return r, nil
}

func FilterFnbCosts(rows []models.FnbCostRow, kind, search string) []models.FnbCostRow {
	result := []models.FnbCostRow{}
	search = strings.ToLower(strings.TrimSpace(search))
	for _, r := range rows {
		if kind != "" && kind != r.MenuKind && !(kind == "unclassified" && r.MenuKind == "") {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(r.ProductName+" "+r.Category), search) {
			continue
		}
		result = append(result, r)
	}
	return result
}
