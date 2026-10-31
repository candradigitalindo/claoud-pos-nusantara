package models

type BatchSyncItem struct {
	EntityType string      `json:"entity_type"`
	Operation  string      `json:"operation"`
	Data       interface{} `json:"data"`
}

type BatchSyncRequest struct {
	OutletID      string          `json:"outlet_id"`
	OutletCode    string          `json:"outlet_code"`
	SyncTimestamp string          `json:"sync_timestamp"`
	Items         []BatchSyncItem `json:"items"`
}

type BatchSyncResult struct {
	EventID    string `json:"event_id,omitempty"`
	EntityType string `json:"entity_type"`
	LocalID    string `json:"local_id"`
	CloudID    string `json:"cloud_id,omitempty"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type BatchSyncResponse struct {
	Processed int               `json:"processed"`
	Success   int               `json:"success"`
	Failed    int               `json:"failed"`
	Results   []BatchSyncResult `json:"results"`
	SyncedAt  string            `json:"synced_at"`
}

type UpdateEntity struct {
	CloudID      string   `json:"cloud_id"`
	LocalID      string   `json:"local_id"`
	Name         string   `json:"name,omitempty"`
	Code         string   `json:"code,omitempty"`
	Description  string   `json:"description,omitempty"`
	CategoryID   string   `json:"category_id,omitempty"`
	CategoryName string   `json:"category_name,omitempty"`
	Price        *float64 `json:"price,omitempty"`
	PhotoURL     string   `json:"photo_url"` // empty is an explicit photo deletion
	MenuKind     string   `json:"menu_kind"` // empty is explicitly unclassified
	Version      int      `json:"version"`
	UpdatedAt    string   `json:"updated_at"`
	Action       string   `json:"action"`
}

type DeletedEntity struct {
	EntityType string `json:"entity_type"`
	LocalID    string `json:"local_id"`
	CloudID    string `json:"cloud_id"`
	DeletedAt  string `json:"deleted_at"`
}

type UpdatesResponse struct {
	Products       []UpdateEntity  `json:"products"`
	Categories     []UpdateEntity  `json:"categories"`
	Deleted        []DeletedEntity `json:"deleted"`
	SyncCheckpoint string          `json:"sync_checkpoint"`
}

type SyncConflict struct {
	ID            string  `json:"id"`
	OutletID      string  `json:"outlet_id"`
	EntityType    string  `json:"entity_type"`
	EntityLocalID string  `json:"entity_local_id"`
	EntityCloudID string  `json:"entity_cloud_id"`
	ConflictField string  `json:"conflict_field"`
	CloudValue    string  `json:"cloud_value"`
	LocalValue    string  `json:"local_value"`
	CloudVersion  int     `json:"cloud_version"`
	LocalVersion  int     `json:"local_version"`
	Resolution    string  `json:"resolution"`
	ResolvedBy    string  `json:"resolved_by"`
	ResolvedAt    *string `json:"resolved_at"`
	Notes         string  `json:"notes"`
	CreatedAt     string  `json:"created_at"`
}

type ResolveConflictRequest struct {
	Strategy   string `json:"strategy"`
	ResolvedBy string `json:"resolved_by"`
	Notes      string `json:"notes"`
}

type RestoreResponse struct {
	OpenShifts        []CloudCashierShift       `json:"open_shifts"`
	UnpaidOrders      []CloudOrder              `json:"unpaid_orders"`
	CashMovements     []CloudCashMovement       `json:"cash_movements"`
	AdditionalCharges []RestoreAdditionalCharge `json:"additional_charges"`
	Summary           RestoreSummary            `json:"summary"`
	RestoredAt        string                    `json:"restored_at"`
}

// Portable master IDs let a fresh POS reconnect restored order charges to its
// own integer primary keys without matching names or guessing an old device ID.
type RestoreAdditionalCharge struct {
	CloudAdditionalCharge
	IsActive bool `json:"is_active"`
}

type RestoreSummary struct {
	OpenShiftCount    int `json:"open_shift_count"`
	UnpaidOrderCount  int `json:"unpaid_order_count"`
	CashMovementCount int `json:"cash_movement_count"`
}

type SyncLog struct {
	ID           string `json:"id"`
	OutletID     string `json:"outlet_id"`
	Action       string `json:"action"`
	EntityType   string `json:"entity_type"`
	EntityCount  int    `json:"entity_count"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    string `json:"created_at"`
}
