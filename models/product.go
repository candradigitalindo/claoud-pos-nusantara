package models

import "time"

type PushProductRequest struct {
	LocalID      string  `json:"local_id"`
	OutletID     string  `json:"outlet_id"`
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Description  string  `json:"description"`
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Price        float64 `json:"price"`
	Destination  string  `json:"destination"`
	Version      int     `json:"version"`
	UpdatedAt    string  `json:"updated_at"`
}

type CloudProduct struct {
	Recipes          []ProductRecipeLink `json:"recipes"`
	ID               string              `json:"id"`
	LocalID          string              `json:"local_id"`
	OutletID         string              `json:"outlet_id"`
	OutletName       string              `json:"outlet_name,omitempty"`
	Name             string              `json:"name"`
	Code             string              `json:"code"`
	Description      string              `json:"description"`
	CategoryID       string              `json:"category_id"`
	CategoryName     string              `json:"category_name"`
	Price            float64             `json:"price"`
	Destination      string              `json:"destination"`
	PhotoURL         string              `json:"photo_url"`
	StockType        string              `json:"stock_type"`
	RecipeMasterID   string              `json:"recipe_master_id"`
	RecipeMasterName string              `json:"recipe_master_name"`
	IsDeleted        bool                `json:"is_deleted"`
	Version          int                 `json:"version"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	SyncedAt         time.Time           `json:"synced_at"`
}

type PushCategoryRequest struct {
	LocalID     string `json:"local_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CodePrefix  string `json:"code_prefix"`
	Version     int    `json:"version"`
}

type CloudCategory struct {
	ID         string    `json:"id"`
	LocalID    string    `json:"local_id"`
	OutletID   string    `json:"outlet_id"`
	OutletName string    `json:"outlet_name,omitempty"`
	Name       string    `json:"name"`
	CodePrefix string    `json:"code_prefix"`
	PrinterID  string    `json:"printer_id"`
	IsDeleted  bool      `json:"is_deleted"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	SyncedAt   time.Time `json:"synced_at"`
}

type AdminCreateProductRequest struct {
	Recipes        *[]ProductRecipeLink `json:"recipes"`
	OutletID       string               `json:"outlet_id"`
	Name           string               `json:"name"`
	Code           string               `json:"code"`
	Description    string               `json:"description"`
	CategoryID     string               `json:"category_id"`
	CategoryName   string               `json:"category_name"`
	Price          float64              `json:"price"`
	Destination    string               `json:"destination"`
	RecipeMasterID string               `json:"recipe_master_id"`
}

type AdminUpdateProductRequest struct {
	Recipes      *[]ProductRecipeLink `json:"recipes"`
	Name         string               `json:"name"`
	Code         string               `json:"code"`
	Description  string               `json:"description"`
	CategoryID   string               `json:"category_id"`
	CategoryName string               `json:"category_name"`
	Price        float64              `json:"price"`
	Destination  string               `json:"destination"`
	// Omitted/null preserves the current stock setup; an empty string unlinks it.
	RecipeMasterID *string `json:"recipe_master_id"`
}

// ProductRecipeOption is the compact catalog for recipes available to a product's outlet.
type ProductRecipeOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MenuKind    string `json:"menu_kind"`
	TotalTime   int    `json:"total_time"`
	ItemCount   int    `json:"item_count"`
}

// ProductRecipeDetail omits the access list of other outlets.
type ProductRecipeDetail struct {
	RecipeCost
	ProductRecipeOption
	Instructions string       `json:"instructions"`
	Items        []RecipeItem `json:"items"`
}

// Multiplier is the number of full recipe compositions used by one sold menu.
type ProductRecipeLink struct {
	RecipeMasterID string  `json:"recipe_master_id"`
	Name           string  `json:"name,omitempty"`
	Multiplier     float64 `json:"multiplier"`
}

type RecipeCostSnapshot struct {
	CapturedAt  string                 `json:"captured_at"`
	Source      string                 `json:"source"`
	Total       *float64               `json:"total"`
	Ingredients []RecipeCostIngredient `json:"ingredients"`
}

type RecipeCost struct {
	CurrentCost     *float64               `json:"current_cost"`
	CostIngredients []RecipeCostIngredient `json:"cost_ingredients"`
	InitialCost     *RecipeCostSnapshot    `json:"initial_cost"`
}

type FnbRecipeCost struct {
	ProductRecipeLink
	RecipeCost
}

type AdminCreateCategoryRequest struct {
	OutletID   string `json:"outlet_id"`
	Name       string `json:"name"`
	CodePrefix string `json:"code_prefix"`
}

type AdminUpdateCategoryRequest struct {
	Name       string `json:"name"`
	CodePrefix string `json:"code_prefix"`
}
