package models

type FnbProfile struct {
	MenuKind      string  `json:"menu_kind"`
	TargetCostPct float64 `json:"target_cost_pct"`
	PackagingCost float64 `json:"packaging_cost"`
	OverheadCost  float64 `json:"overhead_cost"`
}

type FnbCostRow struct {
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
	ProductCount      int            `json:"product_count"`
	FoodCount         int            `json:"food_count"`
	BeverageCount     int            `json:"beverage_count"`
	Unclassified      int            `json:"unclassified"`
	CostReady         int            `json:"cost_ready"`
	MissingCost       int            `json:"missing_cost"`
	NoRecipe          int            `json:"no_recipe"`
	OverTarget        int            `json:"over_target"`
	RecipeCount       int            `json:"recipe_count"`
	DevelopmentStages map[string]int `json:"development_stages"`
	Attention         []FnbCostRow   `json:"attention"`
}
