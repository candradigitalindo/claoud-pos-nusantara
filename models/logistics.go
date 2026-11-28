package models

// LogisticsDelivery adalah dokumen Pengantaran Logistik / Surat Jalan (SJ-...)
// yang mengelompokkan barang-barang Purchasing per item (bisa lintas beberapa PR
// dalam satu pengantaran) hingga diterima final oleh Gudang Induk / Bagian Aset.
type LogisticsDelivery struct {
	ID                     string                  `json:"id"`
	DeliveryNumber         string                  `json:"delivery_number"`
	DeliveryDate           string                  `json:"delivery_date"`
	PurchasingGroup        string                  `json:"purchasing_group"` // bahan_baku | barang_jasa
	DispatchMode           string                  `json:"dispatch_mode"`    // logistics | vendor_direct
	Status                 string                  `json:"status"`           // draft | in_transit | partial_received | received | cancelled
	CourierName            string                  `json:"courier_name"`
	CourierID              string                  `json:"courier_id,omitempty"`
	VehicleInfo            string                  `json:"vehicle_info"`
	OriginLabel            string                  `json:"origin_label"`
	DestinationType        string                  `json:"destination_type"` // warehouse | outlet | asset_desk
	DestinationWarehouseID string                  `json:"destination_warehouse_id,omitempty"`
	DestinationOutletID    string                  `json:"destination_outlet_id,omitempty"`
	DestinationLabel       string                  `json:"destination_label"`
	PickupPhotoURLs        []string                `json:"pickup_photo_urls"`
	ReceivePhotoURLs       []string                `json:"receive_photo_urls"`
	Notes                  string                  `json:"notes"`
	CreatedBy              string                  `json:"created_by"`
	DispatchedBy           string                  `json:"dispatched_by,omitempty"`
	DispatchedAt           string                  `json:"dispatched_at,omitempty"`
	ReceivedBy             string                  `json:"received_by,omitempty"`
	ReceivedAt             string                  `json:"received_at,omitempty"`
	CancelledBy            string                  `json:"cancelled_by,omitempty"`
	CancelledAt            string                  `json:"cancelled_at,omitempty"`
	CancelReason           string                  `json:"cancel_reason,omitempty"`
	CreatedAt              string                  `json:"created_at"`
	UpdatedAt              string                  `json:"updated_at"`
	ItemCount              int                     `json:"item_count,omitempty"`
	TotalPlanned           int                     `json:"total_planned,omitempty"`
	TotalReceived          int                     `json:"total_received,omitempty"`
	PRNumbers              []string                `json:"pr_numbers,omitempty"`
	ReceivingWarehouseID   string                  `json:"receiving_warehouse_id,omitempty"`
	CapitalizationMin      float64                 `json:"capitalization_min,omitempty"`
	Items                  []LogisticsDeliveryItem `json:"items,omitempty"`
}

type LogisticsDeliveryItem struct {
	ID                     string  `json:"id"`
	DeliveryID             string  `json:"delivery_id"`
	PurchaseRequestID      string  `json:"purchase_request_id"`
	PRNumber               string  `json:"pr_number"`
	RequestType            string  `json:"request_type"`
	VendorName             string  `json:"vendor_name"`
	OutletID               string  `json:"outlet_id,omitempty"`
	OutletName             string  `json:"outlet_name,omitempty"`
	ProjectID              string  `json:"project_id,omitempty"`
	PRItemKey              string  `json:"pr_item_key"`
	EntryName              string  `json:"entry_name"`
	ItemName               string  `json:"item_name"`
	Unit                   string  `json:"unit"`
	LineKind               string  `json:"line_kind"` // dapur | perlengkapan | projek
	UnitPrice              float64 `json:"unit_price,omitempty"`
	QtyPlanned             int     `json:"qty_planned"`
	QtyReceived            int     `json:"qty_received"`
	Remaining              int     `json:"remaining"`
	Destination            string  `json:"destination,omitempty"` // default suggested destination: stok | aset | material | habis
	SuggestedStockItemID   string  `json:"suggested_stock_item_id,omitempty"`
	SuggestedStockItemName string  `json:"suggested_stock_item_name,omitempty"`
	Notes                  string  `json:"notes"`
}

