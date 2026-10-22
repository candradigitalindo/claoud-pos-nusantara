package models

import "time"

type ProjectUsageItem struct {
	MaterialID    string  `json:"material_id"`
	Qty           float64 `json:"qty"`
	Name          string  `json:"name"`
	Unit          string  `json:"unit"`
	RequestNumber string  `json:"request_number"`
}

type ProjectUsageInput struct {
	ProjectID       string             `json:"project_id"`
	Kind            string             `json:"kind"`
	UsageDate       string             `json:"usage_date"`
	Location        string             `json:"location"`
	WorkDescription string             `json:"work_description"`
	Performer       string             `json:"performer"`
	PICName         string             `json:"pic_name"`
	Items           []ProjectUsageItem `json:"items"`
	Evidence        []string           `json:"evidence"`
	Version         int64              `json:"version"`
}

type ProjectUsageReport struct {
	ProjectUsageInput
	ID            string              `json:"id"`
	ReportNumber  string              `json:"report_number"`
	ProjectName   string              `json:"project_name"`
	ProjectNumber string              `json:"project_number"`
	Status        string              `json:"status"`
	CreatedBy     string              `json:"created_by"`
	CreatedByID   string              `json:"created_by_id"`
	SubmittedByID string              `json:"submitted_by_id"`
	ReviewedBy    string              `json:"reviewed_by"`
	ReviewNote    string              `json:"review_note"`
	SubmittedAt   *time.Time          `json:"submitted_at"`
	ReviewedAt    *time.Time          `json:"reviewed_at"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
	PostedItems   []ProjectUsageItem  `json:"posted_items"`
	Events        []ProjectUsageEvent `json:"events,omitempty"`
}

type ProjectUsageEvent struct {
	Action    string            `json:"action"`
	Actor     string            `json:"actor"`
	Note      string            `json:"note"`
	CreatedAt time.Time         `json:"created_at"`
	Snapshot  ProjectUsageInput `json:"snapshot"`
}

type ProjectUsageAction struct {
	Action  string `json:"action"`
	Version int64  `json:"version"`
	Note    string `json:"note"`
}
