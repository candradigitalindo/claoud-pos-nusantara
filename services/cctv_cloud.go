package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Akun cloud tidak pernah disimpan/dikembalikan sebagai URL plaintext ke UI.
// Tiket pemilihan kamera terenkripsi, terikat admin/outlet, kedaluwarsa 10 menit.
type cctvCloudTicket struct {
	Kind    string `json:"kind,omitempty"`
	Source  string `json:"source"`
	Admin   string `json:"admin"`
	Outlet  string `json:"outlet"`
	Expires int64  `json:"expires"`
}

type CCTVCloudDevice struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Ticket string `json:"ticket"`
}

var TuyaRegions = []struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}{
	{"protect-eu.ismartlife.me", "Europe"},
	{"protect-we.ismartlife.me", "East Europe"},
	{"protect-us.ismartlife.me", "America"},
	{"protect-ue.ismartlife.me", "East America"},
	{"protect.ismartlife.me", "China"},
	{"protect-in.ismartlife.me", "India"},
}

func validTuyaHost(host string) bool {
	for _, region := range TuyaRegions {
		if host == region.ID {
			return true
		}
	}
	switch host {
	case "openapi.tuyaus.com", "openapi.tuyaeu.com", "openapi.tuyacn.com", "openapi.tuyain.com", "openapi-ueaz.tuyaus.com", "openapi-weaz.tuyaeu.com":
		return true
	}
	return false
}

func DiscoverTuyaCameras(adminID, outletID, region, email, password string) ([]CCTVCloudDevice, error) {
	valid := false
	for _, r := range TuyaRegions {
		valid = valid || r.ID == region
	}
	if !valid || strings.TrimSpace(email) == "" || password == "" {
		return nil, fmt.Errorf("%w: region, akun dan password Tuya Smart wajib diisi", ErrCameraValidation)
	}
	q := url.Values{"region": {region}, "email": {email}, "password": {password}}
	req, err := http.NewRequest(http.MethodGet, go2rtcBaseURL+"/api/tuya?"+q.Encode(), nil)
	if err != nil {
		return nil, ErrCCTVRelayUnavailable
	}
	// Login vendor dapat lebih lama daripada satu segmen video.
	client := &http.Client{Timeout: 35 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrCCTVRelayUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: akun Tuya Smart tidak dapat dihubungkan; periksa region, akun, dan dukungan kamera", ErrCameraValidation)
	}
	var result struct {
		Sources []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"sources"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil {
		return nil, ErrCCTVRelayUnavailable
	}
	devices := []CCTVCloudDevice{}
	for _, source := range result.Sources {
		if err := validateCameraSourceURL(source.URL, "tuya"); err != nil {
			continue
		}
		u, _ := url.Parse(source.URL)
		id := u.Query().Get("device_id")
		if id == "" {
			continue
		}
		ticket, err := issueCCTVSourceTicket(source.URL, "tuya", adminID, outletID)
		if err != nil {
			return nil, err
		}
		devices = append(devices, CCTVCloudDevice{ID: id, Name: source.Name, Ticket: ticket})
	}
	return devices, nil
}

func issueCCTVSourceTicket(source, kind, adminID, outletID string) (string, error) {
	payload, err := json.Marshal(cctvCloudTicket{Kind: kind, Source: source, Admin: adminID, Outlet: outletID, Expires: time.Now().Add(10 * time.Minute).Unix()})
	if err != nil {
		return "", err
	}
	return encryptSecret(string(payload))
}

func ResolveCCTVCloudTicket(ticket, adminID, outletID string) (string, error) {
	if len(ticket) > 32768 {
		return "", ErrCCTVAccessDenied
	}
	plain, err := decryptSecret(ticket)
	if err != nil {
		return "", ErrCCTVAccessDenied
	}
	var data cctvCloudTicket
	if json.Unmarshal([]byte(plain), &data) != nil || data.Admin != adminID || data.Outlet != outletID || data.Expires <= time.Now().Unix() {
		return "", ErrCCTVAccessDenied
	}
	if data.Kind == "" {
		data.Kind = "tuya"
	} // tiket Tuya versi sebelumnya
	if (data.Kind != "tuya" && data.Kind != "onvif") || validateCameraSourceURL(data.Source, data.Kind) != nil {
		return "", ErrCCTVAccessDenied
	}
	return data.Source, nil
}