// DispatchablePRItem adalah baris item pengadaan yang masih punya kuota bebas
// (belum diterima & belum masuk Surat Jalan aktif) untuk dijadwalkan pengantarannya.
type DispatchablePRItem struct {
	PurchaseRequestID   string  `json:"purchase_request_id"`
	PRNumber            string  `json:"pr_number"`
	RequestType         string  `json:"request_type"`
	PurchasingGroup     string  `json:"purchasing_group"`
	Status              string  `json:"status"`
	OutletID            string  `json:"outlet_id,omitempty"`
	OutletName          string  `json:"outlet_name,omitempty"`
	ProjectID           string  `json:"project_id,omitempty"`
	VendorName          string  `json:"vendor_name"`
	TargetWarehouseName string  `json:"target_warehouse_name,omitempty"`
	PRItemKey           string  `json:"pr_item_key"`
	EntryName           string  `json:"entry_name"`
	ItemName            string  `json:"item_name"`
	Unit                string  `json:"unit"`
	LineKind            string  `json:"line_kind"`
	UnitPrice           float64 `json:"unit_price"`
	QtyPurchased        int     `json:"qty_purchased"`
	QtyReceived         int     `json:"qty_received"`
	QtyInDelivery       int     `json:"qty_in_delivery"`
	QtyAvailable        int     `json:"qty_available"`
}

// LineDeliveryRef menandai pengantaran logistik aktif yang membawa baris item PR tertentu.
type LineDeliveryRef struct {
	DeliveryID     string `json:"delivery_id"`
	DeliveryNumber string `json:"delivery_number"`
	DispatchMode   string `json:"dispatch_mode"`
	Status         string `json:"status"`
	CourierName    string `json:"courier_name"`
	QtyPending     int    `json:"qty_pending"`
}

// PRDeliverySummary ringkasan pengantaran logistik yang terkait dengan satu PR (tampil di detail PR & dialog terima).
type PRDeliverySummary struct {
	ID               string   `json:"id"`
	DeliveryNumber   string   `json:"delivery_number"`
	DeliveryDate     string   `json:"delivery_date"`
	PurchasingGroup  string   `json:"purchasing_group"`
	DispatchMode     string   `json:"dispatch_mode"`
	Status           string   `json:"status"`
	CourierName      string   `json:"courier_name"`
	VehicleInfo      string   `json:"vehicle_info,omitempty"`
	DestinationLabel string   `json:"destination_label"`
	ItemCount        int      `json:"item_count"`
	TotalPlanned     int      `json:"total_planned"`
	TotalReceived    int      `json:"total_received"`
	PickupPhotoURLs  []string `json:"pickup_photo_urls"`
	ReceivePhotoURLs []string `json:"receive_photo_urls"`
}

type LogisticsDeliveryItemInput struct {
	PurchaseRequestID string `json:"purchase_request_id"`
	PRItemKey         string `json:"pr_item_key"`
	QtyPlanned        int    `json:"qty_planned"`
	Notes             string `json:"notes"`
}

type CreateLogisticsDeliveryInput struct {
	DeliveryDate           string                       `json:"delivery_date"`
	DispatchMode           string                       `json:"dispatch_mode"` // logistics | vendor_direct
	DispatchNow            bool                         `json:"dispatch_now"`  // true → langsung status in_transit
	CourierName            string                       `json:"courier_name"`
	VehicleInfo            string                       `json:"vehicle_info"`
	OriginLabel            string                       `json:"origin_label"`
	DestinationType        string                       `json:"destination_type"` // warehouse | outlet | asset_desk
	DestinationWarehouseID string                       `json:"destination_warehouse_id"`
	DestinationOutletID    string                       `json:"destination_outlet_id"`
	DestinationLabel       string                       `json:"destination_label"`
	PickupPhotoURLs        []string                     `json:"pickup_photo_urls"`
	ReceivePhotoURLs       []string                     `json:"receive_photo_urls"`
	Notes                  string                       `json:"notes"`
	Items                  []LogisticsDeliveryItemInput `json:"items"`
}

