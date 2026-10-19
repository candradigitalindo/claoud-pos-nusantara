package models

import "time"

// OutletCamera — KONFIGURASI koneksi kamera CCTV per outlet (BUKAN rekaman video).
// Cloud hanya menyimpan cara menjangkau DVR (host, channel, kredensial RTSP terenkripsi);
// footage/rekaman tetap 100% di DVR outlet. Password RTSP tidak pernah dikirim balik ke
// frontend — hanya HasPassword (bool) yang diekspos.
type OutletCamera struct {
	ID               string    `json:"id"`
	OutletID         string    `json:"outlet_id"`
	OutletName       string    `json:"outlet_name,omitempty"`
	Name             string    `json:"name"`
	Brand            string    `json:"brand"` // merek perangkat; protokol ditentukan ConnectionType
	ConnectionType   string    `json:"connection_type"`
	AccessMode       string    `json:"access_mode"` // direct | gateway
	HasSourceURL     bool      `json:"has_source_url"`
	HasPlaybackURL   bool      `json:"has_playback_url"`
	SupportsPlayback bool      `json:"supports_playback"`
	Timezone         string    `json:"timezone"`
	Host             string    `json:"host"` // IP/hostname lokal outlet untuk mode gateway
	Port             int       `json:"port"`
	Channel          int       `json:"channel"`
	Subtype          int       `json:"subtype"` // 0 = main-stream, 1 = sub-stream (default sub, hemat bandwidth)
	RTSPUsername     string    `json:"rtsp_username"`
	HasPassword      bool      `json:"has_password"`
	IsActive         bool      `json:"is_active"`
	SortOrder        int       `json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateCameraRequest struct {
	Name           string  `json:"name"`
	Brand          string  `json:"brand"`
	ConnectionType string  `json:"connection_type"`
	AccessMode     string  `json:"access_mode"`
	SourceURL      *string `json:"source_url"`
	PlaybackURL    *string `json:"playback_url"`
	SourceTicket   string  `json:"source_ticket"`
	Host           string  `json:"host"`
	Port           int     `json:"port"`
	Channel        int     `json:"channel"`
	Subtype        int     `json:"subtype"`
	RTSPUsername   string  `json:"rtsp_username"`
	RTSPPassword   string  `json:"rtsp_password"`
	IsActive       *bool   `json:"is_active"`
	SortOrder      int     `json:"sort_order"`
	Timezone       string  `json:"timezone"`
}

type UpdateCameraRequest struct {
	Name           string  `json:"name"`
	Brand          string  `json:"brand"`
	ConnectionType string  `json:"connection_type"`
	AccessMode     string  `json:"access_mode"`
	SourceURL      *string `json:"source_url"`
	PlaybackURL    *string `json:"playback_url"`
	SourceTicket   string  `json:"source_ticket"`
	Host           string  `json:"host"`
	Port           int     `json:"port"`
	Channel        int     `json:"channel"`
	Subtype        int     `json:"subtype"`
	RTSPUsername   string  `json:"rtsp_username"`
	RTSPPassword   *string `json:"rtsp_password"` // nil / tidak dikirim = pertahankan password lama
	IsActive       *bool   `json:"is_active"`
	SortOrder      int     `json:"sort_order"`
	Timezone       string  `json:"timezone"`
}

type CameraProbeRequest struct {
	UpdateCameraRequest
	CameraID string `json:"camera_id"`
}

// PlaybackRequest — rentang waktu putar ulang. start/end = wall-clock lokal DVR
// (format "YYYY-MM-DDTHH:MM:SS"), sesuai jam yang dipilih operator di dashboard.
type PlaybackRequest struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// StreamResponse — jawaban saat memulai live/playback. Berisi NAMA stream + URL relay
// go2rtc melalui relay Go /cctv/. Tidak ada kredensial RTSP yang bocor ke sini.
type StreamResponse struct {
	Name      string `json:"name"`
	Mode      string `json:"mode"` // live | playback
	HLSURL    string `json:"hls_url"`
	WebRTCURL string `json:"webrtc_url,omitempty"`
}
