package models

// ProductClassificationInput contains only catalog evidence. It is also the
// input contract for the offline classifier; it never causes a database write.
type ProductClassificationInput struct {
	ProductID      string                        `json:"product_id"`
	OutletID       string                        `json:"outlet_id"`
	OutletName     string                        `json:"outlet_name"`
	Name           string                        `json:"name"`
	CategoryName   string                        `json:"category_name"`
	Description    string                        `json:"description"`
	CurrentKind    string                        `json:"current_kind"`
	MenuKindSource string                        `json:"menu_kind_source"`
	UpdatedAt      string                        `json:"updated_at"`
	RecipeEvidence []ProductClassificationRecipe `json:"recipe_evidence"`
}

type ProductClassificationRecipe struct {
	RecipeID string `json:"recipe_id"`
	Name     string `json:"name"`
	MenuKind string `json:"menu_kind"`
	Position int    `json:"position"`
}

type ProductClassificationRow struct {
	ProductID     string `json:"product_id"`
	OutletID      string `json:"outlet_id"`
	OutletName    string `json:"outlet_name"`
	Name          string `json:"name"`
	CategoryName  string `json:"category_name"`
	CurrentKind   string `json:"current_kind"`
	SuggestedKind string `json:"suggested_kind"`
	Status        string `json:"status"`
	Confidence    string `json:"confidence"`
	Reason        string `json:"reason"`
	Fingerprint   string `json:"fingerprint"`
}

type ProductClassificationTotals struct {
	Total       int `json:"total"`
	Existing    int `json:"existing"`
	Suggested   int `json:"suggested"`
	NeedsReview int `json:"needs_review"`
	Food        int `json:"food"`
	Beverage    int `json:"beverage"`
	Activity    int `json:"activity"`
	Other       int `json:"other"`
}

type ProductClassificationReport struct {
	Totals ProductClassificationTotals `json:"totals"`
	Rows   []ProductClassificationRow  `json:"rows"`
}

type ApplyProductClassificationItem struct {
	ProductID     string `json:"product_id"`
	Fingerprint   string `json:"fingerprint"`
	SuggestedKind string `json:"suggested_kind"`
}

type ApplyProductClassificationRequest struct {
	Items []ApplyProductClassificationItem `json:"items"`
}
