package services

import (
	"cloud-pos/models"
	"sort"
	"strings"
)

// GetPpicHppReport uses the same outlet valuation, recipe quantities, additional
// costs and outlet thresholds as F&B. The legacy idealPct argument is retained
// for API compatibility; saved outlet/menu policies are the source of truth.
func GetPpicHppReport(outletID string, _ float64, outletScope []string) (*models.PpicHppReport, error) {
	costs, err := GetFnbCosting(outletID, outletScope)
	if err != nil {
		return nil, err
	}
	rep := &models.PpicHppReport{Rows: []models.PpicHppRow{}, ProductCount: len(costs)}
	noRecipe := []string{}
	sumPct := 0.0
	count := 0
	for i, c := range costs {
		r := models.PpicHppRow{ProductID: c.ProductID, OutletID: c.OutletID, OutletName: c.OutletName, ProductName: c.ProductName, Category: c.Category,
			Price: c.Price, Hpp: c.TotalCost, HppPct: c.CostPct, Margin: c.Margin, TargetCostPct: c.TargetCostPct, CostStatus: c.CostStatus, CostBasis: c.CostBasis,
			PackagingCost: c.PackagingCost, OverheadCost: c.OverheadCost, ProjectedHpp: c.ProjectedTotalCost, ProjectedCostPct: c.ProjectedCostPct, ProjectedIsOver: c.ProjectedIsOver,
			IdealCost: round4(c.Price * c.TargetCostPct / 100), IsOver: c.IsOver, HasRecipe: c.CostStatus != "no_recipe" && c.CostStatus != "restricted", Ingredients: c.Ingredients}
		if i == 0 {
			rep.IdealPct = c.TargetCostPct
		} else if rep.IdealPct != c.TargetCostPct {
			rep.IdealPct = 0
		}
		if r.HasRecipe {
			rep.WithRecipe++
		} else {
			noRecipe = append(noRecipe, c.ProductName)
		}
		if c.CostStatus == "ready" && c.Price > 0 {
			r.MarginPct = round4(100 - c.CostPct)
			sumPct += c.CostPct
			count++
		}
		if c.IsOver {
			rep.OverCount++
		}
		rep.Rows = append(rep.Rows, r)
	}
	if count > 0 {
		rep.AvgHppPct = round4(sumPct / float64(count))
	}
	rep.NoRecipeSample = strings.Join(uniqueHead(noRecipe, 10), ", ")
	sort.SliceStable(rep.Rows, func(i, j int) bool {
		a, b := rep.Rows[i], rep.Rows[j]
		if a.HasRecipe != b.HasRecipe {
			return a.HasRecipe
		}
		return a.HppPct > b.HppPct
	})
	return rep, nil
}
