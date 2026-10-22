package models

import "time"

type Document struct {
	ID             string    `json:"id"`
	DocumentNumber string    `json:"document_number"`
	About          string    `json:"about"`
	FileName       string    `json:"file_name"`
	ContentType    string    `json:"content_type"`
	FileSize       int64     `json:"file_size"`
	UploadedBy     string    `json:"uploaded_by"`
	UploadedAt     time.Time `json:"uploaded_at"`
	DownloadCount  int64     `json:"download_count"`
}