type UpdateLogisticsDeliveryInput struct {
	DeliveryDate           string                       `json:"delivery_date"`
	DispatchMode           string                       `json:"dispatch_mode"`
	CourierName            string                       `json:"courier_name"`
	VehicleInfo            string                       `json:"vehicle_info"`
	OriginLabel            string                       `json:"origin_label"`
	DestinationType        string                       `json:"destination_type"`
	DestinationWarehouseID string                       `json:"destination_warehouse_id"`
	DestinationOutletID    string                       `json:"destination_outlet_id"`
	DestinationLabel       string                       `json:"destination_label"`
	PickupPhotoURLs        *[]string                    `json:"pickup_photo_urls,omitempty"`
	ReceivePhotoURLs       *[]string                    `json:"receive_photo_urls,omitempty"`
	Notes                  string                       `json:"notes"`
	Items                  []LogisticsDeliveryItemInput `json:"items,omitempty"`
}

type UpdateLogisticsPhotosInput struct {
	PickupPhotoURLs  *[]string `json:"pickup_photo_urls,omitempty"`
	ReceivePhotoURLs *[]string `json:"receive_photo_urls,omitempty"`
}

type UpdateLogisticsStatusInput struct {
	Status          string    `json:"status"` // in_transit | cancelled
	CancelReason    string    `json:"cancel_reason"`
	PickupPhotoURLs *[]string `json:"pickup_photo_urls,omitempty"`
}

type ReceiveLogisticsLineInput struct {
	DeliveryItemID   string `json:"delivery_item_id"`
	Qty              int    `json:"qty"`
	Destination      string `json:"destination"` // aset | stok | material | habis
	OutletID         string `json:"outlet_id,omitempty"`
	Category         string `json:"category,omitempty"`
	TrackingMode     string `json:"tracking_mode,omitempty"`
	AssetName        string `json:"asset_name,omitempty"`
	SerialNumber     string `json:"serial_number,omitempty"`
	Brand            string `json:"brand,omitempty"`
	Model            string `json:"model,omitempty"`
	Location         string `json:"location,omitempty"`
	UsefulLifeMonths int    `json:"useful_life_months,omitempty"`
	ResidualValue    float64 `json:"residual_value,omitempty"`
	StockItemID      string `json:"stock_item_id,omitempty"`
	WarehouseID      string `json:"warehouse_id,omitempty"`
	UnitUsed         string `json:"unit_used,omitempty"`
	ExpiryDate       string `json:"expiry_date,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

type ReceiveLogisticsDeliveryInput struct {
	OutletID  string                      `json:"outlet_id,omitempty"`
	Desk      string                      `json:"desk,omitempty"`
	PhotoURL  string                      `json:"photo_url,omitempty"`
	PhotoURLs []string                    `json:"photo_urls,omitempty"`
	Notes     string                      `json:"notes,omitempty"`
	Lines     []ReceiveLogisticsLineInput `json:"lines"`
}

type ReceiveLogisticsDeliveryResult struct {
	DeliveryID       string   `json:"delivery_id"`
	DeliveryNumber   string   `json:"delivery_number"`
	DeliveryStatus   string   `json:"delivery_status"`
	AssetsCreated    int      `json:"assets_created"`
	GRNNumbers       []string `json:"grn_numbers,omitempty"`
	GRNNumber        string   `json:"grn_number,omitempty"`
	MaterialLines    int      `json:"material_lines"`
	ExpenseLines     int      `json:"expense_lines"`
	StockLinesQueued int      `json:"stock_lines_queued"`
	RemainingItems   int      `json:"remaining_items"`
	Message          string   `json:"message"`
}
