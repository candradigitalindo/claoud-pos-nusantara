package models

// AssetCategory — master kategori aset beserta angka bawaannya.
type AssetCategory struct {
	ID                        string `json:"id"`
	Name                      string `json:"name"`
	UsefulLifeMonths          int    `json:"useful_life_months"`
	MaintenanceIntervalMonths int    `json:"maintenance_interval_months"`
	Notes                     string `json:"notes"`
	IsActive                  bool   `json:"is_active"`
	AssetCount                int    `json:"asset_count"`
}

type AssetCategoryRequest struct {
	Name                      string `json:"name"`
	UsefulLifeMonths          int    `json:"useful_life_months"`
	MaintenanceIntervalMonths int    `json:"maintenance_interval_months"`
	Notes                     string `json:"notes"`
	IsActive                  bool   `json:"is_active"`
}
