package models

type FnbProfile struct {
	MenuKind      string  `json:"menu_kind"`
	TargetCostPct float64 `json:"target_cost_pct"`
	PackagingCost float64 `json:"packaging_cost"`
	OverheadCost  float64 `json:"overhead_cost"`
}

type FnbCostRow struct {
	OutletTargetCostPct        *float64             `json:"outlet_target_cost_pct"`
	TargetSource               string               `json:"target_source"`
	CostBasis                  string               `json:"cost_basis"`
	ProjectedMaterialCost      *float64             `json:"projected_material_cost"`
	ProjectedTotalCost         *float64             `json:"projected_total_cost"`
	ProjectedCostPct           *float64             `json:"projected_cost_pct"`
	ProjectedSuggestedPrice    *float64             `json:"projected_suggested_price"`
	ProjectedIsOver            bool                 `json:"projected_is_over"`
	MaxMaterialCost            float64              `json:"max_material_cost"`
	MaterialReduction          float64              `json:"material_reduction"`
	ProjectedMaterialReduction *float64             `json:"projected_material_reduction"`
	PriceComparisons           []FnbPriceComparison `json:"price_comparisons,omitempty"`
	Recipes                    []FnbRecipeCost      `json:"recipes,omitempty"`
	ReferenceMaterialCost      *float64             `json:"reference_material_cost"`
	FnbProfile
	ProductID      string              `json:"product_id"`
	ProductName    string              `json:"product_name"`
	OutletID       string              `json:"outlet_id"`
	OutletName     string              `json:"outlet_name"`
	Category       string              `json:"category"`
	RecipeID       string              `json:"recipe_id"`
	Price          float64             `json:"price"`
	MaterialCost   float64             `json:"material_cost"`
	TotalCost      float64             `json:"total_cost"`
	CostPct        float64             `json:"cost_pct"`
	Margin         float64             `json:"margin"`
	SuggestedPrice float64             `json:"suggested_price"`
	CostStatus     string              `json:"cost_status"` // ready, no_recipe, missing_cost, restricted
	IsOver         bool                `json:"is_over"`
	Ingredients    []PpicHppIngredient `json:"ingredients,omitempty"`
}

type FnbOutletPolicy struct {
	OutletName    string  `json:"outlet_name"`
	OutletID      string  `json:"outlet_id"`
	TargetCostPct float64 `json:"target_cost_pct"`
	Configured    bool    `json:"configured"`
}

type FnbPriceComparison struct {
	ItemID            string   `json:"item_id"`
	ItemName          string   `json:"item_name"`
	Unit              string   `json:"unit"`
	QtyBase           float64  `json:"qty_base"`
	CurrentUnitCost   float64  `json:"current_unit_cost"`
	ProjectedUnitCost *float64 `json:"projected_unit_cost"`
	Source            string   `json:"source"`
	AsOf              string   `json:"as_of"`
}

type FnbDevelopment struct {
	ID            string  `json:"id"`
	OutletID      string  `json:"outlet_id"`
	OutletName    string  `json:"outlet_name"`
	Name          string  `json:"name"`
	MenuKind      string  `json:"menu_kind"`
	Status        string  `json:"status"`
	Brief         string  `json:"brief"`
	TrialNotes    string  `json:"trial_notes"`
	TargetPrice   float64 `json:"target_price"`
	TargetCostPct float64 `json:"target_cost_pct"`
	RecipeID      string  `json:"recipe_master_id"`
	RecipeName    string  `json:"recipe_name"`
	ProductID     string  `json:"product_id"`
	ProductName   string  `json:"product_name"`
	Version       int     `json:"version"`
	CreatedBy     string  `json:"created_by"`
	UpdatedBy     string  `json:"updated_by"`
	ApprovedBy    string  `json:"approved_by"`
	UpdatedAt     string  `json:"updated_at"`
}

type FnbOverview struct {
	ProjectedOverTarget int            `json:"projected_over_target"`
	ProductCount        int            `json:"product_count"`
	FoodCount           int            `json:"food_count"`
	BeverageCount       int            `json:"beverage_count"`
	Unclassified        int            `json:"unclassified"`
	CostReady           int            `json:"cost_ready"`
	MissingCost         int            `json:"missing_cost"`
	NoRecipe            int            `json:"no_recipe"`
	OverTarget          int            `json:"over_target"`
	RecipeCount         int            `json:"recipe_count"`
	DevelopmentStages   map[string]int `json:"development_stages"`
	Attention           []FnbCostRow   `json:"attention"`
}
