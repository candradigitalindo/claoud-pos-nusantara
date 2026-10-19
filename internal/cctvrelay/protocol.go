// Package cctvrelay menghubungkan relay outlet melalui permintaan HTTPS keluar.
// Tidak menyediakan proxy HTTP umum, tunnel jaringan, atau eksekusi perintah.
package cctvrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const MaxBody = 8 << 20

type Request struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query"`
}

type Response struct {
	ID          string `json:"id"`
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Body        []byte `json:"body"`
}

func ValidRequest(r Request) bool {
	q, err := url.ParseQuery(r.Query)
	if err != nil {
		return false
	}
	for key, values := range q {
		if len(values) != 1 || (key != "src" && key != "name" && key != "id" && key != "n" && key != "mp4" && key != "video") {
			return false
		}
	}
	if r.Path == "/api/streams" {
		if r.Method == http.MethodPatch {
			if !streamName(q.Get("name")) || q.Get("src") == "" {
				return false
			}
			u, err := url.Parse(q.Get("src"))
			if err != nil || u.Fragment != "" {
				return false
			}
			switch u.Scheme {
			case "rtsp", "rtsps", "onvif", "dvrip", "tapo", "xiaomi", "bubble", "tuya":
				return u.Hostname() != ""
			}
			return false
		}
		return (r.Method == http.MethodGet || r.Method == http.MethodDelete) && streamName(q.Get("src"))
	}
	if r.Method != http.MethodGet {
		return false
	}
	if r.Path == "/api/onvif" {
		u, err := url.Parse(q.Get("src"))
		return err == nil && u.Scheme == "onvif" && u.Hostname() != "" && u.Fragment == ""
	}
	switch r.Path {
	case "/api/stream.m3u8":
		return streamName(q.Get("src"))
	case "/api/hls/playlist.m3u8", "/api/hls/segment.ts", "/api/hls/init.mp4", "/api/hls/segment.m4s":
		return q.Get("id") != "" && len(q.Get("id")) <= 128
	}
	return false
}

func streamName(name string) bool {
	if len(name) > 100 || !(strings.HasPrefix(name, "cam_") || strings.HasPrefix(name, "pb_") || strings.HasPrefix(name, "check_")) {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}

func Execute(ctx context.Context, client *http.Client, relayURL string, job Request) Response {
	result := Response{ID: job.ID, Status: http.StatusBadGateway}
	if !ValidRequest(job) {
		result.Status = http.StatusForbidden
		return result
	}
	req, err := http.NewRequestWithContext(ctx, job.Method, strings.TrimRight(relayURL, "/")+job.Path+"?"+job.Query, nil)
	if err != nil {
		return result
	}
	resp, err := client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	result.Status = resp.StatusCode
	if resp.StatusCode >= 300 {
		return result
	} // jangan kirim galat yang memuat URL/password
	result.ContentType = resp.Header.Get("Content-Type")
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil || len(data) > MaxBody {
		result.Status = http.StatusBadGateway
		return result
	}
	result.Body = data
	return result
}

type Client struct {
	CloudURL string
	Token    string
	RelayURL string
	HTTP     *http.Client
}

func (c *Client) Validate() error {
	u, err := url.Parse(c.CloudURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("alamat cloud tidak valid")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return errors.New("koneksi cloud harus HTTPS; HTTP hanya untuk uji localhost")
	}
	if len(c.Token) < 32 {
		return errors.New("token penghubung belum diisi")
	}
	r, err := url.Parse(c.RelayURL)
	if err != nil || r.Scheme != "http" || (r.Hostname() != "127.0.0.1" && r.Hostname() != "localhost" && r.Hostname() != "::1") || r.RawQuery != "" || r.User != nil {
		return errors.New("relay outlet harus berada di localhost")
	}
	return nil
}

func (c *Client) exchange(ctx context.Context, path string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	base := strings.TrimSuffix(strings.TrimRight(c.CloudURL, "/"), "/api/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/cctv/gateway/"+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errors.New("cloud tidak dapat dihubungi")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("cloud menolak koneksi penghubung; periksa token dan alamat cloud")
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, MaxBody*2)).Decode(result)
	}
	return nil
}

func (c *Client) Run(ctx context.Context, onError func(error)) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	relayClient := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	workers := make(chan struct{}, 4)
	for ctx.Err() == nil {
		var reply struct {
			Jobs []Request `json:"jobs"`
		}
		if err := c.exchange(ctx, "poll", struct{}{}, &reply); err != nil {
			if ctx.Err() != nil {
				break
			}
			if onError != nil {
				onError(err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}
		for _, job := range reply.Jobs {
			select {
			case workers <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			go func(job Request) {
				defer func() { <-workers }()
				result := Execute(ctx, relayClient, c.RelayURL, job)
				if err := c.exchange(ctx, "result", result, nil); err != nil && ctx.Err() == nil && onError != nil {
					onError(err)
				}
			}(job)
		}
	}
	return ctx.Err()
}
